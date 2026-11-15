package api

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"kubebench/internal/config"
	"kubebench/internal/plugin"
)

func loginRequest(user, password string) *http.Request {
	body := `{"username":"` + user + `","password":"` + password + `"}`
	req := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	return req
}

func sessionCookieOf(t *testing.T, rec *httptest.ResponseRecorder) *http.Cookie {
	t.Helper()
	for _, c := range rec.Result().Cookies() {
		if c.Name == sessionCookie {
			return c
		}
	}
	t.Fatalf("no session cookie in %v", rec.Header())
	return nil
}

func TestLoginSetsTheSessionCookie(t *testing.T) {
	e := newTestEnv(t, nil)
	rec := e.serve(loginRequest("alice", testPassword))
	expect(t, rec, http.StatusOK, "")
	cookie := sessionCookieOf(t, rec)
	if !cookie.HttpOnly || cookie.SameSite != http.SameSiteLaxMode || cookie.Path != "/" || cookie.Secure {
		t.Fatalf("cookie = %+v", cookie)
	}
	if cookie.MaxAge != int(e.cfg.SessionTTL.Seconds()) {
		t.Fatalf("MaxAge = %d", cookie.MaxAge)
	}
	me := decode[meView](t, rec)
	if me.Username != "alice" || me.Role != "user" || me.MustChangePassword {
		t.Fatalf("login body = %+v", me)
	}
	e.tokens["alice"] = cookie.Value
	expect(t, e.do(http.MethodGet, "/api/pods", "alice", nil), http.StatusOK, "")

	expect(t, e.serve(loginRequest("alice", "wrong-password")), http.StatusUnauthorized, plugin.CodeUnauthenticated)
	expect(t, e.serve(loginRequest("nobody", testPassword)), http.StatusUnauthorized, plugin.CodeUnauthenticated)
}

func TestSecureCookieModes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		env    map[string]string
		tls    bool
		proto  string
		secure bool
	}{
		{"auto over http", nil, false, "", false},
		{"auto over tls", nil, true, "", true},
		{"auto ignores an untrusted proxy header", nil, false, "https", false},
		{"auto behind a trusted proxy", map[string]string{"KUBEBENCH_TRUST_PROXY": "true"}, false, "https", true},
		{"forced on", map[string]string{"KUBEBENCH_SECURE_COOKIES": "true"}, false, "", true},
		{"forced off", map[string]string{"KUBEBENCH_SECURE_COOKIES": "false"}, true, "", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			e := newTestEnv(t, tc.env)
			req := loginRequest("alice", testPassword)
			if tc.tls {
				req.TLS = &tls.ConnectionState{}
			}
			if tc.proto != "" {
				req.Header.Set("X-Forwarded-Proto", tc.proto)
			}
			if got := sessionCookieOf(t, e.serve(req)).Secure; got != tc.secure {
				t.Fatalf("Secure = %v, want %v", got, tc.secure)
			}
		})
	}
}

func TestLoginRateLimit(t *testing.T) {
	e := newTestEnv(t, nil)
	for i := 0; i < loginMaxFailures; i++ {
		expect(t, e.serve(loginRequest("alice", "wrong-password")), http.StatusUnauthorized, plugin.CodeUnauthenticated)
	}
	rec := e.serve(loginRequest("alice", testPassword))
	expect(t, rec, http.StatusTooManyRequests, plugin.CodeRateLimited)
	if rec.Header().Get("Retry-After") != "900" {
		t.Fatalf("Retry-After = %q", rec.Header().Get("Retry-After"))
	}
	// Other usernames and other clients are not affected.
	expect(t, e.serve(loginRequest("bob", testPassword)), http.StatusOK, "")
	other := loginRequest("alice", testPassword)
	other.RemoteAddr = "192.0.2.99:1234"
	expect(t, e.serve(other), http.StatusOK, "")

	e.clock.Advance(loginWindow)
	expect(t, e.serve(loginRequest("alice", testPassword)), http.StatusOK, "")
	// A success clears the failures.
	for i := 0; i < loginMaxFailures-1; i++ {
		e.serve(loginRequest("alice", "wrong-password"))
	}
	expect(t, e.serve(loginRequest("alice", testPassword)), http.StatusOK, "")
	e.serve(loginRequest("alice", "wrong-password"))
	expect(t, e.serve(loginRequest("alice", testPassword)), http.StatusOK, "")
}

func TestLimiterTrustsOnlyTheProxysAddress(t *testing.T) {
	e := newTestEnv(t, map[string]string{"KUBEBENCH_TRUST_PROXY": "true"})
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Forwarded-For", "203.0.113.9, 198.51.100.7")
	if ip := e.srv.clientIP(req); ip != "198.51.100.7" {
		t.Fatalf("clientIP = %q; the client controls all but the last entry", ip)
	}
	e = newTestEnv(t, nil)
	if ip := e.srv.clientIP(req); ip != "192.0.2.1" {
		t.Fatalf("without a trusted proxy the header is ignored, got %q", ip)
	}
}

