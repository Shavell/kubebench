package api

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"strings"
	"time"

	"kubebench/internal/plugin"
	"kubebench/internal/watch"
)

// maxWatchTopics bounds the topics of one GET /api/watch.
const maxWatchTopics = 32

// parseWatchTopics parses the comma-separated topics parameter: 1 to
// maxWatchTopics valid topics, duplicates dropped, order kept.
func parseWatchTopics(raw string) ([]string, error) {
	var topics []string
	seen := map[string]bool{}
	for t := range strings.SplitSeq(raw, ",") {
		t = strings.TrimSpace(t)
		if t == "" || seen[t] {
			continue
		}
		if !watch.ValidTopic(t) {
			return nil, badRequest("invalid topic %q: give a resource like pods or applications.argoproj.io", t)
		}
		seen[t] = true
		topics = append(topics, t)
	}
	if len(topics) == 0 {
		return nil, badRequest("topics is required")
	}
	if len(topics) > maxWatchTopics {
		return nil, badRequest("at most %d topics per stream", maxWatchTopics)
	}
	return topics, nil
}

// handleWatch streams the changes of the active cluster's resources as
// Server-Sent Events (§7.10): "event: status" with a topic's mode (live,
// starting or polling with the reason) whenever it changes, the first
// ones at once; "event: change" with {topic, namespace}, debounced;
// heartbeat comments; and "event: end" with the reason when the server
// ends the stream (the cluster connection was rebuilt, or the caller's
// access changed), after which clients may reconnect.
func (s *Server) handleWatch(w http.ResponseWriter, r *http.Request, c *call) {
	topics, err := parseWatchTopics(r.URL.Query().Get("topics"))
	if err != nil {
		writeErr(w, err)
		return
	}
	release, err := s.streamLimits.acquire(c.user.ID, kindWatch)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer release()
	sub, err := c.rt.Watch().Subscribe(c.user.ID, topics)
	switch {
	case errors.Is(err, watch.ErrClosed):
		writeErr(w, errorf(http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable,
			"the connection to cluster %q is being rebuilt; try again", c.cluster.Name))
		return
	case errors.Is(err, watch.ErrTooManySubscribers):
		writeErr(w, errorf(http.StatusTooManyRequests, plugin.CodeRateLimited, "%v", err))
		return
	case err != nil:
		writeErr(w, err)
		return
	}
	defer sub.Close()

	sse, err := startSSE(w, s.streams.watchWriteWait)
	defer sse.end()
	if err != nil {
		return // the client is gone
	}
	// A logout or a revoked assignment ends the stream too.
	streamChanges(r.Context(), sse, sub, s.streams.watchTimings, func() error { return s.stillAllowed(c, accessRead) })
}

// watchTimings are the intervals of a watch stream.
type watchTimings struct {
	// heartbeat is the interval of the comment lines that keep an idle
	// stream open through proxies; the caller's access is checked again
	// at the same interval.
	heartbeat time.Duration
	// debounce is how long changes are collected before they are sent:
	// a burst (a rollout, a namespace deletion) becomes one event per
	// topic and namespace.
	debounce time.Duration
}

// streamChanges writes sub's statuses and changes to sse until ctx ends,
// the client stops reading, the hub closes or check fails (then
// "event: end" with the reason).
func streamChanges(ctx context.Context, sse *sseWriter, sub *watch.Subscription, timings watchTimings, check func() error) {
	heartbeat := time.NewTicker(timings.heartbeat)
	defer heartbeat.Stop()
	// debounced fires once the changes collected since the first pending
	// one are due; nil while none are pending.
	var debounced <-chan time.Time
	end := func(reason string) {
		_ = sse.event("end", reason)
		_ = sse.flush()
	}
	for {
		select {
		case <-ctx.Done():
			return
		case <-sub.Done():
			end("the cluster connection was rebuilt")
			return
		case <-heartbeat.C:
			if err := check(); err != nil {
				end(endedMessage(err))
				return
			}
			if sse.comment("ping") != nil || sse.flush() != nil {
				return
			}
		case <-sub.Notify():
			for _, st := range sub.TakeStatuses() {
				if writeEvent(sse, "status", st) != nil {
					return
				}
			}
			if sse.flush() != nil {
				return
			}
			if debounced == nil && sub.HasChanges() {
				debounced = time.After(timings.debounce)
			}
		case <-debounced:
			debounced = nil
			for _, ch := range sub.TakeChanges() {
				if writeEvent(sse, "change", ch) != nil {
					return
				}
			}
			if sse.flush() != nil {
				return
			}
		}
	}
}

func writeEvent(sse *sseWriter, name string, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	return sse.event(name, string(data))
}

// watchStatus is the body of GET /api/watch/status: the active cluster's
// informers, for diagnostics.
type watchStatus struct {
	Subscribers int                   `json:"subscribers"`
	Informers   []watch.InformerStats `json:"informers"`
}

func (s *Server) handleWatchStatus(w http.ResponseWriter, _ *http.Request, c *call) {
	informers, subscribers := c.rt.Watch().Stats()
	plugin.WriteJSON(w, watchStatus{Subscribers: subscribers, Informers: informers})
}
