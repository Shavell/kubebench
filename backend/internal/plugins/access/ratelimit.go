package access

import (
	"sync"
	"time"
)

// Access reviews per user: each POST /review creates a
// SubjectAccessReview, so a script must not flood the API server with them.
const (
	reviewsPerWindow = 30
	reviewWindow     = time.Minute
)

// reviewLimiter allows reviewsPerWindow reviews per user in a fixed window
// that starts with the user's first review.
type reviewLimiter struct {
	mu      sync.Mutex
	now     func() time.Time
	windows map[int64]*reviewCount
}

type reviewCount struct {
	start time.Time
	n     int
}

func newReviewLimiter(now func() time.Time) *reviewLimiter {
	return &reviewLimiter{now: now, windows: map[int64]*reviewCount{}}
}

// take counts one review of user and returns 0, or how long the user must
// wait when the window's reviews are used up (the review is not counted).
func (l *reviewLimiter) take(user int64) time.Duration {
	l.mu.Lock()
	defer l.mu.Unlock()
	now := l.now()
	for id, w := range l.windows { // a handful of users: prune as we go
		if now.Sub(w.start) >= reviewWindow {
			delete(l.windows, id)
		}
	}
	w := l.windows[user]
	if w == nil {
		l.windows[user] = &reviewCount{start: now, n: 1}
		return 0
	}
	if w.n >= reviewsPerWindow {
		return w.start.Add(reviewWindow).Sub(now)
	}
	w.n++
	return 0
}
