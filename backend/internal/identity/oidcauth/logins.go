package oidcauth

import (
	"crypto/rand"
	"encoding/base64"
	"sync"
	"time"
)

// Pending sign-ins expire after loginTTL; at most maxLogins are kept, so a
// flood of started sign-ins cannot grow memory without bound.
const (
	loginTTL  = 10 * time.Minute
	maxLogins = 10000
)

// Login is a sign-in between its start and the provider's callback.
type Login struct {
	State       string // returned by the provider; also kept in a browser cookie
	Nonce       string // must come back in the ID token
	Verifier    string // PKCE code verifier
	RedirectURL string // the callback URL the code was issued for
	created     time.Time
}

// Logins keeps pending sign-ins in memory, keyed by state. Each can be
// taken once.
type Logins struct {
	mu      sync.Mutex
	now     func() time.Time
	pending map[string]*Login
}

// NewLogins returns an empty store; now is the clock.
func NewLogins(now func() time.Time) *Logins {
	return &Logins{now: now, pending: map[string]*Login{}}
}

// Put keeps a pending sign-in. When the store is full, expired entries go
// first, then the oldest.
func (l *Logins) Put(login *Login) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	login.created = now
	if len(l.pending) >= maxLogins {
		var oldest *Login
		for state, p := range l.pending {
			if now.Sub(p.created) >= loginTTL {
				delete(l.pending, state)
			} else if oldest == nil || p.created.Before(oldest.created) {
				oldest = p
			}
		}
		if len(l.pending) >= maxLogins && oldest != nil {
			delete(l.pending, oldest.State)
		}
	}
	l.pending[login.State] = login
}

// Take removes and returns the pending sign-in of state, unless unknown
// or expired.
func (l *Logins) Take(state string) (*Login, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	login, ok := l.pending[state]
	if !ok {
		return nil, false
	}
	delete(l.pending, state)
	return login, l.now().Sub(login.created) < loginTTL
}

// randomToken is 256 random bits, URL-safe.
func randomToken() string {
	b := make([]byte, 32)
	_, _ = rand.Read(b) // never fails (crypto/rand panics instead)
	return base64.RawURLEncoding.EncodeToString(b)
}
