// Package oidcauth signs users in with an OpenID Connect provider
// (docs/design-v0.5.md §3.6): the authorization code flow with PKCE, state
// and nonce, discovery and JWKS verification of the ID token.
package oidcauth

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"

	"kubebench/internal/config"
	"kubebench/internal/identity"
	"kubebench/internal/store"
)

// requestTimeout bounds each call to the provider.
const requestTimeout = 15 * time.Second

// Provider is the configured OpenID Connect provider. Discovery happens on
// first use and is retried until it succeeds, so KubeBench starts while
// the provider is down.
type Provider struct {
	cfg    config.OIDCConfig
	client *http.Client

	mu         sync.Mutex
	discovered *oidc.Provider
	endSession string // RP-initiated logout endpoint; "" when unsupported
}

// New returns the provider for cfg. Its HTTP client verifies TLS against
// the system roots plus cfg.CAFile.
func New(cfg config.OIDCConfig) (*Provider, error) {
	transport := http.DefaultTransport.(*http.Transport).Clone()
	if cfg.CAFile != "" {
		pool, err := x509.SystemCertPool()
		if err != nil {
			pool = x509.NewCertPool()
		}
		pem, err := os.ReadFile(cfg.CAFile)
		if err != nil {
			return nil, fmt.Errorf("read the OIDC CA bundle: %w", err)
		}
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("the OIDC CA bundle %s holds no PEM certificate", cfg.CAFile)
		}
		transport.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	return &Provider{cfg: cfg, client: &http.Client{Transport: transport, Timeout: requestTimeout}}, nil
}

func (p *Provider) Source() string { return store.UserSourceOIDC }
func (p *Provider) Name() string   { return p.cfg.Name }

// Config is the provider's configuration (it holds the client secret).
func (p *Provider) Config() config.OIDCConfig { return p.cfg }

// discover fetches the discovery document once it is reachable.
func (p *Provider) discover(ctx context.Context) (*oidc.Provider, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.discovered != nil {
		return p.discovered, nil
	}
	ctx, cancel := context.WithTimeout(oidc.ClientContext(ctx, p.client), requestTimeout)
	defer cancel()
	discovered, err := oidc.NewProvider(ctx, p.cfg.Issuer)
	if err != nil {
		return nil, fmt.Errorf("discover %s: %w", p.cfg.Issuer, err)
	}
	var extra struct {
		EndSession string `json:"end_session_endpoint"`
	}
	if err := discovered.Claims(&extra); err != nil {
		return nil, fmt.Errorf("discover %s: %w", p.cfg.Issuer, err)
	}
	p.discovered, p.endSession = discovered, extra.EndSession
	return discovered, nil
}

// Check reports whether the provider's discovery document can be read.
func (p *Provider) Check(ctx context.Context) error {
	_, err := p.discover(ctx)
	return err
}

// SupportsLogout reports whether the provider announced an end-session
// endpoint (after discovery succeeded).
func (p *Provider) SupportsLogout() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.endSession != ""
}

func (p *Provider) oauth2Config(discovered *oidc.Provider, redirectURL string) *oauth2.Config {
	return &oauth2.Config{
		ClientID:     p.cfg.ClientID,
		ClientSecret: p.cfg.ClientSecret,
		Endpoint:     discovered.Endpoint(),
		RedirectURL:  redirectURL,
		Scopes:       p.cfg.Scopes,
	}
}

// Start begins a sign-in: it returns the pending login to keep until the
// callback and the provider URL to send the browser to.
func (p *Provider) Start(ctx context.Context, redirectURL string) (*Login, string, error) {
	discovered, err := p.discover(ctx)
	if err != nil {
		return nil, "", identity.Fail(identity.ClassProviderError, "%v", err)
	}
	login := &Login{
		State:       randomToken(),
		Nonce:       randomToken(),
		Verifier:    oauth2.GenerateVerifier(),
		RedirectURL: redirectURL,
	}
	authURL := p.oauth2Config(discovered, redirectURL).AuthCodeURL(login.State,
		oidc.Nonce(login.Nonce), oauth2.S256ChallengeOption(login.Verifier))
	return login, authURL, nil
}

