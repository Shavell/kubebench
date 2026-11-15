package watch

import (
	"cmp"
	"errors"
	"log"
	"slices"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
)

// Defaults of Options.
const (
	DefaultIdle           = 5 * time.Minute
	DefaultMaxInformers   = 64
	DefaultMaxSubscribers = 512
	defaultProbeTimeout   = 15 * time.Second
	defaultRetry          = 30 * time.Second
)

var (
	// ErrClosed: the hub was closed; its cluster connection is gone.
	ErrClosed = errors.New("the cluster connection was closed")
	// ErrTooManySubscribers: the hub has Options.MaxSubscribers already.
	ErrTooManySubscribers = errors.New("too many live-update subscribers on this cluster")
)

// Options tunes a Hub; zero values take the defaults.
type Options struct {
	// Cluster names the cluster in log lines.
	Cluster string
	// Idle is how long an informer runs on without subscribers.
	Idle time.Duration
	// MaxInformers bounds the resource types watched at once; topics
	// beyond it are polled.
	MaxInformers int
	// MaxSubscribers bounds the open subscriptions.
	MaxSubscribers int
	// ProbeTimeout bounds the list request that checks access to a
	// resource before its informer starts.
	ProbeTimeout time.Duration
	// Retry is the pause before a failed probe (other than a denial) is
	// tried again.
	Retry time.Duration
	// Logf writes log lines; log.Printf by default.
	Logf func(format string, args ...any)
}

func (o Options) withDefaults() Options {
	o.Idle = cmp.Or(o.Idle, DefaultIdle)
	o.MaxInformers = cmp.Or(o.MaxInformers, DefaultMaxInformers)
	o.MaxSubscribers = cmp.Or(o.MaxSubscribers, DefaultMaxSubscribers)
	o.ProbeTimeout = cmp.Or(o.ProbeTimeout, defaultProbeTimeout)
	o.Retry = cmp.Or(o.Retry, defaultRetry)
	if o.Logf == nil {
		o.Logf = log.Printf
	}
	return o
}

// Hub fans the changes of one cluster out to subscriptions. It is safe
// for concurrent use.
type Hub struct {
	dyn     dynamic.Interface
	resolve Resolver
	opts    Options
	done    chan struct{} // closed by Close
	wg      sync.WaitGroup

	mu        sync.Mutex
	closed    bool
	informers map[schema.GroupVersionResource]*informer
	subs      map[*Subscription]struct{}
}

// NewHub returns a hub watching through dyn. It starts nothing until the
// first subscription.
func NewHub(dyn dynamic.Interface, resolve Resolver, opts Options) *Hub {
	return &Hub{
		dyn:       dyn,
		resolve:   resolve,
		opts:      opts.withDefaults(),
		done:      make(chan struct{}),
		informers: map[schema.GroupVersionResource]*informer{},
		subs:      map[*Subscription]struct{}{},
	}
}

// Subscribe subscribes to topics (valid, see ValidTopic, and without
// duplicates). The subscription's first statuses, pending at once, give
// every topic's mode: a topic that cannot be resolved, or that would
// exceed MaxInformers, is polled with the reason.
func (h *Hub) Subscribe(topics []string) (*Subscription, error) {
	// Resolving may ask the cluster (discovery), so not under h.mu.
	resolved := make(map[string]schema.GroupVersionResource, len(topics))
	failed := map[string]error{}
	for _, t := range topics {
		if gvr, err := h.resolve(t); err != nil {
			failed[t] = err
		} else {
			resolved[t] = gvr
		}
	}

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrClosed
	}
	if len(h.subs) >= h.opts.MaxSubscribers {
		return nil, ErrTooManySubscribers
	}
	sub := newSubscription(h)
	h.subs[sub] = struct{}{}
	for _, t := range topics {
		if err, ok := failed[t]; ok {
			sub.addStatus(TopicStatus{Topic: t, Mode: ModePolling, Reason: err.Error()})
			continue
		}
		gvr := resolved[t]
		inf := h.informers[gvr]
		if inf == nil {
			if len(h.informers) >= h.opts.MaxInformers {
				sub.addStatus(TopicStatus{Topic: t, Mode: ModePolling,
					Reason: "too many resource types are watched on this cluster at once"})
				continue
			}
			inf = h.startInformer(gvr)
		}
		inf.attach(sub, t)
		sub.watching[gvr] = append(sub.watching[gvr], t)
		sub.addStatus(TopicStatus{Topic: t, Mode: inf.mode, Reason: inf.reason})
	}
	return sub, nil
}

func (h *Hub) unsubscribe(sub *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	delete(h.subs, sub)
	for gvr := range sub.watching {
		if inf := h.informers[gvr]; inf != nil && inf.detach(sub) {
			h.armIdle(inf)
		}
	}
	clear(sub.watching)
}

// armIdle stops inf after Options.Idle unless it gets a subscriber
// before. h.mu is held.
func (h *Hub) armIdle(inf *informer) {
	inf.idleSince = time.Now()
	inf.idle = time.AfterFunc(h.opts.Idle, func() {
		h.mu.Lock()
		defer h.mu.Unlock()
		if h.closed || h.informers[inf.gvr] != inf || len(inf.subs) > 0 {
			return
		}
		delete(h.informers, inf.gvr)
		inf.cancel()
		h.opts.Logf("[watch] cluster=%s stopped informer %s (no subscribers for %s)",
			h.opts.Cluster, gvrName(inf.gvr), h.opts.Idle)
	})
}

// Close stops every informer, ends every subscription (Done) and waits
// until the informers' goroutines returned. It is safe to call more than
// once.
func (h *Hub) Close() {
	h.mu.Lock()
	if !h.closed {
		h.closed = true
		close(h.done)
		for _, inf := range h.informers {
			inf.stopIdle()
			inf.cancel()
		}
		if n := len(h.informers); n > 0 {
			h.opts.Logf("[watch] cluster=%s stopped %d informers (connection closed)", h.opts.Cluster, n)
		}
		clear(h.informers)
		clear(h.subs)
	}
	h.mu.Unlock()
	h.wg.Wait()
}

// InformerStats describes one running informer (GET /api/watch/status).
type InformerStats struct {
	Resource    string     `json:"resource"` // group/version/resource
	Mode        Mode       `json:"mode"`
	Reason      string     `json:"reason,omitempty"`
	Subscribers int        `json:"subscribers"`
	IdleSince   *time.Time `json:"idleSince,omitempty"` // when the last subscriber left
}

// Stats lists the informers, ordered by resource, and the number of open
// subscriptions.
func (h *Hub) Stats() (informers []InformerStats, subscribers int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	informers = make([]InformerStats, 0, len(h.informers))
	for _, inf := range h.informers {
		st := InformerStats{Resource: gvrName(inf.gvr), Mode: inf.mode, Reason: inf.reason, Subscribers: len(inf.subs)}
		if len(inf.subs) == 0 {
			since := inf.idleSince
			st.IdleSince = &since
		}
		informers = append(informers, st)
	}
	slices.SortFunc(informers, func(a, b InformerStats) int { return cmp.Compare(a.Resource, b.Resource) })
	return informers, len(h.subs)
}
