package api

import (
	"runtime"
	"strconv"
	"strings"
	"testing"
	"time"
)

// Failed logins may name any username up to the 1 MiB body limit; the
// limiter's table must stay small whatever their length.
func TestLimiterDoesNotKeepUsernames(t *testing.T) {
	l := newLoginLimiter(time.Now)
	var before, after runtime.MemStats
	runtime.GC()
	runtime.ReadMemStats(&before)
	for i := range 100 {
		l.fail(newLimiterKey("192.0.2.1", strconv.Itoa(i)+strings.Repeat("x", 1<<20)))
	}
	runtime.GC()
	runtime.ReadMemStats(&after)
	if len(l.failures) != 100 {
		t.Fatalf("entries = %d, want 100", len(l.failures))
	}
	if grown := int64(after.HeapAlloc) - int64(before.HeapAlloc); grown > 8<<20 {
		t.Fatalf("100 failed logins with 1 MiB usernames keep %d MiB", grown>>20)
	}

	// The digest still tells usernames apart.
	long := strings.Repeat("y", 1<<20)
	for range loginMaxFailures {
		l.fail(newLimiterKey("192.0.2.1", long))
	}
	if l.blocked(newLimiterKey("192.0.2.1", long)) == 0 {
		t.Fatal("a long username must be throttled like any other")
	}
	if l.blocked(newLimiterKey("192.0.2.1", long+"z")) != 0 || l.blocked(newLimiterKey("192.0.2.2", long)) != 0 {
		t.Fatal("other usernames and other clients must not be throttled")
	}
}
