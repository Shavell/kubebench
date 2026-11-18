package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"kubebench/internal/identity"
	"kubebench/internal/identity/oidcauth/oidctest"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

const testPublicURL = "https://kubebench.test"

// oidcEnv is a test server with OIDC sign-in through a fake provider and
// the mappings kb-admins → admin, kb-ops → operator and kb-dev → viewer on
// the test cluster.
func oidcEnv(t *testing.T, extra map[string]string) (*testEnv, *oidctest.Provider) {
	t.Helper()
	idp := oidctest.New(t)
	vars := map[string]string{
		"KUBEBENCH_PUBLIC_URL":         testPublicURL,
		"KUBEBENCH_OIDC_ISSUER":        idp.URL,
		"KUBEBENCH_OIDC_CLIENT_ID":     oidctest.ClientID,
		"KUBEBENCH_OIDC_CLIENT_SECRET": oidctest.ClientSecret,
		"KUBEBENCH_OIDC_NAME":          "Fake IdP",
	}
	for k, v := range extra {
		vars[k] = v
	}
	e := newTestEnv(t, vars)
	for _, m := range []store.GroupMapping{
		{Group: "kb-admins", Role: store.RoleAdmin},
		{Group: "kb-ops", Role: store.ClusterRoleOperator, Cluster: "test"},
		{Group: "kb-dev", Role: store.ClusterRoleViewer, Cluster: "test"},
	} {
		if _, err := e.store.CreateGroupMapping(m); err != nil {
			t.Fatal(err)
		}
	}
	return e, idp
}

func claimsOf(sub, username string, groups ...string) map[string]any {
	return map[string]any{"sub": sub, "preferred_username": username, "email": username + "@example.com", "groups": groups}
}

// startOIDC starts a sign-in and returns the provider URL and the state
// cookie.
func startOIDC(t *testing.T, e *testEnv) (string, *http.Cookie) {
	t.Helper()
	start := e.serve(httptest.NewRequest(http.MethodGet, oidcLoginPath, nil))
	if start.Code != http.StatusFound {
		t.Fatalf("login start = %d %s", start.Code, start.Body)
	}
	for _, c := range start.Result().Cookies() {
		if c.Name == oidcStateCookie {
			if !c.HttpOnly || c.SameSite != http.SameSiteLaxMode {
				t.Fatalf("state cookie = %+v", c)
			}
			return start.Header().Get("Location"), c
		}
	}
	t.Fatal("no state cookie")
	return "", nil
}

// authorize lets the fake provider sign the user in and returns the
// callback request target.
func authorize(t *testing.T, authURL string) string {
	t.Helper()
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	back, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || !strings.HasPrefix(back.String(), testPublicURL+oidcCallbackPath) {
		t.Fatalf("provider redirected to %q", resp.Header.Get("Location"))
	}
	return back.RequestURI()
}

// oidcSignIn runs a complete browser sign-in of the user with claims and
// returns the callback's response.
func oidcSignIn(t *testing.T, e *testEnv, idp *oidctest.Provider, claims map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	idp.SetUser(claims)
	authURL, state := startOIDC(t, e)
	req := httptest.NewRequest(http.MethodGet, authorize(t, authURL), nil)
	req.AddCookie(state)
	return e.serve(req)
}

// signedIn checks a successful callback and keeps the session as name.
func signedIn(t *testing.T, e *testEnv, rec *httptest.ResponseRecorder, name string) {
	t.Helper()
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != testPublicURL+"/" {
		t.Fatalf("callback = %d → %q", rec.Code, rec.Header().Get("Location"))
	}
	e.tokens[name] = sessionCookieOf(t, rec).Value
}

// refusedWith checks a refused callback's redirect to the sign-in screen.
func refusedWith(t *testing.T, rec *httptest.ResponseRecorder, class string) {
	t.Helper()
	want := testPublicURL + "/?" + signInErrorParam + "=" + class
	if rec.Code != http.StatusFound || rec.Header().Get("Location") != want {
		t.Fatalf("callback = %d → %q, want %q", rec.Code, rec.Header().Get("Location"), want)
	}
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie && c.MaxAge >= 0 {
			t.Fatal("a refused sign-in set a session cookie")
		}
	}
}

