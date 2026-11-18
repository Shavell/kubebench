package api

import (
	"context"
	"net"
	"net/http"
	"strings"
	"testing"

	"kubebench/internal/identity"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// closedPort is a local address nothing listens on.
func closedPort(t *testing.T) string {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	addr := l.Addr().String()
	_ = l.Close()
	return addr
}

func TestLDAPProviderSettings(t *testing.T) {
	const bindPassword = "bind-pw-never-shown"
	e := newTestEnv(t, map[string]string{
		"KUBEBENCH_LDAP_URL":           "ldap://" + closedPort(t),
		"KUBEBENCH_LDAP_TYPE":          "ad",
		"KUBEBENCH_LDAP_NAME":          "Corp AD",
		"KUBEBENCH_LDAP_BIND_DN":       "CN=svc,DC=corp,DC=example",
		"KUBEBENCH_LDAP_BIND_PASSWORD": bindPassword,
		"KUBEBENCH_LDAP_USER_BASE_DN":  "DC=corp,DC=example",
		"KUBEBENCH_LDAP_TIMEOUT":       "1s",
	})

	methods := decode[struct{ Providers []signInMethod }](t, e.do(http.MethodGet, "/api/auth/methods", "", nil))
	if len(methods.Providers) != 1 || methods.Providers[0] != (signInMethod{Source: "ldap", Name: "Corp AD", Type: "password"}) {
		t.Fatalf("methods = %+v", methods)
	}

	rec := e.do(http.MethodGet, "/api/settings/auth", "admin", nil)
	expect(t, rec, http.StatusOK, "")
	if strings.Contains(rec.Body.String(), bindPassword) {
		t.Fatal("the bind password was returned")
	}
	view := decode[authSettingsView](t, rec)
	if len(view.Providers) != 1 {
		t.Fatalf("providers = %+v", view.Providers)
	}
	p := view.Providers[0]
	settings := map[string]providerSetting{}
	for _, s := range p.Settings {
		settings[s.Key] = s
	}
	if p.Status.OK || p.Status.Error == "" || len(p.Warnings) != 1 || p.Warnings[0] != "plaintext" ||
		settings["bindPassword"].SecretSet == nil || !*settings["bindPassword"].SecretSet ||
		settings["userFilter"].Value != "(|(sAMAccountName=%s)(userPrincipalName=%s))" {
		t.Fatalf("ldap view = %+v", p)
	}

	// An unreachable directory is a provider error, not a wrong password.
	expect(t, e.serve(directoryLogin("jdoe", "pw")), http.StatusBadGateway, plugin.CodeInternal)
	rec = e.serve(directoryLogin("jdoe", ""))
	expect(t, rec, http.StatusUnauthorized, plugin.CodeUnauthenticated)
	// The sign-in screen translates the failure class (auth.signin.<class>).
	if refusal := decode[plugin.ErrorBody](t, rec); refusal.Reason != "auth.signin."+identity.ClassInvalidCredentials {
		t.Fatalf("refusal = %+v", refusal)
	}
	for _, r := range e.audit() {
		if strings.Contains(r.Detail, bindPassword) {
			t.Fatalf("audit row holds the bind password: %+v", r)
		}
	}
}

// normalizingDirectory is fakeDirectory with a directory's login
// normalization (DOMAIN\ prefix, case).
type normalizingDirectory struct{ fakeDirectory }

func (normalizingDirectory) NormalizeUsername(login string) string {
	login = strings.ToLower(strings.TrimSpace(login))
	if i := strings.LastIndexByte(login, '\\'); i >= 0 {
		login = login[i+1:]
	}
	return login
}

func (d normalizingDirectory) Authenticate(ctx context.Context, username, password string) (*identity.Identity, error) {
	return d.fakeDirectory.Authenticate(ctx, d.NormalizeUsername(username), password)
}

// The forms of one account share their failed attempts, and the audit log
// names the account once.
func TestDirectoryThrottleUsesNormalizedName(t *testing.T) {
	e, _ := oidcEnv(t, nil)
	e.srv.ident.passwords[store.UserSourceLDAP] = normalizingDirectory{}
	logins := []string{"Eve", `CORP\eve`, " EVE ", `other\Eve`, "eve"}
	for i := range loginMaxFailures {
		e.serve(directoryLogin(logins[i%len(logins)], "x"))
	}
	expect(t, e.serve(directoryLogin(`CORP\EVE`, "x")), http.StatusTooManyRequests, plugin.CodeRateLimited)
	for _, r := range e.audit() {
		if r.Action == auditSignInAction && r.Username != "eve" {
			t.Fatalf("audit username %q, want eve", r.Username)
		}
	}
	// The preview resolves the login the same way.
	p := decode[previewView](t, e.do(http.MethodPost, "/api/settings/auth/preview", "admin",
		map[string]any{"source": "ldap", "username": `CORP\Dora`, "groups": []string{"kb-ops"}}))
	if p.Username != "dora" || !p.Allowed {
		t.Fatalf("preview = %+v", p)
	}
}
