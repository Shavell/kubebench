// Package oidctest is a fake OpenID Connect provider for tests: discovery,
// JWKS, an authorization endpoint that signs in a configured user at once,
// a token endpoint that checks the client secret and the PKCE verifier,
// userinfo and an end-session endpoint.
package oidctest

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"net/url"
	"sync"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
)

// ClientID and ClientSecret are the client the fake provider accepts.
const (
	ClientID     = "kubebench"
	ClientSecret = "fake-client-secret"
)

// Provider is a running fake provider.
type Provider struct {
	*httptest.Server
	key *rsa.PrivateKey

	mu sync.Mutex
	// claims are the claims of the user the next sign-in returns; sub is
	// required.
	claims map[string]any
	// userInfoOnly are claims served only by userinfo, not in the ID token.
	userInfoOnly map[string]any
	// nonce overrides the nonce put into ID tokens when non-empty.
	nonce string
	// deny makes the authorization endpoint answer error=access_denied.
	deny  bool
	codes map[string]grant
	// TokenRequests counts token endpoint calls.
	TokenRequests int
}

type grant struct {
	nonce, challenge, redirectURI string
}

// New starts a fake provider; it stops with the test.
func New(t *testing.T) *Provider {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	p := &Provider{key: key, codes: map[string]grant{}, claims: map[string]any{}}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /.well-known/openid-configuration", p.discovery)
	mux.HandleFunc("GET /jwks", p.jwks)
	mux.HandleFunc("GET /authorize", p.authorize)
	mux.HandleFunc("POST /token", p.token)
	mux.HandleFunc("GET /userinfo", p.userinfo)
	p.Server = httptest.NewServer(mux)
	t.Cleanup(p.Close)
	return p
}

// SetUser sets the claims of the user the next sign-in returns.
func (p *Provider) SetUser(claims map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.claims = claims
}

// SetUserInfoOnly sets claims that only the userinfo endpoint returns.
func (p *Provider) SetUserInfoOnly(claims map[string]any) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.userInfoOnly = claims
}

// SetNonce makes ID tokens carry nonce instead of the requested one.
func (p *Provider) SetNonce(nonce string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.nonce = nonce
}

// SetDeny makes the provider refuse sign-ins with error=access_denied.
func (p *Provider) SetDeny(deny bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.deny = deny
}

func (p *Provider) discovery(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, map[string]any{
		"issuer":                                p.URL,
		"authorization_endpoint":                p.URL + "/authorize",
		"token_endpoint":                        p.URL + "/token",
		"jwks_uri":                              p.URL + "/jwks",
		"userinfo_endpoint":                     p.URL + "/userinfo",
		"end_session_endpoint":                  p.URL + "/logout",
		"id_token_signing_alg_values_supported": []string{"RS256"},
		"code_challenge_methods_supported":      []string{"S256"},
	})
}

func (p *Provider) jwks(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, jose.JSONWebKeySet{Keys: []jose.JSONWebKey{
		{Key: &p.key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"},
	}})
}

// authorize signs the configured user in at once and redirects back.
func (p *Provider) authorize(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	redirect, err := url.Parse(q.Get("redirect_uri"))
	if err != nil || q.Get("client_id") != ClientID || q.Get("response_type") != "code" ||
		q.Get("code_challenge_method") != "S256" || q.Get("code_challenge") == "" {
		http.Error(w, "bad authorization request", http.StatusBadRequest)
		return
	}
	back := redirect.Query()
	back.Set("state", q.Get("state"))
	p.mu.Lock()
	if p.deny {
		back.Set("error", "access_denied")
	} else {
		code := randomString()
		p.codes[code] = grant{nonce: q.Get("nonce"), challenge: q.Get("code_challenge"), redirectURI: q.Get("redirect_uri")}
		back.Set("code", code)
	}
	p.mu.Unlock()
	redirect.RawQuery = back.Encode()
	http.Redirect(w, r, redirect.String(), http.StatusFound)
}

func (p *Provider) token(w http.ResponseWriter, r *http.Request) {
	id, secret, ok := r.BasicAuth()
	if !ok {
		id, secret = r.FormValue("client_id"), r.FormValue("client_secret")
	}
	if id != ClientID || secret != ClientSecret {
		tokenError(w, "invalid_client")
		return
	}
	p.mu.Lock()
	defer p.mu.Unlock()
	p.TokenRequests++
	g, ok := p.codes[r.FormValue("code")]
	delete(p.codes, r.FormValue("code"))
	sum := sha256.Sum256([]byte(r.FormValue("code_verifier")))
	if !ok || g.redirectURI != r.FormValue("redirect_uri") ||
		base64.RawURLEncoding.EncodeToString(sum[:]) != g.challenge {
		tokenError(w, "invalid_grant")
		return
	}
	nonce := g.nonce
	if p.nonce != "" {
		nonce = p.nonce
	}
	claims := map[string]any{
		"iss": p.URL, "aud": ClientID, "nonce": nonce,
		"iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
	}
	for k, v := range p.claims {
		claims[k] = v
	}
	idToken, err := p.sign(claims)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, map[string]any{
		"access_token": "access-" + randomString(), "token_type": "Bearer", "expires_in": 300, "id_token": idToken,
	})
}

func (p *Provider) userinfo(w http.ResponseWriter, _ *http.Request) {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := map[string]any{"sub": p.claims["sub"]}
	for k, v := range p.userInfoOnly {
		out[k] = v
	}
	writeJSON(w, out)
}

func (p *Provider) sign(claims map[string]any) (string, error) {
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.RS256, Key: p.key},
		(&jose.SignerOptions{}).WithType("JWT").WithHeader("kid", "k1"))
	if err != nil {
		return "", err
	}
	payload, err := json.Marshal(claims)
	if err != nil {
		return "", err
	}
	object, err := signer.Sign(payload)
	if err != nil {
		return "", err
	}
	return object.CompactSerialize()
}

func tokenError(w http.ResponseWriter, code string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_ = json.NewEncoder(w).Encode(map[string]string{"error": code})
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func randomString() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return base64.RawURLEncoding.EncodeToString(b)
}
