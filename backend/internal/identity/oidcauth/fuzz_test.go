package oidcauth

import (
	"encoding/json"
	"strings"
	"testing"

	"kubebench/internal/config"
)

// FuzzIdentityFromClaims checks how a provider's claims become an identity:
// it never panics on claims of any shape, an accepted identity has a
// trimmed, non-empty username taken from the configured (possibly dotted)
// claim, and with allowed domains only a verified address of one of them
// signs in.
func FuzzIdentityFromClaims(f *testing.F) {
	for _, seed := range [][2]string{
		{`{"preferred_username":"alice","email":"alice@example.com","email_verified":true,"groups":["a","b"]}`, "groups"},
		{`{"preferred_username":" bob ","email":"bob@EXAMPLE.com","email_verified":"TRUE","realm_access":{"roles":["admin",1,""]}}`, "realm_access.roles"},
		{`{"preferred_username":"eve","email":"eve@evil.com@example.com","email_verified":true}`, "groups"},
		{`{"preferred_username":"eve","email":"eve@example.com","email_verified":"yes"}`, "groups"},
		{`{"preferred_username":"eve","email":"eve@example.com.evil.com","email_verified":true}`, "groups"},
		{`{"preferred_username":["x"],"realm_access.roles":"direct","realm_access":"s"}`, "realm_access.roles"},
		{`{"a":{"b":{"c":"deep"}}}`, "a.b.c"},
		{`{}`, ""}, {`{"":{"":"x"}}`, "."},
	} {
		f.Add([]byte(seed[0]), seed[1])
	}
	f.Fuzz(func(t *testing.T, raw []byte, groupsClaim string) {
		var claims map[string]any
		if json.Unmarshal(raw, &claims) != nil || claims == nil {
			return
		}
		for _, cfg := range []config.OIDCConfig{
			{UsernameClaim: "preferred_username", GroupsClaim: groupsClaim},
			{UsernameClaim: groupsClaim, GroupsClaim: groupsClaim, AllowedDomains: []string{"example.com"}},
		} {
			id, err := identityFromClaims(cfg, "sub-1", claims)
			if err != nil {
				continue
			}
			if id.Username == "" || id.Username != strings.TrimSpace(id.Username) {
				t.Fatalf("claims %s gave the username %q", raw, id.Username)
			}
			if v, _ := lookupClaim(claims, cfg.UsernameClaim); v == nil || strings.TrimSpace(v.(string)) != id.Username {
				t.Fatalf("claims %s: username %q is not the %q claim", raw, id.Username, cfg.UsernameClaim)
			}
			if id.Groups == nil {
				t.Fatalf("claims %s gave nil groups", raw)
			}
			if len(cfg.AllowedDomains) > 0 {
				_, domain, _ := strings.Cut(id.Email, "@")
				if !emailVerified(claims) || !strings.EqualFold(domain, "example.com") ||
					!strings.HasSuffix(strings.ToLower(id.Email), "@example.com") {
					t.Fatalf("claims %s signed in with the address %q", raw, id.Email)
				}
			}
		}
	})
}
