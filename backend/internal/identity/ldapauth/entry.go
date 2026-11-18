package ldapauth

import (
	"encoding/binary"
	"fmt"
	"slices"
	"strconv"
	"strings"

	"github.com/go-ldap/ldap/v3"

	"kubebench/internal/config"
	"kubebench/internal/identity"
	"kubebench/internal/store"
)

// Active Directory specifics.
const (
	attrUserAccountControl = "userAccountControl"
	attrMemberOf           = "memberOf"
	attrObjectGUID         = "objectGUID"
	// uacAccountDisable is the ACCOUNTDISABLE bit of userAccountControl.
	uacAccountDisable = 0x2
	// groupPageSize pages the group search (Active Directory answers at
	// most 1000 entries per page).
	groupPageSize = 500
)

// user is a directory entry found for a login.
type user struct {
	dn       string
	id       *identity.Identity
	username string // the raw username attribute, for the group filter
	memberOf []string
	disabled bool
}

// findUser searches the entry of a login name as the service account. No
// entry, or more than one, is invalid_credentials: the sign-in screen must
// not tell which accounts exist.
func (p *Provider) findUser(name string) (*user, error) {
	filter := userFilter(p.cfg.UserFilter, name)
	attrs := []string{p.cfg.UsernameAttribute, p.cfg.IDAttribute, p.cfg.DisplayNameAttribute, p.cfg.EmailAttribute,
		attrUserAccountControl}
	if p.cfg.GroupsFrom == config.LDAPGroupsFromMemberOf {
		attrs = append(attrs, attrMemberOf)
	}
	req := ldap.NewSearchRequest(p.cfg.UserBaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		2, p.timeLimit(), false, filter, attrs, nil)
	var res *ldap.SearchResult
	err := p.pool.with(func(c conn) error {
		var err error
		res, err = c.Search(req)
		return err
	})
	switch {
	case ldap.IsErrorWithCode(err, ldap.LDAPResultSizeLimitExceeded) || (err == nil && len(res.Entries) > 1):
		return nil, identity.Fail(identity.ClassInvalidCredentials, "more than one directory entry matches %q", name)
	case err != nil:
		return nil, identity.Fail(identity.ClassProviderError, "search %q: %v", name, err)
	case len(res.Entries) == 0:
		return nil, identity.Fail(identity.ClassInvalidCredentials, "no directory entry matches %q", name)
	}
	return p.userOf(res.Entries[0])
}

// userOf reads the identity of a user entry.
func (p *Provider) userOf(e *ldap.Entry) (*user, error) {
	subject := e.GetAttributeValue(p.cfg.IDAttribute)
	if strings.EqualFold(p.cfg.IDAttribute, attrObjectGUID) {
		subject = formatGUID(e.GetRawAttributeValue(p.cfg.IDAttribute))
	}
	if subject == "" {
		return nil, identity.Fail(identity.ClassProviderError, "directory entry %s has no %s (KUBEBENCH_LDAP_ID_ATTRIBUTE)",
			e.DN, p.cfg.IDAttribute)
	}
	raw := e.GetAttributeValue(p.cfg.UsernameAttribute)
	u := &user{
		dn:       e.DN,
		username: raw,
		memberOf: e.GetAttributeValues(attrMemberOf),
		id: &identity.Identity{
			Source:      store.UserSourceLDAP,
			Subject:     subject,
			Username:    strings.ToLower(raw),
			DisplayName: e.GetAttributeValue(p.cfg.DisplayNameAttribute),
			Email:       e.GetAttributeValue(p.cfg.EmailAttribute),
		},
	}
	if uac := e.GetAttributeValue(attrUserAccountControl); uac != "" {
		flags, err := strconv.ParseInt(uac, 10, 64)
		u.disabled = err != nil || flags&uacAccountDisable != 0
	}
	return u, nil
}

