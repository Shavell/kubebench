package watch

import (
	"cmp"
	"regexp"
	"slices"
	"sync"
)

// InternalPrefix starts the topics of KubeBench's own data (shared notes,
// …) as opposed to Kubernetes resources: "kubebench:notes". The colon
// cannot occur in a resource name, so the two never collide.
const InternalPrefix = "kubebench:"

var internalPattern = regexp.MustCompile(`^kubebench:[a-z0-9]([-a-z0-9]{0,38}[a-z0-9])?$`)

// IsInternalTopic reports whether topic names KubeBench data
// ("kubebench:<name>", the name like a plugin id) rather than a resource.
func IsInternalTopic(topic string) bool { return internalPattern.MatchString(topic) }

// Audience is who may learn that a KubeBench-internal topic changed: the
// subscribers watching cluster ClusterID (data every user of the cluster
// sees, such as shared notes), or user UserID on whatever cluster they
// watch (their personal data). Exactly one of the two is set.
type Audience struct {
	ClusterID int64
	UserID    int64
}

// Board fans the changes of KubeBench-internal topics out to the
// subscriptions of every cluster. Unlike a Hub it watches nothing: the
// code that changes the data publishes. It is safe for concurrent use;
// the zero value is not usable, call NewBoard.
type Board struct {
	mu   sync.Mutex
	subs map[*BoardSubscription]struct{}
}

// NewBoard returns an empty board.
func NewBoard() *Board {
	return &Board{subs: map[*BoardSubscription]struct{}{}}
}

// BoardSubscription receives the changes of its internal topics. Notify
// signals that some are pending; TakeChanges collects them. Close it when
// done.
type BoardSubscription struct {
	board     *Board
	userID    int64
	clusterID int64
	topics    []string
	notify    chan struct{}

	mu      sync.Mutex
	pending map[string]struct{}
}

// Subscribe subscribes user userID, watching cluster clusterID, to
// topics (internal topics, see IsInternalTopic). The caller has checked
// the user's access to the cluster and keeps checking it while the
// subscription lives.
func (b *Board) Subscribe(userID, clusterID int64, topics []string) *BoardSubscription {
	sub := &BoardSubscription{
		board:     b,
		userID:    userID,
		clusterID: clusterID,
		topics:    slices.Clone(topics),
		notify:    make(chan struct{}, 1),
		pending:   map[string]struct{}{},
	}
	b.mu.Lock()
	b.subs[sub] = struct{}{}
	b.mu.Unlock()
	return sub
}

// Publish tells the subscriptions of topic in audience that it changed.
// A zero Audience reaches nobody.
func (b *Board) Publish(topic string, audience Audience) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for sub := range b.subs {
		if sub.reaches(topic, audience) {
			sub.add(topic)
		}
	}
}

// Subscribers is the number of open subscriptions.
func (b *Board) Subscribers() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.subs)
}

func (s *BoardSubscription) reaches(topic string, a Audience) bool {
	if !slices.Contains(s.topics, topic) {
		return false
	}
	switch {
	case a.UserID != 0:
		return a.UserID == s.userID
	case a.ClusterID != 0:
		return a.ClusterID == s.clusterID
	}
	return false
}

func (s *BoardSubscription) add(topic string) {
	s.mu.Lock()
	s.pending[topic] = struct{}{}
	s.mu.Unlock()
	select {
	case s.notify <- struct{}{}:
	default: // a signal is pending already
	}
}

// Notify receives a value whenever changes became pending since the last
// receive.
func (s *BoardSubscription) Notify() <-chan struct{} { return s.notify }

// TakeChanges returns the changed topics since the last take, ordered by
// topic, as changes "anywhere" (Namespace "").
func (s *BoardSubscription) TakeChanges() []Change {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Change, 0, len(s.pending))
	for t := range s.pending {
		out = append(out, Change{Topic: t})
	}
	clear(s.pending)
	slices.SortFunc(out, func(a, b Change) int { return cmp.Compare(a.Topic, b.Topic) })
	return out
}

// Close ends the subscription. It is safe to call more than once.
func (s *BoardSubscription) Close() {
	s.board.mu.Lock()
	delete(s.board.subs, s)
	s.board.mu.Unlock()
}
