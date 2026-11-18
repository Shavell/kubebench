package config

import (
	"bytes"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"strings"
)

// AuthConfig configures external identities (docs/design-v0.5.md §3.6):
// the providers users may sign in with besides local accounts, and the
// group mappings that grant them roles.
type AuthConfig struct {
	// RequireGroup refuses external users whose groups grant nothing
	// (KUBEBENCH_AUTH_REQUIRE_GROUP); otherwise they sign in and see no
	// cluster.
	RequireGroup bool
	// Mappings are seeded into the database on the first start that has
	// them (KUBEBENCH_AUTH_GROUP_MAPPINGS); afterwards admins manage them.
	Mappings []MappingSeed
	// OIDC is the OpenID Connect provider; nil when not configured.
	OIDC *OIDCConfig
}

// External reports whether any external identity provider is configured.
func (a AuthConfig) External() bool { return a.OIDC != nil }

// MappingSeed is one group mapping of KUBEBENCH_AUTH_GROUP_MAPPINGS:
// {"group":"kb-admins","role":"admin"} or
// {"group":"kb-ops","role":"operator","cluster":"prod","source":"oidc"}.
type MappingSeed struct {
	Group   string `json:"group"`
	Source  string `json:"source,omitempty"`
	Role    string `json:"role"`
	Cluster string `json:"cluster,omitempty"`
}

// OIDCConfig is the OpenID Connect provider (authorization code flow with
// PKCE). ClientSecret is a secret: never log or return it.
type OIDCConfig struct {
	Issuer       string
	ClientID     string
	ClientSecret string // empty for a public client
	Scopes       []string
	// UsernameClaim and GroupsClaim name ID token (or userinfo) claims;
	// GroupsClaim may be a dotted path such as realm_access.roles.
	UsernameClaim string
	GroupsClaim   string
	// Name is shown on the sign-in button ("Sign in with <Name>").
	Name string
	// AllowedDomains, when set, admit only verified email addresses of
	// these domains.
	AllowedDomains []string
	// CAFile is a PEM bundle trusted for the provider's TLS certificate,
	// in addition to the system roots.
	CAFile string
}

// DefaultOIDCScopes are requested when KUBEBENCH_OIDC_SCOPES is unset.
var DefaultOIDCScopes = []string{"openid", "profile", "email"}

func (p *parser) auth() AuthConfig {
	a := AuthConfig{RequireGroup: p.boolean("KUBEBENCH_AUTH_REQUIRE_GROUP", false)}
	if raw := p.str("KUBEBENCH_AUTH_GROUP_MAPPINGS", ""); raw != "" {
		dec := json.NewDecoder(strings.NewReader(raw))
		dec.DisallowUnknownFields()
		if err := dec.Decode(&a.Mappings); err != nil {
			p.fail("KUBEBENCH_AUTH_GROUP_MAPPINGS", fmt.Sprintf(
				`must be a JSON list such as [{"group":"kb-admins","role":"admin"},{"group":"kb-ops","role":"operator","cluster":"prod"}]: %v`, err))
		}
	}
	a.OIDC = p.oidc()
	return a
}

func (p *parser) oidc() *OIDCConfig {
	issuer := p.str("KUBEBENCH_OIDC_ISSUER", "")
	if issuer == "" {
		return nil
	}
	o := &OIDCConfig{
		Issuer:        strings.TrimSuffix(issuer, "/"),
		ClientID:      p.str("KUBEBENCH_OIDC_CLIENT_ID", ""),
		ClientSecret:  p.secret("KUBEBENCH_OIDC_CLIENT_SECRET"),
		Scopes:        p.list("KUBEBENCH_OIDC_SCOPES", DefaultOIDCScopes),
		UsernameClaim: p.str("KUBEBENCH_OIDC_USERNAME_CLAIM", "preferred_username"),
		GroupsClaim:   p.str("KUBEBENCH_OIDC_GROUPS_CLAIM", "groups"),
		Name:          p.str("KUBEBENCH_OIDC_NAME", "SSO"),
		CAFile:        p.str("KUBEBENCH_OIDC_CA_FILE", ""),
	}
	if u, err := url.Parse(issuer); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		p.fail("KUBEBENCH_OIDC_ISSUER", "must be the provider's http or https issuer URL")
	}
	if o.ClientID == "" {
		p.fail("KUBEBENCH_OIDC_CLIENT_ID", "is required with KUBEBENCH_OIDC_ISSUER")
	}
	if !containsFold(o.Scopes, "openid") {
		o.Scopes = append([]string{"openid"}, o.Scopes...)
	}
	for _, d := range p.list("KUBEBENCH_OIDC_ALLOWED_DOMAINS", nil) {
		o.AllowedDomains = append(o.AllowedDomains, strings.ToLower(strings.TrimPrefix(d, "@")))
	}
	if o.CAFile != "" {
		if err := checkCABundle(o.CAFile); err != nil {
			p.fail("KUBEBENCH_OIDC_CA_FILE", err.Error())
		}
	}
	return o
}

// secret reads a secret from key or, when that is unset, from the file
// key_FILE names (a mounted Kubernetes Secret, a systemd credential).
// Setting both is an error: which one wins would be a guess.
func (p *parser) secret(key string) string {
	value := p.str(key, "")
	file := p.str(key+"_FILE", "")
	if file == "" {
		return value
	}
	if value != "" {
		p.fail(key, "set either "+key+" or "+key+"_FILE, not both")
		return value
	}
	b, err := os.ReadFile(file)
	if err != nil {
		p.fail(key+"_FILE", fmt.Sprintf("cannot be read: %v", err))
		return ""
	}
	return strings.TrimSpace(string(b))
}

// list splits a comma or space separated value; fallback when unset.
func (p *parser) list(key string, fallback []string) []string {
	raw := p.str(key, "")
	if raw == "" {
		return fallback
	}
	return strings.FieldsFunc(raw, func(r rune) bool { return r == ',' || r == ' ' })
}

// origin parses an http(s) origin without a path; example illustrates the
// expected form in the error.
func (p *parser) origin(key, example string) string {
	raw := p.str(key, "")
	if raw == "" {
		return ""
	}
	u, err := url.Parse(raw)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
		strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
		p.fail(key, "must be an http or https URL without a path, e.g. "+example)
		return ""
	}
	return strings.ToLower(u.Scheme) + "://" + u.Host
}

// checkCABundle reports whether file holds at least one PEM certificate.
func checkCABundle(file string) error {
	b, err := os.ReadFile(file)
	if err != nil {
		return fmt.Errorf("cannot be read: %v", err)
	}
	if !x509.NewCertPool().AppendCertsFromPEM(bytes.TrimSpace(b)) {
		return fmt.Errorf("%s holds no PEM certificate", file)
	}
	return nil
}

func containsFold(list []string, s string) bool {
	for _, v := range list {
		if strings.EqualFold(v, s) {
			return true
		}
	}
	return false
}
