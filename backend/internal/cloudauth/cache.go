package cloudauth

import (
	"sync"
	"time"

	"golang.org/x/oauth2"
)

// refreshMargin is how long before its expiry a cached token is replaced,
// so a request never leaves with a token that expires on the way.
const refreshMargin = time.Minute

// cachedSource caches a token until refreshMargin before it expires. It
// implements client-go's ResettableTokenSource: a 401 from the API server
// drops the token that request used, so the next request fetches a new
// one (e.g. after the cloud revoked a session).
type cachedSource struct {
	base oauth2.TokenSource
	now  func() time.Time // judges expiry

	mu        sync.Mutex
	token     *oauth2.Token
	fetchedAt time.Time
}

func newCachedSource(base oauth2.TokenSource, now func() time.Time) *cachedSource {
	return &cachedSource{base: base, now: now}
}

func (c *cachedSource) Token() (*oauth2.Token, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now()
	if c.token != nil && (c.token.Expiry.IsZero() || now.Before(c.token.Expiry.Add(-refreshMargin))) {
		return c.token, nil
	}
	token, err := c.base.Token()
	if err != nil {
		return nil, err
	}
	// The wall clock, not c.now: client-go compares it with its own
	// time.Now() in ResetTokenOlderThan.
	c.token, c.fetchedAt = token, time.Now()
	return token, nil
}

// ResetTokenOlderThan drops the cached token when it was fetched before t.
func (c *cachedSource) ResetTokenOlderThan(t time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.token != nil && c.fetchedAt.Before(t) {
		c.token = nil
	}
}
