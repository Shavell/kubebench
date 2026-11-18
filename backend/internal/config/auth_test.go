package config

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestAuthDefaults(t *testing.T) {
	c, err := FromEnv(env(nil), false)
	if err != nil {
		t.Fatal(err)
	}
	if c.Auth.OIDC != nil || c.Auth.External() || c.Auth.RequireGroup || c.Auth.Mappings != nil || c.PublicURL != "" {
		t.Fatalf("auth defaults: %+v, public URL %q", c.Auth, c.PublicURL)
	}
}

func TestOIDCConfig(t *testing.T) {
	c, err := FromEnv(env(map[string]string{
		"KUBEBENCH_PUBLIC_URL":            "https://KubeBench.example.com/",
		"KUBEBENCH_OIDC_ISSUER":           "https://idp.example.com/realms/main/",
		"KUBEBENCH_OIDC_CLIENT_ID":        "kubebench",
		"KUBEBENCH_OIDC_CLIENT_SECRET":    "s3cret",
		"KUBEBENCH_OIDC_SCOPES":           "profile, email groups",
		"KUBEBENCH_OIDC_ALLOWED_DOMAINS":  "@Example.com,corp.example",
		"KUBEBENCH_OIDC_GROUPS_CLAIM":     "realm_access.roles",
		"KUBEBENCH_AUTH_REQUIRE_GROUP":    "true",
		"KUBEBENCH_AUTH_GROUP_MAPPINGS":   `[{"group":"kb-admins","role":"admin"},{"group":"ops","role":"operator","cluster":"prod","source":"oidc"}]`,
		"KUBEBENCH_OIDC_USERNAME_CLAIM":   "email",
		"KUBEBENCH_OIDC_NAME":             "Keycloak",
		"KUBEBENCH_SESSION_TTL":           "8h",
		"KUBEBENCH_TRUST_PROXY":           "true",
		"KUBEBENCH_SECURE_COOKIES":        "auto",
		"KUBEBENCH_AUDIT_RETENTION_DAYS":  "30",
		"KUBEBENCH_ALLOW_PATH_KUBECONFIG": "false",
	}), false)
	if err != nil {
		t.Fatal(err)
	}
	o := c.Auth.OIDC
	if c.PublicURL != "https://KubeBench.example.com" || o == nil || o.Issuer != "https://idp.example.com/realms/main" ||
		o.ClientSecret != "s3cret" || o.UsernameClaim != "email" || o.GroupsClaim != "realm_access.roles" || o.Name != "Keycloak" {
		t.Fatalf("config: %q %+v", c.PublicURL, o)
	}
	if !slices.Equal(o.Scopes, []string{"openid", "profile", "email", "groups"}) {
		t.Fatalf("scopes = %v", o.Scopes)
	}
	if !slices.Equal(o.AllowedDomains, []string{"example.com", "corp.example"}) {
		t.Fatalf("domains = %v", o.AllowedDomains)
	}
	if !c.Auth.RequireGroup || len(c.Auth.Mappings) != 2 ||
		c.Auth.Mappings[1] != (MappingSeed{Group: "ops", Role: "operator", Cluster: "prod", Source: "oidc"}) {
		t.Fatalf("auth = %+v", c.Auth)
	}
}

func TestOIDCClientSecretFile(t *testing.T) {
	file := filepath.Join(t.TempDir(), "secret")
	if err := os.WriteFile(file, []byte("from-file\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	base := map[string]string{"KUBEBENCH_OIDC_ISSUER": "https://idp.example.com", "KUBEBENCH_OIDC_CLIENT_ID": "kb",
		"KUBEBENCH_OIDC_CLIENT_SECRET_FILE": file}
	c, err := FromEnv(env(base), false)
	if err != nil || c.Auth.OIDC.ClientSecret != "from-file" {
		t.Fatalf("secret file: %v %+v", err, c)
	}
	base["KUBEBENCH_OIDC_CLIENT_SECRET"] = "inline"
	if _, err := FromEnv(env(base), false); err == nil || !strings.Contains(err.Error(), "not both") {
		t.Fatalf("both set: %v", err)
	}
}

func TestAuthValidation(t *testing.T) {
	ca := filepath.Join(t.TempDir(), "ca.pem")
	if err := os.WriteFile(ca, []byte("not a certificate"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := FromEnv(env(map[string]string{
		"KUBEBENCH_OIDC_ISSUER":             "idp.example.com",
		"KUBEBENCH_OIDC_CA_FILE":            ca,
		"KUBEBENCH_OIDC_CLIENT_SECRET_FILE": filepath.Join(t.TempDir(), "missing"),
		"KUBEBENCH_PUBLIC_URL":              "https://kubebench.example.com/app",
		"KUBEBENCH_AUTH_GROUP_MAPPINGS":     `{"group":"x"}`,
		"KUBEBENCH_AUTH_REQUIRE_GROUP":      "maybe",
	}), false)
	if err == nil {
		t.Fatal("invalid auth settings accepted")
	}
	for _, key := range []string{"KUBEBENCH_OIDC_ISSUER", "KUBEBENCH_OIDC_CLIENT_ID", "KUBEBENCH_OIDC_CA_FILE",
		"KUBEBENCH_OIDC_CLIENT_SECRET_FILE", "KUBEBENCH_PUBLIC_URL", "KUBEBENCH_AUTH_GROUP_MAPPINGS", "KUBEBENCH_AUTH_REQUIRE_GROUP"} {
		if !strings.Contains(err.Error(), key+":") {
			t.Errorf("no error for %s in %v", key, err)
		}
	}
	// The secret itself never appears in an error.
	_, err = FromEnv(env(map[string]string{"KUBEBENCH_OIDC_ISSUER": "https://idp", "KUBEBENCH_OIDC_CLIENT_SECRET": "s3cret"}), false)
	if err == nil || strings.Contains(err.Error(), "s3cret") {
		t.Fatalf("err = %v", err)
	}
}
