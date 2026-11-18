package oidcauth

import (
	"context"
	"net/http"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"testing"
	"time"

	"kubebench/internal/config"
	"kubebench/internal/identity"
	"kubebench/internal/identity/oidcauth/oidctest"
)

const redirectURL = "https://kubebench.test/api/auth/oidc/callback"

func newProvider(t *testing.T, idp *oidctest.Provider, tweak func(*config.OIDCConfig)) *Provider {
	t.Helper()
	cfg := config.OIDCConfig{
		Issuer: idp.URL, ClientID: oidctest.ClientID, ClientSecret: oidctest.ClientSecret,
		Scopes: config.DefaultOIDCScopes, UsernameClaim: "preferred_username", GroupsClaim: "groups", Name: "Fake",
	}
	if tweak != nil {
		tweak(&cfg)
	}
	p, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

// signIn runs the browser's part: start, the provider's authorization
// endpoint, and returns the callback's code and state.
func signIn(t *testing.T, p *Provider) (*Login, url.Values) {
	t.Helper()
	login, authURL, err := p.Start(context.Background(), redirectURL)
	if err != nil {
		t.Fatal(err)
	}
	u, _ := url.Parse(authURL)
	q := u.Query()
	if q.Get("state") != login.State || q.Get("nonce") != login.Nonce || q.Get("code_challenge_method") != "S256" {
		t.Fatalf("authorization URL lacks state, nonce or PKCE: %s", authURL)
	}
	if !strings.Contains(q.Get("scope"), "openid") || q.Get("redirect_uri") != redirectURL {
		t.Fatalf("authorization URL scope/redirect: %s", authURL)
	}
	client := &http.Client{CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Get(authURL)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	back, err := url.Parse(resp.Header.Get("Location"))
	if err != nil || back.Query().Get("state") != login.State {
		t.Fatalf("callback = %q", resp.Header.Get("Location"))
	}
	return login, back.Query()
}

func TestFinishReadsTheIdentity(t *testing.T) {
	idp := oidctest.New(t)
	idp.SetUser(map[string]any{
		"sub": "u-1", "preferred_username": "alice", "name": "Alice A", "email": "alice@example.com",
		"groups": []string{"kb-admins", "devs"},
	})
	p := newProvider(t, idp, nil)
	login, q := signIn(t, p)
	id, rawIDToken, err := p.Finish(context.Background(), login, q.Get("code"))
	if err != nil {
		t.Fatal(err)
	}
	if id.Subject != "u-1" || id.Username != "alice" || id.DisplayName != "Alice A" || id.Email != "alice@example.com" ||
		id.Source != "oidc" || !slices.Equal(id.Groups, []string{"kb-admins", "devs"}) {
		t.Fatalf("identity = %+v", id)
	}
	if rawIDToken == "" {
		t.Fatal("no raw ID token")
	}
	// A code is redeemed once.
	if _, _, err := p.Finish(context.Background(), login, q.Get("code")); identity.ClassOf(err) != identity.ClassInvalidRequest {
		t.Fatalf("second redemption: %v", err)
	}
}

func TestFinishRefusesAWrongVerifierOrNonce(t *testing.T) {
	idp := oidctest.New(t)
	idp.SetUser(map[string]any{"sub": "u-1", "preferred_username": "alice"})
	p := newProvider(t, idp, nil)

	login, q := signIn(t, p)
	tampered := *login
	tampered.Verifier = "another-verifier-of-sufficient-length-0123456789"
	if _, _, err := p.Finish(context.Background(), &tampered, q.Get("code")); identity.ClassOf(err) != identity.ClassInvalidRequest {
		t.Fatalf("wrong PKCE verifier: %v", err)
	}

	idp.SetNonce("replayed")
	login, q = signIn(t, p)
	if _, _, err := p.Finish(context.Background(), login, q.Get("code")); identity.ClassOf(err) != identity.ClassInvalidRequest {
		t.Fatalf("wrong nonce: %v", err)
	}
}

func TestGroupsFromUserinfoAndDottedClaims(t *testing.T) {
	idp := oidctest.New(t)
	idp.SetUser(map[string]any{"sub": "u-2", "preferred_username": "bob"})
	idp.SetUserInfoOnly(map[string]any{"realm_access": map[string]any{"roles": []string{"ops"}}})
	p := newProvider(t, idp, func(c *config.OIDCConfig) { c.GroupsClaim = "realm_access.roles" })
	login, q := signIn(t, p)
	id, _, err := p.Finish(context.Background(), login, q.Get("code"))
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(id.Groups, []string{"ops"}) {
		t.Fatalf("groups = %v", id.Groups)
	}
}

func TestAllowedDomains(t *testing.T) {
	idp := oidctest.New(t)
	p := newProvider(t, idp, func(c *config.OIDCConfig) { c.AllowedDomains = []string{"example.com"} })
	cases := []struct {
		claims map[string]any
		class  string
	}{
		{map[string]any{"email": "a@example.com", "email_verified": true}, ""},
		{map[string]any{"email": "a@EXAMPLE.com", "email_verified": true}, ""},
		{map[string]any{"email": "a@example.com", "email_verified": "True"}, ""},
		{map[string]any{"email": "a@example.com"}, identity.ClassDomainNotAllowed},
		{map[string]any{"email": "a@example.com", "email_verified": false}, identity.ClassDomainNotAllowed},
		{map[string]any{"email": "a@example.com", "email_verified": "false"}, identity.ClassDomainNotAllowed},
		{map[string]any{"email": "a@example.com", "email_verified": 1}, identity.ClassDomainNotAllowed},
		{map[string]any{"email": "a@evil.test", "email_verified": true}, identity.ClassDomainNotAllowed},
		{map[string]any{}, identity.ClassDomainNotAllowed},
	}
	for _, tc := range cases {
		claims := map[string]any{"sub": "u-3", "preferred_username": "carol"}
		for k, v := range tc.claims {
			claims[k] = v
		}
		idp.SetUser(claims)
		login, q := signIn(t, p)
		_, _, err := p.Finish(context.Background(), login, q.Get("code"))
		if tc.class == "" && err != nil {
			t.Errorf("%v: %v", tc.claims, err)
		}
		if tc.class != "" {
			f, ok := err.(*identity.Failure)
			if !ok || f.Class != tc.class || f.Subject != "u-3" {
				t.Errorf("%v: err = %v, want %s with the subject", tc.claims, err, tc.class)
			}
		}
	}
}

func TestMissingUsernameClaim(t *testing.T) {
	idp := oidctest.New(t)
	idp.SetUser(map[string]any{"sub": "u-4"})
	p := newProvider(t, idp, nil)
	login, q := signIn(t, p)
	if _, _, err := p.Finish(context.Background(), login, q.Get("code")); identity.ClassOf(err) != identity.ClassInvalidUsername {
		t.Fatalf("err = %v", err)
	}
}

func TestLogoutURL(t *testing.T) {
	idp := oidctest.New(t)
	p := newProvider(t, idp, nil)
	got, err := url.Parse(p.LogoutURL(context.Background(), "the-id-token", "https://kubebench.test/"))
	if err != nil {
		t.Fatal(err)
	}
	q := got.Query()
	if got.Path != "/logout" || q.Get("id_token_hint") != "the-id-token" || q.Get("client_id") != oidctest.ClientID ||
		q.Get("post_logout_redirect_uri") != "https://kubebench.test/" {
		t.Fatalf("logout URL = %s", got)
	}
	if !p.SupportsLogout() {
		t.Fatal("SupportsLogout = false")
	}
}

func TestUnreachableProvider(t *testing.T) {
	idp := oidctest.New(t)
	p := newProvider(t, idp, nil)
	idp.Close()
	if _, _, err := p.Start(context.Background(), redirectURL); identity.ClassOf(err) != identity.ClassProviderError {
		t.Fatalf("err = %v", err)
	}
	if p.LogoutURL(context.Background(), "", "https://kubebench.test/") != "" {
		t.Fatal("logout URL without discovery")
	}
}

func TestLogins(t *testing.T) {
	now := time.Unix(1000, 0)
	l := NewLogins(func() time.Time { return now })
	l.Put(&Login{State: "a"})
	if _, ok := l.Take("a"); !ok {
		t.Fatal("pending sign-in not found")
	}
	if _, ok := l.Take("a"); ok {
		t.Fatal("a sign-in was taken twice")
	}
	l.Put(&Login{State: "b"})
	now = now.Add(loginTTL)
	if _, ok := l.Take("b"); ok {
		t.Fatal("an expired sign-in was accepted")
	}
	for i := range maxLogins + 5 {
		l.Put(&Login{State: strconv.Itoa(i)})
	}
	if len(l.pending) > maxLogins {
		t.Fatalf("%d pending sign-ins kept", len(l.pending))
	}
}
