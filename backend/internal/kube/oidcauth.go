package kube

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"golang.org/x/oauth2"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	"k8s.io/client-go/transport"

	"kubebench/internal/reason"
)

// The legacy "oidc" auth provider of kubeconfigs (users[].user.
// auth-provider), as kubectl's built-in plugin reads it. KubeBench
// handles it in process instead of registering client-go's plugin, which
// keeps one provider per (API server, issuer, client ID) for the whole
// process: two clusters with the same server and issuer but different
// users would share one identity, and new tokens pasted by an admin would
// be ignored until a restart.
const (
	oidcProviderName  = "oidc"
	oidcIssuerURL     = "idp-issuer-url"
	oidcClientID      = "client-id"
	oidcClientSecret  = "client-secret"
	oidcIDToken       = "id-token"
	oidcRefreshToken  = "refresh-token"
	oidcCAFile        = "idp-certificate-authority"
	oidcCAData        = "idp-certificate-authority-data"
	oidcExpiryMargin  = 10 * time.Second // client-go's expiryDelta
	oidcIssuerTimeout = 30 * time.Second
	maxDiscoveryBytes = 1 << 20
)

// OIDCCAFileKey is the auth provider setting that names a CA file on the
// server; pasted kubeconfigs must embed the CA (idp-certificate-authority-data).
const OIDCCAFileKey = oidcCAFile

// IsOIDCAuthProvider reports whether user signs in with the legacy oidc
// auth provider.
func IsOIDCAuthProvider(user *clientcmdapi.AuthInfo) bool {
	return user != nil && user.AuthProvider != nil && user.AuthProvider.Name == oidcProviderName
}

// CheckAuthProvider refuses an auth provider KubeBench cannot use: any
// but oidc (client-go dropped gcp and azure), or an oidc provider without
// an issuer, client ID or token. nil when user has none.
func CheckAuthProvider(user *clientcmdapi.AuthInfo) error {
	if user == nil || user.AuthProvider == nil {
		return nil
	}
	if !IsOIDCAuthProvider(user) {
		return reason.New(reasonAuthProviderUnsupported, reason.Params{"name": user.AuthProvider.Name})
	}
	cfg := user.AuthProvider.Config
	for _, key := range []string{oidcIssuerURL, oidcClientID} {
		if strings.TrimSpace(cfg[key]) == "" {
			return reason.New(reasonOIDCIncomplete, reason.Params{"field": key})
		}
	}
	if cfg[oidcIDToken] == "" && cfg[oidcRefreshToken] == "" {
		return reason.New(reasonOIDCIncomplete, reason.Params{"field": oidcIDToken + " / " + oidcRefreshToken})
	}
	if data := cfg[oidcCAData]; data != "" {
		if _, err := base64.StdEncoding.DecodeString(data); err != nil {
			return reason.New(reasonOIDCIncomplete, reason.Params{"field": oidcCAData + " (base64)"})
		}
	}
	return nil
}

// oidcSources keeps the token state of each cluster's oidc provider across
// runtime rebuilds (a rename, a changed flag): an identity provider that
// rotates refresh tokens has spent the stored one after the first
// refresh. The key includes a digest of the stored settings, so a changed
// kubeconfig starts over with its own tokens.
var oidcSources = struct {
	sync.Mutex
	byCluster map[int64]*oidcSource
}{byCluster: map[int64]*oidcSource{}}

// oidcAuth replaces user's oidc auth provider with an in-process token
// source and returns the transport wrapper that adds its ID token, or nil
// when user has none. clusterID keys the token state (see oidcSources).
func oidcAuth(user *clientcmdapi.AuthInfo, clusterID int64, clusterName string) (func(http.RoundTripper) http.RoundTripper, error) {
	if user == nil || user.AuthProvider == nil {
		return nil, nil
	}
	if err := CheckAuthProvider(user); err != nil {
		return nil, err
	}
	cfg := user.AuthProvider.Config
	digest := oidcDigest(cfg)

	oidcSources.Lock()
	src := oidcSources.byCluster[clusterID]
	if src == nil || src.digest != digest {
		client, err := oidcHTTPClient(cfg)
		if err != nil {
			oidcSources.Unlock()
			return nil, err
		}
		src = &oidcSource{
			digest: digest, cluster: clusterName, client: client, now: time.Now,
			issuer: cfg[oidcIssuerURL], clientID: cfg[oidcClientID], clientSecret: cfg[oidcClientSecret],
			idToken: cfg[oidcIDToken], refreshToken: cfg[oidcRefreshToken],
		}
		oidcSources.byCluster[clusterID] = src
	}
	oidcSources.Unlock()

	user.AuthProvider = nil
	return transport.TokenSourceWrapTransport(src), nil
}

func oidcDigest(cfg map[string]string) [sha256.Size]byte {
	b, _ := json.Marshal(cfg) // map keys are sorted
	return sha256.Sum256(b)
}

