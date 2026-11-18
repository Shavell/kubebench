package config

import (
	"net/url"
	"strings"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Directory flavours of KUBEBENCH_LDAP_TYPE. They choose the defaults of
// the search filters and attributes; every default can be overridden.
const (
	LDAPTypeOpenLDAP        = "openldap"
	LDAPTypeActiveDirectory = "ad"
)

// Where the groups of a directory user come from (KUBEBENCH_LDAP_GROUPS_FROM).
const (
	LDAPGroupsFromSearch   = "search"   // a group search with GroupFilter
	LDAPGroupsFromMemberOf = "memberOf" // the user entry's memberOf values
)

// LDAPGroupNameDN as the group name attribute reports groups by their full
// DN instead of an attribute value.
const LDAPGroupNameDN = "dn"

// Placeholders of the search filters, replaced by escaped values.
const (
	LDAPUsernamePlaceholder = "%s" // UserFilter: the login name
	LDAPUserDNPlaceholder   = "%d" // GroupFilter: the user's DN
	LDAPUserPlaceholder     = "%u" // GroupFilter: the user's username attribute
)

// LDAPConfig is the LDAP or Active Directory provider: a service account
// searches the user, a bind as the user checks the password. BindPassword
// is a secret: never log or return it.
type LDAPConfig struct {
	// URL is ldap://host[:port] or ldaps://host[:port].
	URL  string
	Type string
	// Name is shown on the sign-in screen.
	Name string
	// StartTLS upgrades an ldap:// connection before anything is sent.
	StartTLS bool
	// CAFile is a PEM bundle trusted for the server's certificate, in
	// addition to the system roots.
	CAFile string
	// InsecureSkipVerify turns off TLS certificate verification. Only for
	// tests: anyone on the path can read the passwords.
	InsecureSkipVerify bool
	// BindDN and BindPassword are the service account that searches users
	// and groups; empty BindDN searches anonymously.
	BindDN       string
	BindPassword string

	UserBaseDN string
	// UserFilter finds the user by the login name (%s).
	UserFilter           string
	UsernameAttribute    string
	IDAttribute          string
	DisplayNameAttribute string
	EmailAttribute       string

	GroupsFrom  string
	GroupBaseDN string
	// GroupFilter finds the user's groups by DN (%d) or username (%u).
	GroupFilter        string
	GroupNameAttribute string

	// Timeout bounds connecting and each request.
	Timeout time.Duration
}

// TLS reports whether connections are encrypted (LDAPS or StartTLS).
func (c LDAPConfig) TLS() bool {
	return c.StartTLS || strings.HasPrefix(strings.ToLower(c.URL), "ldaps://")
}

// ldapDefaults are the per-flavour defaults.
type ldapDefaults struct {
	name, userFilter, username, id, displayName, groupsFrom, groupFilter string
}

var ldapFlavours = map[string]ldapDefaults{
	LDAPTypeOpenLDAP: {
		name:        "Directory",
		userFilter:  "(uid=%s)",
		username:    "uid",
		id:          "entryUUID",
		displayName: "cn",
		groupsFrom:  LDAPGroupsFromSearch,
		groupFilter: "(|(member=%d)(uniqueMember=%d)(memberUid=%u))",
	},
	LDAPTypeActiveDirectory: {
		name:        "Active Directory",
		userFilter:  "(|(sAMAccountName=%s)(userPrincipalName=%s))",
		username:    "sAMAccountName",
		id:          "objectGUID",
		displayName: "displayName",
		groupsFrom:  LDAPGroupsFromSearch,
		// LDAP_MATCHING_RULE_IN_CHAIN: the groups the user is a member of,
		// directly or through nested groups.
		groupFilter: "(member:1.2.840.113556.1.4.1941:=%d)",
	},
}

func (p *parser) ldap() *LDAPConfig {
	raw := p.str("KUBEBENCH_LDAP_URL", "")
	if raw == "" {
		return nil
	}
	flavour := strings.ToLower(p.str("KUBEBENCH_LDAP_TYPE", LDAPTypeOpenLDAP))
	d, ok := ldapFlavours[flavour]
	if !ok {
		p.fail("KUBEBENCH_LDAP_TYPE", "must be openldap or ad")
		d = ldapFlavours[LDAPTypeOpenLDAP]
	}
	c := &LDAPConfig{
		URL:                  raw,
		Type:                 flavour,
		Name:                 p.str("KUBEBENCH_LDAP_NAME", d.name),
		StartTLS:             p.boolean("KUBEBENCH_LDAP_START_TLS", false),
		CAFile:               p.str("KUBEBENCH_LDAP_CA_FILE", ""),
		InsecureSkipVerify:   p.boolean("KUBEBENCH_LDAP_INSECURE_SKIP_VERIFY", false),
		BindDN:               p.str("KUBEBENCH_LDAP_BIND_DN", ""),
		BindPassword:         p.secret("KUBEBENCH_LDAP_BIND_PASSWORD"),
		UserBaseDN:           p.str("KUBEBENCH_LDAP_USER_BASE_DN", ""),
		UserFilter:           p.str("KUBEBENCH_LDAP_USER_FILTER", d.userFilter),
		UsernameAttribute:    p.str("KUBEBENCH_LDAP_USERNAME_ATTRIBUTE", d.username),
		IDAttribute:          p.str("KUBEBENCH_LDAP_ID_ATTRIBUTE", d.id),
		DisplayNameAttribute: p.str("KUBEBENCH_LDAP_DISPLAY_NAME_ATTRIBUTE", d.displayName),
		EmailAttribute:       p.str("KUBEBENCH_LDAP_EMAIL_ATTRIBUTE", "mail"),
		GroupsFrom:           p.str("KUBEBENCH_LDAP_GROUPS_FROM", d.groupsFrom),
		GroupFilter:          p.str("KUBEBENCH_LDAP_GROUP_FILTER", d.groupFilter),
		GroupNameAttribute:   p.str("KUBEBENCH_LDAP_GROUP_NAME_ATTRIBUTE", "cn"),
		Timeout:              p.duration("KUBEBENCH_LDAP_TIMEOUT", 10*time.Second),
	}
	c.GroupBaseDN = p.str("KUBEBENCH_LDAP_GROUP_BASE_DN", c.UserBaseDN)

	if u, err := url.Parse(raw); err != nil || (u.Scheme != "ldap" && u.Scheme != "ldaps") || u.Host == "" ||
		strings.Trim(u.Path, "/") != "" || u.User != nil {
		p.fail("KUBEBENCH_LDAP_URL", "must be ldap://host[:port] or ldaps://host[:port]")
	} else if u.Scheme == "ldaps" && c.StartTLS {
		p.fail("KUBEBENCH_LDAP_START_TLS", "applies to ldap:// URLs; ldaps:// is already encrypted")
	}
	if c.CAFile != "" {
		if err := checkCABundle(c.CAFile); err != nil {
			p.fail("KUBEBENCH_LDAP_CA_FILE", err.Error())
		}
	}
	if c.BindDN != "" && c.BindPassword == "" {
		p.fail("KUBEBENCH_LDAP_BIND_PASSWORD", "is required with KUBEBENCH_LDAP_BIND_DN")
	}
	if c.UserBaseDN == "" {
		p.fail("KUBEBENCH_LDAP_USER_BASE_DN", "is required with KUBEBENCH_LDAP_URL")
	}
	p.dn("KUBEBENCH_LDAP_BIND_DN", c.BindDN)
	p.dn("KUBEBENCH_LDAP_USER_BASE_DN", c.UserBaseDN)
	p.dn("KUBEBENCH_LDAP_GROUP_BASE_DN", c.GroupBaseDN)
	if !strings.Contains(c.UserFilter, LDAPUsernamePlaceholder) {
		p.fail("KUBEBENCH_LDAP_USER_FILTER", "must contain %s, the login name")
	}
	p.filter("KUBEBENCH_LDAP_USER_FILTER", c.UserFilter)
	switch c.GroupsFrom {
	case LDAPGroupsFromSearch:
		if !strings.Contains(c.GroupFilter, LDAPUserDNPlaceholder) && !strings.Contains(c.GroupFilter, LDAPUserPlaceholder) {
			p.fail("KUBEBENCH_LDAP_GROUP_FILTER", "must contain %d (the user's DN) or %u (the username)")
		}
		p.filter("KUBEBENCH_LDAP_GROUP_FILTER", c.GroupFilter)
	case LDAPGroupsFromMemberOf:
	default:
		p.fail("KUBEBENCH_LDAP_GROUPS_FROM", "must be search or memberOf")
	}
	if c.Timeout < time.Second {
		p.fail("KUBEBENCH_LDAP_TIMEOUT", "must be at least 1s")
	}
	return c
}

// dn checks a distinguished name; empty is left to the caller.
func (p *parser) dn(key, value string) {
	if value == "" {
		return
	}
	if _, err := ldap.ParseDN(value); err != nil {
		p.fail(key, "must be a distinguished name such as dc=example,dc=org: "+err.Error())
	}
}

// filter checks that a search filter compiles with its placeholders filled.
func (p *parser) filter(key, value string) {
	sample := strings.NewReplacer(LDAPUsernamePlaceholder, "x", LDAPUserDNPlaceholder, "cn=x", LDAPUserPlaceholder, "x").Replace(value)
	if _, err := ldap.CompileFilter(sample); err != nil {
		p.fail(key, "must be an LDAP search filter such as (uid=%s): "+err.Error())
	}
}