// ErrNotRedeemed marks a Finish failure that happened before the provider
// redeemed the authorization code (the provider unreachable, the code
// refused). Anyone can cause one with a made-up code, so it proves nothing
// about who is signing in.
var ErrNotRedeemed = errors.New("the authorization code was not redeemed")

// Finish completes a sign-in with the authorization code of the callback:
// it redeems the code with the PKCE verifier, verifies the ID token
// (signature, issuer, audience, expiry, nonce) and reads the identity from
// its claims, completed from the userinfo endpoint when the token lacks
// the groups claim. It returns the raw ID token for RP-initiated logout.
func (p *Provider) Finish(ctx context.Context, login *Login, code string) (*identity.Identity, string, error) {
	discovered, err := p.discover(ctx)
	if err != nil {
		return nil, "", identity.Fail(identity.ClassProviderError, "%w: %v", ErrNotRedeemed, err)
	}
	ctx, cancel := context.WithTimeout(oidc.ClientContext(ctx, p.client), requestTimeout)
	defer cancel()
	token, err := p.oauth2Config(discovered, login.RedirectURL).Exchange(ctx, code, oauth2.VerifierOption(login.Verifier))
	if err != nil {
		if _, ok := errors.AsType[*oauth2.RetrieveError](err); ok {
			return nil, "", identity.Fail(identity.ClassInvalidRequest, "%w: %v", ErrNotRedeemed, err)
		}
		return nil, "", identity.Fail(identity.ClassProviderError, "%w: %v", ErrNotRedeemed, err)
	}
	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, "", identity.Fail(identity.ClassProviderError, "the token response has no ID token")
	}
	idToken, err := discovered.Verifier(&oidc.Config{ClientID: p.cfg.ClientID}).Verify(ctx, rawIDToken)
	if err != nil {
		return nil, "", identity.Fail(identity.ClassInvalidRequest, "verify the ID token: %v", err)
	}
	if idToken.Nonce != login.Nonce {
		return nil, "", identity.Fail(identity.ClassInvalidRequest, "the ID token's nonce does not match the sign-in")
	}
	claims := map[string]any{}
	if err := idToken.Claims(&claims); err != nil {
		return nil, "", identity.Fail(identity.ClassProviderError, "read the ID token claims: %v", err)
	}
	if _, ok := lookupClaim(claims, p.cfg.GroupsClaim); !ok && discovered.UserInfoEndpoint() != "" {
		if err := p.mergeUserInfo(ctx, discovered, token, idToken.Subject, claims); err != nil {
			return nil, "", identity.Fail(identity.ClassProviderError, "%v", err)
		}
	}
	id, err := identityFromClaims(p.cfg, idToken.Subject, claims)
	if err != nil {
		return nil, "", err
	}
	return id, rawIDToken, nil
}

// mergeUserInfo adds the userinfo claims the ID token does not carry.
func (p *Provider) mergeUserInfo(ctx context.Context, discovered *oidc.Provider, token *oauth2.Token, subject string, claims map[string]any) error {
	info, err := discovered.UserInfo(ctx, oauth2.StaticTokenSource(token))
	if err != nil {
		return fmt.Errorf("read userinfo: %w", err)
	}
	if info.Subject != subject {
		return errors.New("userinfo belongs to another subject than the ID token")
	}
	extra := map[string]any{}
	if err := info.Claims(&extra); err != nil {
		return fmt.Errorf("read userinfo claims: %w", err)
	}
	for k, v := range extra {
		if _, ok := claims[k]; !ok {
			claims[k] = v
		}
	}
	return nil
}

