package api

import (
	"crypto/sha256"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

// Login throttling (§3.5): 10 failed attempts per (client IP, username)
// within 15 minutes block further attempts until the window ends.
const (
	loginMaxFailures = 10
	loginWindow      = 15 * time.Minute
	// loginMaxKeys bounds the memory a flood of distinct usernames can use.
	loginMaxKeys = 10000
)

// limiterKey identifies a (client IP, username) pair. The username is kept
// as a digest: login attempts name arbitrary usernames of up to the 1 MiB
// body limit, and the table must not keep them (loginMaxKeys bounds the
// number of keys, so their size has to be fixed).
type limiterKey struct {
	ip       string // parsed from RemoteAddr or X-Forwarded-For, so short
	username [sha256.Size]byte
}

func newLimiterKey(ip, username string) limiterKey {
	return limiterKey{ip: ip, username: sha256.Sum256([]byte(username))}
}

type failureWindow struct {
	count int
	start time.Time
}

type loginLimiter struct {
	mu       sync.Mutex
	now      func() time.Time
	failures map[limiterKey]*failureWindow
}

func newLoginLimiter(now func() time.Time) *loginLimiter {
	return &loginLimiter{now: now, failures: map[limiterKey]*failureWindow{}}
}

// blocked returns how long key must still wait, or 0.
func (l *loginLimiter) blocked(key limiterKey) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	f, ok := l.failures[key]
	if !ok || f.count < loginMaxFailures {
		return 0
	}
	return max(0, f.start.Add(loginWindow).Sub(l.now()))
}

// fail records a failed attempt.
func (l *loginLimiter) fail(key limiterKey) {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	f, ok := l.failures[key]
	if !ok || now.Sub(f.start) >= loginWindow {
		if !ok && len(l.failures) >= loginMaxKeys {
			l.evict(now)
		}
		l.failures[key] = &failureWindow{count: 1, start: now}
		return
	}
	f.count++
}

// reset forgets key's failures after a successful attempt.
func (l *loginLimiter) reset(key limiterKey) {
	l.mu.Lock()
	defer l.mu.Unlock()
	delete(l.failures, key)
}

// evict drops expired windows and, if the table is still full, arbitrary
// ones: under a flood the limiter degrades rather than growing unbounded.
func (l *loginLimiter) evict(now time.Time) {
	for key, f := range l.failures {
		if now.Sub(f.start) >= loginWindow {
			delete(l.failures, key)
		}
	}
	for key := range l.failures {
		if len(l.failures) < loginMaxKeys {
			break
		}
		delete(l.failures, key)
	}
}

// clientIP is the caller's address. Behind a trusted proxy it is the last
// X-Forwarded-For entry: the address that proxy saw, which a client cannot
// forge by sending its own header.
func (s *Server) clientIP(r *http.Request) string {
	if s.cfg.TrustProxy {
		if values := r.Header.Values("X-Forwarded-For"); len(values) > 0 {
			entries := strings.Split(values[len(values)-1], ",")
			if ip := strings.TrimSpace(entries[len(entries)-1]); net.ParseIP(ip) != nil {
				return ip
			}
		}
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return r.RemoteAddr
	}
	return host
}
