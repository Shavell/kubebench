package api

import (
	"errors"
	"net/http"
	"sync"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
)

// Long-lived requests (terminals, log and watch streams) are authorized by the
// router when they start, like every request, but may run for hours. They
// check again while they run (stillAllowed) and are counted per user
// (streamLimiter), so that neither a revoked user nor a single client can
// keep them going without bound.

// Concurrent long-lived requests one user may have open.
const (
	maxLogStreamsPerUser = 20
	maxTerminalsPerUser  = 10
	// Each browser tab holds one watch stream, and briefly two while it
	// changes its topics.
	maxWatchStreamsPerUser = 16
)

// stillAllowed repeats the checks authorize made for c at level (read or
// exec) against the current state: the session still exists (no logout,
// password change or user deletion since), no password change is due,
// the cluster still exists, the caller still has a role on it and, for
// exec, that role and the cluster's read-only flag still allow changes.
// It checks c.cluster rather than the caller's active cluster: switching
// clusters in another tab does not end a stream on the previous one.
func (s *Server) stillAllowed(c *call, level access) error {
	user, err := s.store.UserBySession(c.token)
	if errors.Is(err, store.ErrNotFound) {
		return fail(http.StatusUnauthorized, plugin.CodeSessionExpired, reasonSessionEnded, nil)
	}
	if err != nil {
		return err
	}
	if user.MustChangePassword {
		return fail(http.StatusForbidden, plugin.CodePasswordChangeRequired, reasonPasswordChange, nil)
	}
	cluster, err := s.store.GetCluster(c.cluster.ID)
	if errors.Is(err, store.ErrNotFound) {
		return fail(http.StatusConflict, plugin.CodeNoCluster, reasonClusterRemoved, reason.Params{"cluster": c.cluster.Name})
	}
	if err != nil {
		return err
	}
	role, err := s.store.ClusterRole(user, cluster.ID)
	if err != nil {
		return err
	}
	if role == "" {
		return fail(http.StatusForbidden, plugin.CodeForbidden, reasonAccessLost, reason.Params{"cluster": cluster.Name})
	}
	if level == accessWrite || level == accessExec {
		return mutationDenied(role, cluster)
	}
	return nil
}

// streamKind names a kind of long-lived request for streamLimiter.
type streamKind string

const (
	kindLogStream streamKind = "log stream"
	kindTerminal  streamKind = "terminal"
	kindWatch     streamKind = "live-update stream"
)

// tooMany is the refusal of one stream more than limit.
func (k streamKind) tooMany() reason.Code {
	switch k {
	case kindTerminal:
		return reasonTooManyTerminals
	case kindWatch:
		return reasonTooManyWatches
	}
	return reasonTooManyLogs
}

func (k streamKind) limit() int {
	switch k {
	case kindTerminal:
		return maxTerminalsPerUser
	case kindWatch:
		return maxWatchStreamsPerUser
	}
	return maxLogStreamsPerUser
}

// streamLimiter counts each user's open long-lived requests by kind.
type streamLimiter struct {
	mu   sync.Mutex
	open map[streamSlot]int
}

type streamSlot struct {
	user int64
	kind streamKind
}

func newStreamLimiter() *streamLimiter { return &streamLimiter{open: map[streamSlot]int{}} }

// acquire takes one of user's slots of kind, or refuses with 429
// rate_limited when all are taken. release gives the slot back; it must
// be called exactly once.
func (l *streamLimiter) acquire(user int64, kind streamKind) (release func(), err error) {
	slot := streamSlot{user, kind}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.open[slot] >= kind.limit() {
		return nil, fail(http.StatusTooManyRequests, plugin.CodeRateLimited, kind.tooMany(), reason.Params{"max": kind.limit()})
	}
	l.open[slot]++
	var once sync.Once
	return func() {
		once.Do(func() {
			l.mu.Lock()
			defer l.mu.Unlock()
			if l.open[slot]--; l.open[slot] == 0 {
				delete(l.open, slot)
			}
		})
	}, nil
}

// endedMessage is what a stream tells its client when stillAllowed ended it.
func endedMessage(err error) string { return "the session was closed: " + err.Error() }
