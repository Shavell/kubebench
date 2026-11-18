package watch

import (
	"cmp"
	"log"
	"slices"
	"sync"
	"time"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"

	"kubebench/internal/reason"
)

// Defaults of Options.
const (
	DefaultIdle           = 5 * time.Minute
	DefaultMaxInformers   = 64
	DefaultMaxSubscribers = 512
	// DefaultMaxOwnerTypes is half of DefaultMaxInformers: one user
	// cannot take every informer of a cluster. It is also the topic limit
	// of one GET /api/watch, so a single stream always fits.
	DefaultMaxOwnerTypes = 32
	defaultProbeTimeout  = 15 * time.Second
	defaultRetry         = 30 * time.Second
)

var (
	// ErrClosed: the hub was closed; its cluster connection is gone.
	ErrClosed = reason.New(reasonClosed, nil)
	// ErrTooManySubscribers: the hub has Options.MaxSubscribers already.
	ErrTooManySubscribers = reason.New(reasonTooManyWatchers, nil)
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
	// MaxOwnerTypes bounds the resource types one owner (a user) watches
	// at once, across all of its subscriptions; topics beyond it are
	// polled.
	MaxOwnerTypes int
	// ProbeTimeout bounds the list request that checks access to a
	// resource before its informer starts.
	ProbeTimeout time.Duration
	// Retry is the pause before a failed probe (other than a denial), or
	// a topic that failed to resolve (other than NotServedError), is
	// tried again.
	Retry time.Duration
	// Logf writes log lines; log.Printf by default.
	Logf func(format string, args ...any)
}

func (o Options) withDefaults() Options {
	o.Idle = cmp.Or(o.Idle, DefaultIdle)
	o.MaxInformers = cmp.Or(o.MaxInformers, DefaultMaxInformers)
	o.MaxSubscribers = cmp.Or(o.MaxSubscribers, DefaultMaxSubscribers)
	o.MaxOwnerTypes = cmp.Or(o.MaxOwnerTypes, DefaultMaxOwnerTypes)
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
	// owned counts, per owner and resource type, the owner's
	// subscriptions watching it (see Options.MaxOwnerTypes).
	owned map[int64]map[schema.GroupVersionResource]int
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
		owned:     map[int64]map[schema.GroupVersionResource]int{},
	}
}

// Subscribe subscribes owner (a user ID) to topics (valid, see
// ValidTopic, and without duplicates). The subscription's first statuses,
// pending at once, give every topic's mode: a topic that cannot be
// resolved, or that would exceed MaxInformers or MaxOwnerTypes, is polled
// with the reason. A topic whose resolution failed for another reason
// than NotServedError is resolved again every Options.Retry until it
// succeeds, and then gets a new status.
func (h *Hub) Subscribe(owner int64, topics []string) (*Subscription, error) {
	// Resolving may ask the cluster (discovery), so not under h.mu.
	resolved, failed := h.resolveAll(topics)

	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return nil, ErrClosed
	}
	if len(h.subs) >= h.opts.MaxSubscribers {
		return nil, ErrTooManySubscribers
	}
	sub := newSubscription(h, owner)
	h.subs[sub] = struct{}{}
	var retry []string
	reasons := map[string]string{}
	for _, t := range topics {
		if err, ok := failed[t]; ok {
			sub.addStatus(topicStatus(t, ModePolling, reason.From(err)))
			if !isNotServed(err) {
				retry = append(retry, t)
				reasons[t] = err.Error()
			}
			continue
		}
		h.attachTopic(sub, t, resolved[t])
	}
	if len(retry) > 0 {
		h.wg.Go(func() { h.retryResolve(sub, retry, reasons) })
	}
	return sub, nil
}

// resolveAll resolves topics; h.mu is not held.
func (h *Hub) resolveAll(topics []string) (map[string]schema.GroupVersionResource, map[string]error) {
	resolved := make(map[string]schema.GroupVersionResource, len(topics))
	failed := map[string]error{}
	for _, t := range topics {
		if gvr, err := h.resolve(t); err != nil {
			failed[t] = err
		} else {
			resolved[t] = gvr
		}
	}
	return resolved, failed
}

// retryResolve resolves sub's topics that failed transiently every
// Options.Retry until they resolve, fail for good or sub ends. reasons
// holds each topic's last reported reason.
func (h *Hub) retryResolve(sub *Subscription, topics []string, reasons map[string]string) {
	for len(topics) > 0 {
		select {
		case <-h.done:
			return
		case <-sub.gone:
			return
		case <-time.After(h.opts.Retry):
		}
		resolved, failed := h.resolveAll(topics)
		h.mu.Lock()
		if _, ok := h.subs[sub]; !ok {
			h.mu.Unlock()
			return
		}
		var again []string
		for _, t := range topics {
			err, ok := failed[t]
			if !ok {
				h.attachTopic(sub, t, resolved[t])
				continue
			}
			if msg := err.Error(); msg != reasons[t] {
				reasons[t] = msg
				sub.addStatus(topicStatus(t, ModePolling, reason.From(err)))
			}
			if !isNotServed(err) {
				again = append(again, t)
			}
		}
		h.mu.Unlock()
		topics = again
	}
}

// attachTopic attaches topic of sub to the informer of gvr, starting it
// if needed, and adds the topic's status. h.mu is held.
func (h *Hub) attachTopic(sub *Subscription, topic string, gvr schema.GroupVersionResource) {
	_, watched := sub.watching[gvr]
	owned := h.owned[sub.owner]
	if !watched && owned[gvr] == 0 && len(owned) >= h.opts.MaxOwnerTypes {
		sub.addStatus(topicStatus(topic, ModePolling, reason.New(reasonOwnerTypes, nil)))
		return
	}
	inf := h.informers[gvr]
	if inf == nil {
		if len(h.informers) >= h.opts.MaxInformers {
			sub.addStatus(topicStatus(topic, ModePolling, reason.New(reasonClusterTypes, nil)))
			return
		}
		inf = h.startInformer(gvr)
	}
	inf.attach(sub, topic)
	sub.watching[gvr] = append(sub.watching[gvr], topic)
	if !watched {
		if owned == nil {
			owned = map[schema.GroupVersionResource]int{}
			h.owned[sub.owner] = owned
		}
		owned[gvr]++
	}
	sub.addStatus(topicStatus(topic, inf.mode, inf.reason))
}

func (h *Hub) unsubscribe(sub *Subscription) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return
	}
	delete(h.subs, sub)
	close(sub.gone)
	owned := h.owned[sub.owner]
	for gvr := range sub.watching {
		if inf := h.informers[gvr]; inf != nil && inf.detach(sub) {
			h.armIdle(inf)
		}
		if owned[gvr]--; owned[gvr] <= 0 {
			delete(owned, gvr)
		}
	}
	if len(owned) == 0 {
		delete(h.owned, sub.owner)
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
		clear(h.owned)
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
		st := InformerStats{Resource: gvrName(inf.gvr), Mode: inf.mode, Subscribers: len(inf.subs)}
		if inf.reason != nil {
			st.Reason = inf.reason.Error()
		}
		if len(inf.subs) == 0 {
			since := inf.idleSince
			st.IdleSince = &since
		}
		informers = append(informers, st)
	}
	slices.SortFunc(informers, func(a, b InformerStats) int { return cmp.Compare(a.Resource, b.Resource) })
	return informers, len(h.subs)
}
