package api

import (
	"errors"
	"net/http"
	"sync"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// Long-lived requests (terminals, log streams) are authorized by the
// router when they start, like every request, but may run for hours. They
// check again while they run (stillAllowed) and are counted per user
// (streamLimiter), so that neither a revoked user nor a single client can
// keep them going without bound.

// Concurrent long-lived requests one user may have open.
const (
	maxLogStreamsPerUser = 20
	maxTerminalsPerUser  = 10
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
		return errorf(http.StatusUnauthorized, plugin.CodeSessionExpired, "your session has ended; sign in again")
	}
	if err != nil {
		return err
	}
	if user.MustChangePassword {
		return errorf(http.StatusForbidden, plugin.CodePasswordChangeRequired, "change your password to continue")
	}
	cluster, err := s.store.GetCluster(c.cluster.ID)
	if errors.Is(err, store.ErrNotFound) {
		return errorf(http.StatusConflict, plugin.CodeNoCluster, "cluster %q was removed", c.cluster.Name)
	}
	if err != nil {
		return err
	}
	role, err := s.store.ClusterRole(user, cluster.ID)
	if err != nil {
		return err
	}
	if role == "" {
		return errorf(http.StatusForbidden, plugin.CodeForbidden, "you no longer have access to cluster %q", cluster.Name)
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
)

func (k streamKind) limit() int {
	if k == kindTerminal {
		return maxTerminalsPerUser
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
		return nil, errorf(http.StatusTooManyRequests, plugin.CodeRateLimited,
			"you have %d %ss open, the most at once; close one first", kind.limit(), kind)
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
