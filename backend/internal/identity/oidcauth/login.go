package oidcauth

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"time"
)

// LoginTTL is how long a started sign-in can be completed.
const LoginTTL = 10 * time.Minute

// Login is a sign-in between its start and the provider's callback.
type Login struct {
	State       string    `json:"s"` // returned by the provider in the callback
	Nonce       string    `json:"n"` // must come back in the ID token
	Verifier    string    `json:"v"` // PKCE code verifier
	RedirectURL string    `json:"r"` // the callback URL the code was issued for
	Expires     time.Time `json:"e"`
}

// sealAAD binds sealed values to their use.
var sealAAD = []byte("kubebench-oidc-login")

// Sealer keeps pending sign-ins in the browser instead of on the server:
// it encrypts and authenticates a Login (AES-256-GCM, a key drawn at
// startup) into a cookie value only this process can open. The server
// keeps nothing per started sign-in, so starting sign-ins in a flood
// cannot push out other users' sign-ins in progress.
type Sealer struct {
	aead cipher.AEAD
	now  func() time.Time
}

// NewSealer returns a sealer with a fresh random key; now is the clock.
// Sign-ins started before a restart cannot be completed after it.
func NewSealer(now func() time.Time) *Sealer {
	key := make([]byte, 32)
	_, _ = rand.Read(key) // never fails (crypto/rand panics instead)
	block, err := aes.NewCipher(key)
	if err != nil {
		panic(err) // a 32-byte key is always valid
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		panic(err)
	}
	return &Sealer{aead: aead, now: now}
}

// Seal sets the login's expiry and returns it as an opaque, URL-safe
// value.
func (s *Sealer) Seal(login *Login) string {
	login.Expires = s.now().Add(LoginTTL)
	plain, _ := json.Marshal(login) // strings and a time: cannot fail
	nonce := make([]byte, s.aead.NonceSize())
	_, _ = rand.Read(nonce)
	return base64.RawURLEncoding.EncodeToString(s.aead.Seal(nonce, nonce, plain, sealAAD))
}

// ErrInvalidLogin is a sealed value that is malformed, was not sealed by
// this process or has expired.
var ErrInvalidLogin = errors.New("the sign-in is unknown or expired")

// Open returns the login sealed in value, unless it was tampered with or
// has expired.
func (s *Sealer) Open(value string) (*Login, error) {
	sealed, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil || len(sealed) < s.aead.NonceSize() {
		return nil, ErrInvalidLogin
	}
	nonce, box := sealed[:s.aead.NonceSize()], sealed[s.aead.NonceSize():]
	plain, err := s.aead.Open(nil, nonce, box, sealAAD)
	if err != nil {
		return nil, ErrInvalidLogin
	}
	var login Login
	if err := json.Unmarshal(plain, &login); err != nil || !s.now().Before(login.Expires) {
		return nil, ErrInvalidLogin
	}
	return &login, nil
}

// randomToken is 256 random bits, URL-safe.
func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // never fails (crypto/rand panics instead)
	return base64.RawURLEncoding.EncodeToString(b)
}