func TestAuthMethods(t *testing.T) {
	e := newTestEnv(t, nil)
	expectBody(t, e.do(http.MethodGet, "/api/auth/methods", "", nil), `{"local":true,"providers":[]}`)

	e, _ = oidcEnv(t, nil)
	expectBody(t, e.do(http.MethodGet, "/api/auth/methods", "", nil),
		`{"local":true,"providers":[{"source":"oidc","name":"Fake IdP","type":"redirect","loginUrl":"/api/auth/oidc/login"}]}`)
}

func expectBody(t *testing.T, rec *httptest.ResponseRecorder, want string) {
	t.Helper()
	if rec.Code != http.StatusOK || strings.TrimSpace(rec.Body.String()) != want {
		t.Fatalf("got %d %s, want %s", rec.Code, rec.Body, want)
	}
}

func TestOIDCSignInAppliesTheMappings(t *testing.T) {
	e, idp := oidcEnv(t, nil)
	signedIn(t, e, oidcSignIn(t, e, idp, claimsOf("sub-carol", "carol", "kb-ops")), "carol")

	me := decode[meDetail](t, e.do(http.MethodGet, "/api/auth/me", "carol", nil))
	if me.Source != store.UserSourceOIDC || me.Role != store.RoleUser || me.MustChangePassword {
		t.Fatalf("me = %+v", me)
	}
	info := decode[infoView](t, e.do(http.MethodGet, "/api/info", "carol", nil))
	if info.AuthSource != "oidc" || info.Permissions.Role != store.ClusterRoleOperator || !info.Permissions.CanWrite {
		t.Fatalf("info = %+v", info)
	}
	expect(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=3", "carol", nil), http.StatusOK, "")

	// The next sign-in re-computes the role: now a viewer, refused mutations.
	signedIn(t, e, oidcSignIn(t, e, idp, claimsOf("sub-carol", "carol", "kb-dev")), "carol")
	expect(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=1", "carol", nil),
		http.StatusForbidden, plugin.CodeForbiddenRole)

	// And an admin group makes an admin.
	signedIn(t, e, oidcSignIn(t, e, idp, claimsOf("sub-carol", "carol", "kb-admins")), "carol")
	expect(t, e.do(http.MethodGet, "/api/users", "carol", nil), http.StatusOK, "")

	rows := e.audit()
	signIns := 0
	for _, r := range rows {
		if r.Action == auditSignInAction {
			signIns++
			if r.Username != "carol" || r.Target != "user/carol" || r.Status != http.StatusOK || r.Detail != `{"source":"oidc"}` {
				t.Fatalf("sign-in row %+v", r)
			}
		}
	}
	if signIns != 3 {
		t.Fatalf("%d sign-in rows", signIns)
	}
}

func TestOIDCLogoutEndsTheProviderSession(t *testing.T) {
	e, idp := oidcEnv(t, nil)
	signedIn(t, e, oidcSignIn(t, e, idp, claimsOf("s1", "carol", "kb-dev")), "carol")
	body := decode[map[string]any](t, e.do(http.MethodPost, "/api/auth/logout", "carol", nil))
	logout, _ := body["logoutUrl"].(string)
	u, err := url.Parse(logout)
	if err != nil || !strings.HasPrefix(logout, idp.URL+"/logout?") || u.Query().Get("id_token_hint") == "" ||
		u.Query().Get("post_logout_redirect_uri") != testPublicURL+"/" {
		t.Fatalf("logoutUrl = %q", logout)
	}
	expect(t, e.do(http.MethodGet, "/api/auth/me", "carol", nil), http.StatusUnauthorized, plugin.CodeSessionExpired)

	// Local sessions have no provider session to end.
	body = decode[map[string]any](t, e.do(http.MethodPost, "/api/auth/logout", "alice", nil))
	if _, ok := body["logoutUrl"]; ok || body["ok"] != true {
		t.Fatalf("local logout = %v", body)
	}
}

func TestOIDCCallbackRefusals(t *testing.T) {
	e, idp := oidcEnv(t, map[string]string{"KUBEBENCH_AUTH_REQUIRE_GROUP": "true"})

	// A callback without the browser's state cookie (login CSRF).
	idp.SetUser(claimsOf("s1", "carol", "kb-ops"))
	authURL, _ := startOIDC(t, e)
	refusedWith(t, e.serve(httptest.NewRequest(http.MethodGet, authorize(t, authURL), nil)), identity.ClassInvalidRequest)

	// A callback replayed after it was used.
	authURL, state := startOIDC(t, e)
	target := authorize(t, authURL)
	req := httptest.NewRequest(http.MethodGet, target, nil)
	req.AddCookie(state)
	signedIn(t, e, e.serve(req), "carol")
	req = httptest.NewRequest(http.MethodGet, target, nil)
	req.AddCookie(state)
	refusedWith(t, e.serve(req), identity.ClassInvalidRequest)

	// The provider refused the user.
	idp.SetDeny(true)
	refusedWith(t, oidcSignIn(t, e, idp, claimsOf("s1", "carol", "kb-ops")), identity.ClassProviderError)
	idp.SetDeny(false)

	// No mapped group with KUBEBENCH_AUTH_REQUIRE_GROUP: refused, and the
	// existing session ends.
	refusedWith(t, oidcSignIn(t, e, idp, claimsOf("s1", "carol", "other")), identity.ClassNoMatchingGroup)
	expect(t, e.do(http.MethodGet, "/api/auth/me", "carol", nil), http.StatusUnauthorized, plugin.CodeSessionExpired)

	// A local account's name is never taken over.
	refusedWith(t, oidcSignIn(t, e, idp, claimsOf("s2", "alice", "kb-admins")), identity.ClassUsernameConflict)

	var reasons []string
	for _, r := range e.audit() {
		if r.Action == auditSignInAction && r.Status != http.StatusOK {
			reasons = append(reasons, r.Detail)
		}
	}
	for _, class := range []string{identity.ClassInvalidRequest, identity.ClassProviderError,
		identity.ClassNoMatchingGroup, identity.ClassUsernameConflict} {
		if !strings.Contains(strings.Join(reasons, " "), `"reason":"`+class+`"`) {
			t.Fatalf("no audit row for %s in %v", class, reasons)
		}
	}
}

// Callbacks that never reach the provider's token endpoint (no or another
// browser's state, a junk code) prove nothing and cost nothing, so they do
// not count against the client IP: otherwise anyone could lock SSO for
// everyone behind the same address.
func TestOIDCJunkCallbacksDoNotLockTheIP(t *testing.T) {
	e, idp := oidcEnv(t, nil)
	for range 2 * loginMaxFailures {
		refusedWith(t, e.serve(httptest.NewRequest(http.MethodGet, oidcCallbackPath+"?state=x&code=y", nil)),
			identity.ClassInvalidRequest)
		_, state := startOIDC(t, e)
		req := httptest.NewRequest(http.MethodGet, oidcCallbackPath+"?state="+url.QueryEscape(state.Value)+"&code=junk", nil)
		req.AddCookie(state)
		refusedWith(t, e.serve(req), identity.ClassInvalidRequest)
	}
	signedIn(t, e, oidcSignIn(t, e, idp, claimsOf("s1", "carol", "kb-dev")), "carol")
}

// Sign-ins the provider completed and KubeBench refused count against the
// client IP.
func TestOIDCCallbackRateLimit(t *testing.T) {
	e, idp := oidcEnv(t, map[string]string{"KUBEBENCH_AUTH_REQUIRE_GROUP": "true"})
	for range loginMaxFailures {
		refusedWith(t, oidcSignIn(t, e, idp, claimsOf("s1", "carol", "other")), identity.ClassNoMatchingGroup)
	}
	refusedWith(t, oidcSignIn(t, e, idp, claimsOf("s1", "carol", "kb-dev")), identity.ClassRateLimited)
}

// Starting sign-ins keeps nothing on the server, so a flood of starts
// cannot push out a sign-in in progress.
func TestOIDCStartFloodKeepsPendingSignIns(t *testing.T) {
	e, idp := oidcEnv(t, nil)
	idp.SetUser(claimsOf("s1", "carol", "kb-dev"))
	authURL, state := startOIDC(t, e)
	for range 10001 {
		e.serve(httptest.NewRequest(http.MethodGet, oidcLoginPath, nil))
	}
	req := httptest.NewRequest(http.MethodGet, authorize(t, authURL), nil)
	req.AddCookie(state)
	signedIn(t, e, e.serve(req), "carol")
}

func TestExternalUsersHaveNoLocalPassword(t *testing.T) {
	e, idp := oidcEnv(t, nil)
	signedIn(t, e, oidcSignIn(t, e, idp, claimsOf("s1", "carol", "kb-dev")), "carol")
	expect(t, e.serve(loginRequest("carol", "")), http.StatusUnauthorized, plugin.CodeUnauthenticated)
	expect(t, e.do(http.MethodPost, "/api/auth/password", "carol",
		map[string]string{"currentPassword": "x", "newPassword": "new-password-1"}), http.StatusBadRequest, plugin.CodeBadRequest)
	carol := findUser(t, e, "carol")
	if carol.Source != "oidc" || carol.Email != "carol@example.com" {
		t.Fatalf("user view = %+v", carol)
	}
	expect(t, e.do(http.MethodPatch, fmt.Sprintf("/api/users/%d", carol.ID), "admin",
		map[string]string{"password": "new-password-1"}), http.StatusBadRequest, plugin.CodeBadRequest)
}

func TestBreakGlassAdminStays(t *testing.T) {
	e, idp := oidcEnv(t, nil)
	signedIn(t, e, oidcSignIn(t, e, idp, claimsOf("s1", "boss", "kb-admins")), "boss")
	admin := e.users["admin"]
	// The external admin cannot remove the last local admin.
	expect(t, e.do(http.MethodDelete, fmt.Sprintf("/api/users/%d", admin), "boss", nil), http.StatusConflict, plugin.CodeConflict)
	expect(t, e.do(http.MethodPatch, fmt.Sprintf("/api/users/%d", admin), "boss", map[string]any{"role": "user"}),
		http.StatusConflict, plugin.CodeConflict)
	expect(t, e.serve(loginRequest("admin", testPassword)), http.StatusOK, "")
}

func TestAuthSettingsNeverReturnTheSecret(t *testing.T) {
	e, idp := oidcEnv(t, nil)
	expect(t, e.do(http.MethodGet, "/api/settings/auth", "alice", nil), http.StatusForbidden, plugin.CodeAdminRequired)
	rec := e.do(http.MethodGet, "/api/settings/auth", "admin", nil)
	expect(t, rec, http.StatusOK, "")
	if strings.Contains(rec.Body.String(), oidctest.ClientSecret) {
		t.Fatal("the client secret was returned")
	}
	view := decode[authSettingsView](t, rec)
	if len(view.Providers) != 1 || !view.Providers[0].Status.OK || !view.PublicURLConfigured {
		t.Fatalf("settings = %+v", view)
	}
	settings := map[string]providerSetting{}
	for _, s := range view.Providers[0].Settings {
		settings[s.Key] = s
	}
	if settings["issuer"].Value != idp.URL || settings["redirectUrl"].Value != testPublicURL+oidcCallbackPath ||
		settings["clientSecret"].SecretSet == nil || !*settings["clientSecret"].SecretSet || settings["clientSecret"].Value != "" {
		t.Fatalf("settings = %+v", settings)
	}
	signedIn(t, e, oidcSignIn(t, e, idp, claimsOf("s1", "carol", "kb-dev")), "carol")
	for _, r := range e.audit() {
		if strings.Contains(r.Detail+r.Target, oidctest.ClientSecret) {
			t.Fatalf("audit row holds the secret: %+v", r)
		}
	}
}

func TestGroupMappingRoutes(t *testing.T) {
	e := newTestEnv(t, nil)
	body := map[string]string{"group": "kb-ops", "role": "operator", "cluster": "test"}
	expect(t, e.do(http.MethodPost, "/api/settings/auth/mappings", "alice", body), http.StatusForbidden, plugin.CodeAdminRequired)
	created := decode[map[string]int64](t, e.do(http.MethodPost, "/api/settings/auth/mappings", "admin", body))
	expect(t, e.do(http.MethodPost, "/api/settings/auth/mappings", "admin", body), http.StatusConflict, plugin.CodeConflict)
	expect(t, e.do(http.MethodPost, "/api/settings/auth/mappings", "admin",
		map[string]string{"group": "x", "role": "viewer"}), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/settings/auth/mappings", "admin",
		map[string]string{"group": "future", "role": "viewer", "cluster": "not-yet"}), http.StatusOK, "")

	list := decode[[]mappingView](t, e.do(http.MethodGet, "/api/settings/auth/mappings", "admin", nil))
	if len(list) != 2 || list[0].Group != "future" || list[0].ClusterExists || !list[1].ClusterExists {
		t.Fatalf("list = %+v", list)
	}
	path := fmt.Sprintf("/api/settings/auth/mappings/%d", created["id"])
	expect(t, e.do(http.MethodPut, path, "admin", map[string]string{"group": "kb-admins", "role": "admin"}), http.StatusOK, "")
	expect(t, e.do(http.MethodDelete, path, "admin", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodDelete, path, "admin", nil), http.StatusNotFound, plugin.CodeNotFound)

	var rows []string
	for _, r := range e.audit() {
		if strings.Contains(r.Action, "/api/settings/auth/mappings") {
			rows = append(rows, fmt.Sprintf("%s %s %d %s", r.Action, r.Target, r.Status, r.Detail))
		}
	}
	want := []string{
		`POST /api/settings/auth/mappings mapping 403 admin_required: this action needs an administrator`,
		`POST /api/settings/auth/mappings mapping/kb-ops 200 {"cluster":"test","role":"operator","source":""}`,
		`PUT /api/settings/auth/mappings/{id} mapping/kb-admins 200 {"cluster":"","role":"admin","source":""}`,
		`DELETE /api/settings/auth/mappings/{id} mapping/kb-admins 200 {"cluster":"","role":"admin","source":""}`,
	}
	joined := strings.Join(rows, "\n")
	for _, w := range want {
		if !strings.Contains(joined, w) {
			t.Fatalf("missing audit row %q in\n%s", w, joined)
		}
	}
}