// oidcHTTPClient talks to the issuer: its CA when the kubeconfig names one
// (embedded, or a file for kubeconfigs that may read the server's files),
// a time limit, and no redirects, so a refresh token is never sent on to
// another host.
func oidcHTTPClient(cfg map[string]string) (*http.Client, error) {
	tr := http.DefaultTransport.(*http.Transport).Clone()
	var pem []byte
	switch {
	case cfg[oidcCAData] != "":
		data, err := base64.StdEncoding.DecodeString(cfg[oidcCAData])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", oidcCAData, err)
		}
		pem = data
	case cfg[oidcCAFile] != "":
		data, err := os.ReadFile(cfg[oidcCAFile])
		if err != nil {
			return nil, fmt.Errorf("%s: %w", oidcCAFile, err)
		}
		pem = data
	}
	if pem != nil {
		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(pem) {
			return nil, fmt.Errorf("oidc auth provider: no certificate in the identity provider's CA")
		}
		tr.TLSClientConfig = &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}
	}
	return &http.Client{
		Transport:     tr,
		Timeout:       oidcIssuerTimeout,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}, nil
}

// oidcSource hands out the provider's ID token and refreshes it with the
// refresh token at the issuer's token endpoint when it expires. Refreshed
// tokens live in memory only (and in oidcSources across rebuilds of the
// same cluster): they are never written to a file or back to the stored
// kubeconfig.
type oidcSource struct {
	digest  [sha256.Size]byte
	cluster string
	client  *http.Client
	now     func() time.Time

	issuer, clientID, clientSecret string

	mu           sync.Mutex
	idToken      string
	refreshToken string
	rotated      bool // the issuer replaced the stored refresh token
}

func (s *oidcSource) Token() (*oauth2.Token, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	exp, hasExp := jwtExpiry(s.idToken)
	if s.idToken != "" && hasExp && s.now().Add(oidcExpiryMargin).Before(exp) {
		return &oauth2.Token{AccessToken: s.idToken, TokenType: "Bearer", Expiry: exp}, nil
	}
	if s.refreshToken == "" {
		if s.idToken != "" && !hasExp {
			// No expiry to judge by: let the API server decide.
			return &oauth2.Token{AccessToken: s.idToken, TokenType: "Bearer"}, nil
		}
		return nil, errors.New("oidc auth provider: the id-token has expired and the kubeconfig has no refresh-token")
	}
	if err := s.refresh(); err != nil {
		return nil, err
	}
	exp, _ = jwtExpiry(s.idToken)
	return &oauth2.Token{AccessToken: s.idToken, TokenType: "Bearer", Expiry: exp}, nil
}

// refresh trades the refresh token for a new ID token; s.mu is held.
func (s *oidcSource) refresh() error {
	ctx, cancel := context.WithTimeout(context.Background(), oidcIssuerTimeout)
	defer cancel()
	tokenURL, err := s.tokenEndpoint(ctx)
	if err != nil {
		return fmt.Errorf("oidc auth provider: discovery at %s: %w", s.issuer, err)
	}
	conf := oauth2.Config{ClientID: s.clientID, ClientSecret: s.clientSecret, Endpoint: oauth2.Endpoint{TokenURL: tokenURL}}
	token, err := conf.TokenSource(context.WithValue(ctx, oauth2.HTTPClient, s.client),
		&oauth2.Token{RefreshToken: s.refreshToken}).Token()
	if err != nil {
		return fmt.Errorf("oidc auth provider: cannot refresh the token: %w", err)
	}
	idToken, ok := token.Extra("id_token").(string)
	if !ok || idToken == "" {
		return errors.New("oidc auth provider: the token response has no id_token (was the openid scope requested at sign-in?)")
	}
	s.idToken = idToken
	if token.RefreshToken != "" && token.RefreshToken != s.refreshToken {
		s.refreshToken = token.RefreshToken
		if !s.rotated {
			s.rotated = true
			log.Printf("[oidc] cluster=%s: the identity provider rotated the refresh token; the new one is kept in memory only, so after a restart the stored kubeconfig may need fresh tokens", s.cluster)
		}
	}
	return nil
}

func (s *oidcSource) tokenEndpoint(ctx context.Context) (string, error) {
	wellKnown := strings.TrimSuffix(s.issuer, "/") + "/.well-known/openid-configuration"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, wellKnown, nil)
	if err != nil {
		return "", err
	}
	res, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("%s answered %s", wellKnown, res.Status)
	}
	var doc struct {
		TokenEndpoint string `json:"token_endpoint"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, maxDiscoveryBytes)).Decode(&doc); err != nil {
		return "", fmt.Errorf("%s: %w", wellKnown, err)
	}
	if doc.TokenEndpoint == "" {
		return "", fmt.Errorf("%s names no token_endpoint", wellKnown)
	}
	return doc.TokenEndpoint, nil
}

// jwtExpiry reads the exp claim of a JWT without verifying it (the API
// server does); false when the token has none or is not a JWT.
func jwtExpiry(token string) (time.Time, bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return time.Time{}, false
	}
	payload, err := base64.RawURLEncoding.DecodeString(strings.TrimRight(parts[1], "="))
	if err != nil {
		return time.Time{}, false
	}
	var claims struct {
		Exp *json.Number `json:"exp"`
	}
	if err := json.Unmarshal(payload, &claims); err != nil || claims.Exp == nil {
		return time.Time{}, false
	}
	f, err := claims.Exp.Float64()
	if err != nil {
		return time.Time{}, false
	}
	return time.Unix(int64(f), 0), true
}