// LogoutURL is the provider URL that ends its session and returns the
// browser to postLogoutURL; "" when the provider does not support
// RP-initiated logout.
func (p *Provider) LogoutURL(ctx context.Context, idToken, postLogoutURL string) string {
	if _, err := p.discover(ctx); err != nil {
		return ""
	}
	p.mu.Lock()
	endpoint := p.endSession
	p.mu.Unlock()
	u, err := url.Parse(endpoint)
	if endpoint == "" || err != nil {
		return ""
	}
	q := u.Query()
	q.Set("client_id", p.cfg.ClientID)
	q.Set("post_logout_redirect_uri", postLogoutURL)
	if idToken != "" {
		q.Set("id_token_hint", idToken)
	}
	u.RawQuery = q.Encode()
	return u.String()
}

// identityFromClaims reads the identity from verified claims and applies
// the allowed email domains.
func identityFromClaims(cfg config.OIDCConfig, subject string, claims map[string]any) (*identity.Identity, error) {
	if subject == "" {
		return nil, identity.Fail(identity.ClassProviderError, "the ID token has no subject")
	}
	id := &identity.Identity{
		Source:      store.UserSourceOIDC,
		Subject:     subject,
		DisplayName: stringClaim(claims, "name"),
		Email:       stringClaim(claims, "email"),
		Groups:      groupsClaim(claims, cfg.GroupsClaim),
	}
	id.Username = stringClaim(claims, cfg.UsernameClaim)
	if id.Username == "" {
		return nil, identity.Fail(identity.ClassInvalidUsername, "the token has no %q claim", cfg.UsernameClaim)
	}
	if len(cfg.AllowedDomains) > 0 {
		if err := checkDomain(cfg.AllowedDomains, id.Email, claims); err != nil {
			err.Subject = subject
			return nil, err
		}
	}
	return id, nil
}

// checkDomain admits only verified email addresses of the allowed domains.
func checkDomain(allowed []string, email string, claims map[string]any) *identity.Failure {
	if !emailVerified(claims) {
		return identity.Fail(identity.ClassDomainNotAllowed, "the email address %q is not verified", email)
	}
	_, domain, ok := strings.Cut(email, "@")
	if !ok {
		return identity.Fail(identity.ClassDomainNotAllowed, "the token has no email address")
	}
	for _, d := range allowed {
		if strings.EqualFold(domain, d) {
			return nil
		}
	}
	return identity.Fail(identity.ClassDomainNotAllowed, "the email domain %q is not allowed", domain)
}

// emailVerified reports whether the provider asserts that the email address
// is verified: email_verified is the boolean true or, as some providers
// send it, the string "true". A missing claim (Entra ID never sends one)
// or any other value is not a verification.
func emailVerified(claims map[string]any) bool {
	switch v := claims["email_verified"].(type) {
	case bool:
		return v
	case string:
		return strings.EqualFold(strings.TrimSpace(v), "true")
	default:
		return false
	}
}

// lookupClaim finds a claim by name or, failing that, by dotted path
// (realm_access.roles).
func lookupClaim(claims map[string]any, name string) (any, bool) {
	if v, ok := claims[name]; ok {
		return v, true
	}
	var cur any = claims
	for part := range strings.SplitSeq(name, ".") {
		m, ok := cur.(map[string]any)
		if !ok {
			return nil, false
		}
		if cur, ok = m[part]; !ok {
			return nil, false
		}
	}
	return cur, true
}

func stringClaim(claims map[string]any, name string) string {
	v, _ := lookupClaim(claims, name)
	s, _ := v.(string)
	return strings.TrimSpace(s)
}

// groupsClaim reads a list of strings, or a single string, claim.
func groupsClaim(claims map[string]any, name string) []string {
	v, _ := lookupClaim(claims, name)
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		groups := make([]string, 0, len(v))
		for _, item := range v {
			if s, ok := item.(string); ok && s != "" {
				groups = append(groups, s)
			}
		}
		return groups
	}
	return []string{}
}
