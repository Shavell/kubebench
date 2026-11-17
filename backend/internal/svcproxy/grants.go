package svcproxy

import (
	"crypto/rand"
	"encoding/base64"
	"slices"
	"sync"
	"time"
)

// Grant limits (§7.14). A grant ends after IdleTimeout without requests,
// MaxLifetime after it was issued, or with the session that opened it.
const (
	IdleTimeout = time.Hour
	MaxLifetime = 12 * time.Hour
	// PerSession is how many grants one sign-in session keeps; opening
	// one more ends the least recently used.
	PerSession = 32
	// MaxGrants bounds the grants of all sessions together.
	MaxGrants = 4096
	// MaxNavigations is how many frame navigations a grant remembers.
	MaxNavigations = 16
	// MaxLocation bounds a remembered location; a longer one is recorded
	// as unknown ("").
	MaxLocation = 4 << 10
)

// Grant is one browsing session: the right to reach Target through the
// proxy, as the user of Session, on cluster ClusterID. Its Token is the
// only credential a proxied request carries. Whoever holds the token can
// browse the target until the grant ends, so it is shown only to the
// browser that opened it, and every request checks again that the
// session is alive and its user still holds the access (see the api
// package).
type Grant struct {
	Token     string
	Session   string // the KubeBench session token that opened the grant
	UserID    int64
	ClusterID int64
	Target    Target
	// FrameAncestor is the KubeBench origin (scheme://host) that opened
	// the grant, the only page allowed to frame the proxied content.
	FrameAncestor string

	issued, used time.Time
	navigations  []Navigation // oldest first
	lastSeq      int64
}

// Navigation is one document load of the browser frame: the request of a
// frame's document, numbered in the order the proxy got them. A page with
// nested frames yields one navigation per frame, its own first.
type Navigation struct {
	Seq int64 `json:"seq"`
	// Location is the in-cluster path with its query ("" when longer
	// than MaxLocation).
	Location string `json:"location"`
}

// Navigations are the latest MaxNavigations navigations, oldest first
// (empty, not nil, before the first).
func (g Grant) Navigations() []Navigation { return append([]Navigation{}, g.navigations...) }

// Grants keeps the grants in memory: they end with the process, and a
// reopened browser tab asks for a new one.
type Grants struct {
	mu      sync.Mutex
	byToken map[string]*Grant
	now     func() time.Time
}

// NewGrants returns an empty set; now is the clock (time.Now in production).
func NewGrants(now func() time.Time) *Grants {
	return &Grants{byToken: map[string]*Grant{}, now: now}
}

// Issue stores g with a fresh token and returns it. Expired grants are
// dropped first; beyond PerSession grants of g's session, or MaxGrants in
// all, the least recently used ones end.
func (gs *Grants) Issue(g Grant) (Grant, error) {
	token, err := newToken()
	if err != nil {
		return Grant{}, err
	}
	gs.mu.Lock()
	defer gs.mu.Unlock()
	now := gs.now()
	gs.pruneLocked(now)
	gs.evictLocked(func(o *Grant) bool { return o.Session == g.Session }, PerSession-1)
	gs.evictLocked(func(*Grant) bool { return true }, MaxGrants-1)
	g.Token, g.issued, g.used = token, now, now
	g.navigations, g.lastSeq = nil, 0
	stored := g
	gs.byToken[token] = &stored
	return stored, nil
}

// Use looks a token up for a proxied request and marks the grant used.
func (gs *Grants) Use(token string) (Grant, bool) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	g, ok := gs.liveLocked(token)
	if !ok {
		return Grant{}, false
	}
	g.used = gs.now()
	return *g, true
}

// Get returns the grant of token when session opened it.
func (gs *Grants) Get(token, session string) (Grant, bool) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	g, ok := gs.liveLocked(token)
	if !ok || g.Session != session {
		return Grant{}, false
	}
	return *g, true
}

// RecordNavigation records that a frame loaded the document at location.
func (gs *Grants) RecordNavigation(token, location string) {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	g, ok := gs.byToken[token]
	if !ok {
		return
	}
	if len(location) > MaxLocation {
		location = ""
	}
	g.lastSeq++
	g.navigations = append(g.navigations, Navigation{Seq: g.lastSeq, Location: location})
	if over := len(g.navigations) - MaxNavigations; over > 0 {
		g.navigations = slices.Delete(g.navigations, 0, over)
	}
}

// Revoke ends the grant of token when session opened it.
func (gs *Grants) Revoke(token, session string) bool {
	gs.mu.Lock()
	defer gs.mu.Unlock()
	g, ok := gs.byToken[token]
	if !ok || g.Session != session {
		return false
	}
	delete(gs.byToken, token)
	return true
}

// liveLocked returns the unexpired grant of token, dropping it if expired.
func (gs *Grants) liveLocked(token string) (*Grant, bool) {
	g, ok := gs.byToken[token]
	if !ok {
		return nil, false
	}
	if expired(g, gs.now()) {
		delete(gs.byToken, token)
		return nil, false
	}
	return g, true
}

func expired(g *Grant, now time.Time) bool {
	return !now.Before(g.used.Add(IdleTimeout)) || !now.Before(g.issued.Add(MaxLifetime))
}

func (gs *Grants) pruneLocked(now time.Time) {
	for token, g := range gs.byToken {
		if expired(g, now) {
			delete(gs.byToken, token)
		}
	}
}

// evictLocked ends the least recently used grants matching match until at
// most keep of them are left.
func (gs *Grants) evictLocked(match func(*Grant) bool, keep int) {
	for {
		var oldest *Grant
		count := 0
		for _, g := range gs.byToken {
			if !match(g) {
				continue
			}
			count++
			if oldest == nil || g.used.Before(oldest.used) {
				oldest = g
			}
		}
		if count <= keep {
			return
		}
		delete(gs.byToken, oldest.Token)
	}
}

// newToken is 256 random bits, URL-safe.
func newToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}