// groupsOf lists the names of the user's groups: from memberOf, or from a
// group search as the service account (nested in Active Directory with
// the in-chain matching rule of the default filter).
func (p *Provider) groupsOf(u *user) ([]string, error) {
	if p.cfg.GroupsFrom == config.LDAPGroupsFromMemberOf {
		names := make([]string, 0, len(u.memberOf))
		for _, dn := range u.memberOf {
			names = append(names, p.groupNameOfDN(dn))
		}
		return dedupe(names), nil
	}
	filter := groupFilter(p.cfg.GroupFilter, u.dn, u.username)
	attrs := []string{"1.1"} // no attributes: the DN is the name
	byDN := strings.EqualFold(p.cfg.GroupNameAttribute, config.LDAPGroupNameDN)
	if !byDN {
		attrs = []string{p.cfg.GroupNameAttribute}
	}
	req := ldap.NewSearchRequest(p.cfg.GroupBaseDN, ldap.ScopeWholeSubtree, ldap.NeverDerefAliases,
		0, p.timeLimit(), false, filter, attrs, nil)
	var res *ldap.SearchResult
	err := p.pool.with(func(c conn) error {
		var err error
		res, err = c.SearchWithPaging(req, groupPageSize)
		return err
	})
	if err != nil {
		return nil, identity.Fail(identity.ClassProviderError, "search the groups of %s: %v", u.dn, err)
	}
	names := make([]string, 0, len(res.Entries))
	for _, g := range res.Entries {
		name := g.DN
		if !byDN {
			name = g.GetAttributeValue(p.cfg.GroupNameAttribute)
		}
		if name != "" {
			names = append(names, name)
		}
	}
	return dedupe(names), nil
}

// userFilter is the user search filter for a login name: template with
// the name, escaped, in place of each placeholder.
func userFilter(template, name string) string {
	return strings.ReplaceAll(template, config.LDAPUsernamePlaceholder, ldap.EscapeFilter(name))
}

// groupFilter is the group search filter of a user: template with the
// user's DN and username, escaped, in place of their placeholders.
func groupFilter(template, dn, username string) string {
	return strings.NewReplacer(
		config.LDAPUserDNPlaceholder, ldap.EscapeFilter(dn),
		config.LDAPUserPlaceholder, ldap.EscapeFilter(username),
	).Replace(template)
}

// groupNameOfDN names a memberOf value: the DN itself, or the value of its
// first RDN (cn=kb-admins,ou=groups,… is kb-admins).
func (p *Provider) groupNameOfDN(dn string) string {
	if strings.EqualFold(p.cfg.GroupNameAttribute, config.LDAPGroupNameDN) {
		return dn
	}
	parsed, err := ldap.ParseDN(dn)
	if err != nil || len(parsed.RDNs) == 0 || len(parsed.RDNs[0].Attributes) == 0 {
		return dn
	}
	return parsed.RDNs[0].Attributes[0].Value
}

// dedupe sorts names and drops duplicates that differ only in case
// (mappings match groups ignoring case).
func dedupe(names []string) []string {
	slices.SortFunc(names, func(a, b string) int { return strings.Compare(strings.ToLower(a), strings.ToLower(b)) })
	return slices.CompactFunc(names, strings.EqualFold)
}

// formatGUID renders an Active Directory objectGUID (16 bytes, the first
// three fields little-endian) in its usual string form; "" when malformed.
func formatGUID(b []byte) string {
	if len(b) != 16 {
		return ""
	}
	return fmt.Sprintf("%08x-%04x-%04x-%x-%x",
		binary.LittleEndian.Uint32(b[0:4]), binary.LittleEndian.Uint16(b[4:6]), binary.LittleEndian.Uint16(b[6:8]),
		b[8:10], b[10:16])
}

// adAccountDisabled reports whether a failed bind is Active Directory's
// "account disabled" (data 533) or "account expired" (data 701). Active
// Directory reports these only for the right password.
func adAccountDisabled(err error) bool {
	msg := err.Error()
	return strings.Contains(msg, "data 533,") || strings.Contains(msg, "data 701,")
}
