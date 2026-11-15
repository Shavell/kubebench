package watch

import (
	"cmp"
	"slices"
	"sync"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

// maxPendingChanges bounds the changes a subscription collects between two
// takes. Beyond it the pending changes collapse to one per topic in every
// namespace: a flood of changes costs a client one refresh per topic.
const maxPendingChanges = 256

// Subscription receives the changes and status updates of its topics.
// Notify signals that some are pending; TakeStatuses and TakeChanges
// collect them. Close it when done.
type Subscription struct {
	hub    *Hub
	owner  int64
	notify chan struct{}
	gone   chan struct{} // closed once unsubscribed

	// watching maps each informer the subscription is attached to to the
	// subscribed topics it serves (two topics may name one resource).
	// Guarded by hub.mu.
	watching map[schema.GroupVersionResource][]string

	mu       sync.Mutex
	changes  map[Change]struct{}
	statuses map[string]TopicStatus // the latest untaken status per topic
	closed   bool
}

func newSubscription(h *Hub, owner int64) *Subscription {
	return &Subscription{
		hub:      h,
		owner:    owner,
		notify:   make(chan struct{}, 1),
		gone:     make(chan struct{}),
		watching: map[schema.GroupVersionResource][]string{},
		changes:  map[Change]struct{}{},
		statuses: map[string]TopicStatus{},
	}
}

// Notify receives a value whenever changes or statuses became pending
// since the last receive.
func (s *Subscription) Notify() <-chan struct{} { return s.notify }

// Done is closed when the hub closed (its cluster connection was rebuilt
// or removed): the subscription receives nothing more.
func (s *Subscription) Done() <-chan struct{} { return s.hub.done }

// Close detaches the subscription from its informers. It is safe to call
// more than once.
func (s *Subscription) Close() {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return
	}
	s.closed = true
	s.mu.Unlock()
	s.hub.unsubscribe(s)
}

// TakeStatuses returns the status updates since the last take, one per
// topic (the latest), ordered by topic.
func (s *Subscription) TakeStatuses() []TopicStatus {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]TopicStatus, 0, len(s.statuses))
	for _, st := range s.statuses {
		out = append(out, st)
	}
	clear(s.statuses)
	slices.SortFunc(out, func(a, b TopicStatus) int { return cmp.Compare(a.Topic, b.Topic) })
	return out
}

// HasChanges reports whether changes are pending.
func (s *Subscription) HasChanges() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.changes) > 0
}

// TakeChanges returns the changes since the last take, without
// duplicates, ordered by topic and namespace.
func (s *Subscription) TakeChanges() []Change {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Change, 0, len(s.changes))
	for c := range s.changes {
		out = append(out, c)
	}
	clear(s.changes)
	slices.SortFunc(out, func(a, b Change) int {
		return cmp.Or(cmp.Compare(a.Topic, b.Topic), cmp.Compare(a.Namespace, b.Namespace))
	})
	return out
}

func (s *Subscription) addChange(c Change) {
	s.mu.Lock()
	// A pending change in every namespace ("") covers c already.
	_, covered := s.changes[Change{Topic: c.Topic}]
	if _, seen := s.changes[c]; !seen && !covered {
		if len(s.changes) >= maxPendingChanges {
			s.collapseChanges()
			c.Namespace = ""
		}
		s.changes[c] = struct{}{}
	}
	s.mu.Unlock()
	s.signal()
}

// collapseChanges replaces the pending changes by one per topic in every
// namespace. s.mu is held.
func (s *Subscription) collapseChanges() {
	topics := map[string]struct{}{}
	for c := range s.changes {
		topics[c.Topic] = struct{}{}
	}
	clear(s.changes)
	for t := range topics {
		s.changes[Change{Topic: t}] = struct{}{}
	}
}

func (s *Subscription) addStatus(st TopicStatus) {
	s.mu.Lock()
	s.statuses[st.Topic] = st
	s.mu.Unlock()
	s.signal()
}

func (s *Subscription) signal() {
	select {
	case s.notify <- struct{}{}:
	default: // a signal is pending already
	}
}