func TestPreviewSignIn(t *testing.T) {
	e, idp := oidcEnv(t, nil)
	signedIn(t, e, oidcSignIn(t, e, idp, claimsOf("s1", "carol", "kb-ops")), "carol")
	usersBefore := len(decode[[]userView](t, e.do(http.MethodGet, "/api/users", "admin", nil)))

	p := decode[previewView](t, e.do(http.MethodPost, "/api/settings/auth/preview", "admin",
		map[string]any{"source": "oidc", "username": "carol"}))
	if p.GroupsFrom != identity.GroupsFromLastSignIn || p.Admin || len(p.Clusters) != 1 ||
		p.Clusters[0] != (previewGrant{Cluster: "test", Role: "operator", Exists: true}) || !p.Allowed || p.ExistingSource != "oidc" {
		t.Fatalf("preview = %+v", p)
	}
	p = decode[previewView](t, e.do(http.MethodPost, "/api/settings/auth/preview", "admin",
		map[string]any{"source": "oidc", "username": "dave", "groups": []string{"KB-Admins", " "}}))
	if !p.Admin || p.GroupsFrom != identity.GroupsFromInput || len(p.Groups) != 1 || !p.Allowed {
		t.Fatalf("preview = %+v", p)
	}
	p = decode[previewView](t, e.do(http.MethodPost, "/api/settings/auth/preview", "admin",
		map[string]any{"source": "oidc", "username": "alice", "groups": []string{"kb-admins"}}))
	if p.Allowed || p.Reason != identity.ClassUsernameConflict {
		t.Fatalf("preview = %+v", p)
	}
	expect(t, e.do(http.MethodPost, "/api/settings/auth/preview", "admin", map[string]any{"source": "saml", "username": "x"}),
		http.StatusBadRequest, plugin.CodeBadRequest)
	if n := len(decode[[]userView](t, e.do(http.MethodGet, "/api/users", "admin", nil))); n != usersBefore {
		t.Fatal("a preview created a user")
	}
}