func TestChangePassword(t *testing.T) {
	e := newTestEnv(t, nil)
	other := e.tokens["alice"]
	rec := e.serve(loginRequest("alice", testPassword))
	e.tokens["alice"] = sessionCookieOf(t, rec).Value

	change := func(current, next string) *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/api/auth/password", "alice",
			map[string]string{"currentPassword": current, "newPassword": next})
	}
	expect(t, change("wrong-password", "another-password"), http.StatusForbidden, plugin.CodeForbidden)
	expect(t, change(testPassword, "short"), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, change(testPassword, testPassword), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, change(testPassword, strings.Repeat("x", 73)), http.StatusBadRequest, plugin.CodeBadRequest)
	if err := e.store.SetMustChangePassword(e.users["alice"], true); err != nil {
		t.Fatal(err)
	}
	expect(t, change(testPassword, "another-password"), http.StatusOK, "")

	me := decode[meView](t, e.do(http.MethodGet, "/api/auth/me", "alice", nil))
	if me.MustChangePassword {
		t.Fatal("the forced change was not cleared")
	}
	e.tokens["old"] = other
	expect(t, e.do(http.MethodGet, "/api/pods", "old", nil), http.StatusUnauthorized, plugin.CodeSessionExpired)
	expect(t, e.serve(loginRequest("alice", "another-password")), http.StatusOK, "")
}

func TestOriginCheck(t *testing.T) {
	e := newTestEnv(t, nil)
	post := func(origin string) *httptest.ResponseRecorder {
		req := loginRequest("alice", testPassword)
		req.Header.Set("Origin", origin)
		return e.serve(req)
	}
	expect(t, post("http://evil.example"), http.StatusForbidden, plugin.CodeForbidden)
	expect(t, post("null"), http.StatusForbidden, plugin.CodeForbidden)
	expect(t, post("http://example.com"), http.StatusOK, "") // httptest's Host
	expect(t, post("http://example.com:80"), http.StatusOK, "")

	get := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	get.Header.Set("Origin", "http://evil.example")
	expect(t, e.serve(get), http.StatusOK, "")

	ws := httptest.NewRequest(http.MethodGet, "/api/version", nil)
	ws.Header.Set("Origin", "http://evil.example")
	ws.Header.Set("Connection", "keep-alive, Upgrade")
	ws.Header.Set("Upgrade", "websocket")
	expect(t, e.serve(ws), http.StatusForbidden, plugin.CodeForbidden)

	// Behind a trusted proxy the forwarded host counts too.
	e = newTestEnv(t, map[string]string{"KUBEBENCH_TRUST_PROXY": "true"})
	proxied := loginRequest("alice", testPassword)
	proxied.Host = "kubebench.internal:9876"
	proxied.Header.Set("X-Forwarded-Host", "bench.example.org")
	proxied.Header.Set("Origin", "https://bench.example.org")
	expect(t, e.serve(proxied), http.StatusOK, "")
}

func TestLoginAfterAdminResetMustChangePassword(t *testing.T) {
	e := newTestEnv(t, nil)
	expect(t, e.do(http.MethodPatch, "/api/users/2", "admin", map[string]string{"password": "temporary-pass"}), http.StatusOK, "")
	rec := e.serve(loginRequest("alice", "temporary-pass"))
	expect(t, rec, http.StatusOK, "")
	if !decode[meView](t, rec).MustChangePassword {
		t.Fatal("a reset password must be changed at the next sign-in")
	}
	e.tokens["alice"] = sessionCookieOf(t, rec).Value
	expect(t, e.do(http.MethodGet, "/api/pods", "alice", nil), http.StatusForbidden, plugin.CodePasswordChangeRequired)
}

// An account that still uses the default password without the flag (as
// after an upgrade from v0.4) is forced to change it at its next sign-in.
func TestLoginWithDefaultPasswordForcesChange(t *testing.T) {
	e := newTestEnv(t, nil)
	alice, err := e.store.Authenticate("alice", testPassword)
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetPassword(alice.ID, config.DefaultAdminPassword, false, ""); err != nil {
		t.Fatal(err)
	}
	rec := e.serve(loginRequest("alice", config.DefaultAdminPassword))
	expect(t, rec, http.StatusOK, "")
	if !decode[meView](t, rec).MustChangePassword {
		t.Fatal("the default password must be changed at sign-in")
	}
	e.tokens["alice"] = sessionCookieOf(t, rec).Value
	expect(t, e.do(http.MethodGet, "/api/pods", "alice", nil), http.StatusForbidden, plugin.CodePasswordChangeRequired)
}