// fakeDirectory is a password provider such as a directory: alice and
// bob may sign in with "secret-pw"; bob's account is disabled.
type fakeDirectory struct{}

func (fakeDirectory) Source() string { return store.UserSourceLDAP }
func (fakeDirectory) Name() string   { return "Directory" }
func (fakeDirectory) Authenticate(_ context.Context, username, password string) (*identity.Identity, error) {
	switch {
	case username == "bob":
		return nil, &identity.Failure{Class: identity.ClassAccountDisabled, Subject: "cn=bob"}
	case username != "dora" || password != "secret-pw":
		return nil, identity.Fail(identity.ClassInvalidCredentials, "bind failed")
	}
	return &identity.Identity{Source: store.UserSourceLDAP, Subject: "cn=dora", Username: "dora", Groups: []string{"kb-ops"}}, nil
}

func directoryLogin(user, password string) *http.Request {
	body, _ := json.Marshal(map[string]string{"source": "ldap", "username": user, "password": password})
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func TestPasswordProviderSignIn(t *testing.T) {
	e, _ := oidcEnv(t, nil)
	e.srv.ident.passwords[store.UserSourceLDAP] = fakeDirectory{}

	methods := decode[struct{ Providers []signInMethod }](t, e.do(http.MethodGet, "/api/auth/methods", "", nil))
	if len(methods.Providers) != 2 || methods.Providers[1] != (signInMethod{Source: "ldap", Name: "Directory", Type: "password"}) {
		t.Fatalf("methods = %+v", methods)
	}

	rec := e.serve(directoryLogin("dora", "secret-pw"))
	expect(t, rec, http.StatusOK, "")
	e.tokens["dora"] = sessionCookieOf(t, rec).Value
	expect(t, e.do(http.MethodGet, "/api/info", "dora", nil), http.StatusOK, "") // activates the cluster, like the app's boot
	expect(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=3", "dora", nil), http.StatusOK, "")
	expect(t, e.serve(directoryLogin("dora", "wrong")), http.StatusUnauthorized, plugin.CodeUnauthenticated)

	// A disabled account is refused and loses its sessions.
	bob, err := e.store.SignInExternal(store.ExternalAccount{Source: "ldap", Subject: "cn=bob", Username: "ldap-bob", Role: store.RoleUser})
	if err != nil {
		t.Fatal(err)
	}
	token, _ := e.store.CreateSession(bob.ID, e.cfg.SessionTTL)
	expect(t, e.serve(directoryLogin("bob", "secret-pw")), http.StatusForbidden, plugin.CodeForbidden)
	if _, err := e.store.UserBySession(token); err == nil {
		t.Fatal("the disabled account's session survived")
	}

	expect(t, e.serve(directoryLogin("mallory", "x")), http.StatusUnauthorized, plugin.CodeUnauthenticated)
	body := `{"source":"saml","username":"x","password":"y"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	expect(t, e.serve(req), http.StatusBadRequest, plugin.CodeBadRequest)

	// Failed directory sign-ins are throttled like local ones.
	for range loginMaxFailures {
		e.serve(directoryLogin("eve", "x"))
	}
	expect(t, e.serve(directoryLogin("eve", "x")), http.StatusTooManyRequests, plugin.CodeRateLimited)

	var details []string
	for _, r := range e.audit() {
		if r.Action == auditSignInAction {
			details = append(details, fmt.Sprintf("%s %d %s", r.Username, r.Status, r.Detail))
		}
	}
	joined := strings.Join(details, "\n")
	for _, w := range []string{
		`dora 200 {"source":"ldap"}`,
		`dora 401 {"reason":"invalid_credentials","source":"ldap"}`,
		`bob 403 {"reason":"account_disabled","source":"ldap"}`,
		`eve 429 {"reason":"rate_limited","source":"ldap"}`,
	} {
		if !strings.Contains(joined, w) {
			t.Fatalf("missing %q in\n%s", w, joined)
		}
	}
	if strings.Contains(joined, "secret-pw") {
		t.Fatal("a password reached the audit log")
	}
}
