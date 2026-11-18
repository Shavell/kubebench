package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
	"unicode/utf8"

	"k8s.io/apimachinery/pkg/labels"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// Limits of GET /api/logs/stream (design §7.1).
const (
	defaultStreamTail = 500
	maxStreamTail     = 10000
	maxStreamPods     = 10
	// maxLogEvent bounds the data of one event: longer log lines are sent
	// in parts, so neither side buffers a runaway line without limit.
	maxLogEvent = 64 << 10
)

// logStreamRequest is a parsed GET /api/logs/stream.
type logStreamRequest struct {
	namespace string
	pod       string // either one pod …
	selector  string // … or the pods a label selector matches
	container string
	opts      kube.LogFollowOptions
}

func parseLogStreamRequest(q url.Values) (logStreamRequest, error) {
	req := logStreamRequest{
		namespace: q.Get("ns"),
		pod:       q.Get("pod"),
		selector:  q.Get("selector"),
		container: q.Get("container"),
	}
	if req.namespace == "" {
		return req, badRequest(reason.FieldRequired, reason.Params{"field": "ns"})
	}
	if (req.pod == "") == (req.selector == "") {
		return req, badRequest(reasonPodOrSelector, nil)
	}
	if req.selector != "" {
		if _, err := labels.Parse(req.selector); err != nil {
			return req, badRequest(reasonSelector, reason.Params{"detail": err.Error()})
		}
	}
	tail := int64(defaultStreamTail)
	if raw := q.Get("tail"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 0 {
			return req, badRequest(reasonLogTail, nil)
		}
		tail = min(n, maxStreamTail)
	}
	if tail > 0 {
		req.opts.TailLines = &tail
	}
	var err error
	if req.opts.Previous, err = boolQuery(q, "previous"); err != nil {
		return req, err
	}
	if req.opts.Timestamps, err = boolQuery(q, "timestamps"); err != nil {
		return req, err
	}
	if raw := q.Get("sinceSeconds"); raw != "" {
		n, err := strconv.ParseInt(raw, 10, 64)
		if err != nil || n < 1 {
			return req, badRequest(reason.FieldMin, reason.Params{"field": "sinceSeconds", "min": 1})
		}
		req.opts.SinceSeconds = n
	}
	return req, nil
}

// boolQuery parses an optional true/false query parameter.
func boolQuery(q url.Values, name string) (bool, error) {
	raw := q.Get(name)
	if raw == "" {
		return false, nil
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		return false, badRequest(reason.FieldBoolean, reason.Params{"field": name})
	}
	return v, nil
}

// handleLogStream follows container logs as Server-Sent Events: one
// "data:" line per log line, "event: pods" first with the followed
// containers, "event: error" when a log fails and "event: end" when every
// log ended. Problems found before the stream starts (bad parameters, a
// missing pod or container, no matching pod) are ordinary JSON errors.
func (s *Server) handleLogStream(w http.ResponseWriter, r *http.Request, c *call) {
	req, err := parseLogStreamRequest(r.URL.Query())
	if err != nil {
		writeErr(w, err)
		return
	}
	release, err := s.streamLimits.acquire(c.user.ID, kindLogStream)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer release()
	// The server's shutdown ends the stream too (see streamLogs).
	ctx, cancel := s.streamContext(r.Context())
	defer cancel()

	var feeds []logFeed
	matched := 1
	if req.pod != "" {
		src, err := c.rt.PodLogSource(ctx, req.namespace, req.pod, req.container)
		if err != nil {
			plugin.WriteK8sError(w, err)
			return
		}
		// Opened before the response starts, so that e.g. a missing
		// previous container is reported with its real status.
		body, err := c.rt.FollowLog(ctx, req.namespace, src, req.opts)
		if err != nil {
			plugin.WriteK8sError(w, err)
			return
		}
		feeds = []logFeed{{source: src, open: func(context.Context) (io.ReadCloser, error) { return body, nil }}}
	} else {
		var sources []kube.LogSource
		sources, matched, err = c.rt.SelectorLogSources(ctx, req.namespace, req.selector, req.container, maxStreamPods)
		if err != nil {
			plugin.WriteK8sError(w, err)
			return
		}
		if len(sources) == 0 {
			writeErr(w, fail(http.StatusNotFound, plugin.CodeNotFound, reasonNoPodMatches,
				reason.Params{"namespace": req.namespace, "selector": req.selector}))
			return
		}
		for _, src := range sources {
			feeds = append(feeds, logFeed{source: src, prefixed: true, open: func(ctx context.Context) (io.ReadCloser, error) {
				return c.rt.FollowLog(ctx, req.namespace, src, req.opts)
			}})
		}
	}

	sse, err := startSSE(w, s.streams.logWriteWait)
	defer sse.end()
	if err != nil {
		return // the client is gone
	}
	// A logout or a revoked assignment ends the stream too.
	streamLogs(ctx, sse, feeds, matched, s.streams.logHeartbeat, func() error { return s.stillAllowed(c, accessRead) })
}

// logFeed is one container log of a stream.
type logFeed struct {
	source kube.LogSource
	// prefixed starts every line with "[pod/container] " (selector streams).
	prefixed bool
	open     func(ctx context.Context) (io.ReadCloser, error)
}

func (f logFeed) prefix() string {
	if !f.prefixed {
		return ""
	}
	return "[" + f.source.String() + "] "
}

// logEvent is what a feed hands to the writer: a line, or an error.
type logEvent struct {
	name string // "" for a log line, "error"
	data string
}

// podsEvent is the data of the first event: the containers the stream
// follows ("pod/container") and how many pods matched (at most
// maxStreamPods are followed).
type podsEvent struct {
	Pods  []string `json:"pods"`
	Total int      `json:"total"`
}

// streamLogs runs feeds concurrently and writes their lines to sse until
// all of them ended (then "event: end"), the client went away or stopped
// reading, ctx ended, or check (run every heartbeat, when set) failed or
// the server stopped (ctx from streamContext; both end with "event:
// error" and the reason). Idle streams get a heartbeat comment every
// heartbeat. It returns once every feed has stopped and closed its log.
func streamLogs(ctx context.Context, sse *sseWriter, feeds []logFeed, matched int, heartbeat time.Duration, check func() error) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	announce := podsEvent{Pods: make([]string, 0, len(feeds)), Total: matched}
	for _, f := range feeds {
		announce.Pods = append(announce.Pods, f.source.String())
	}
	data, _ := json.Marshal(announce)
	if sse.event("pods", string(data)) != nil || sse.flush() != nil {
		return
	}

	events := make(chan logEvent, 256)
	var wg sync.WaitGroup
	for _, f := range feeds {
		wg.Go(func() { f.run(ctx, events) })
	}
	go func() {
		wg.Wait()
		close(events)
	}()
	defer func() {
		cancel()
		for range events { // the feeds stop once ctx is done
		}
	}()

	// The feeds stop as well when the server stops, so its end is
	// reported whichever comes first.
	stopped := func() bool {
		if !shuttingDown(ctx) {
			return false
		}
		_ = sse.event("error", errShuttingDown.Error())
		_ = sse.flush()
		return true
	}
	ticker := time.NewTicker(heartbeat)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			stopped()
			return
		case <-ticker.C:
			if check != nil {
				if err := check(); err != nil {
					_ = sse.event("error", endedMessage(err))
					_ = sse.flush()
					return
				}
			}
			if sse.comment("ping") != nil || sse.flush() != nil {
				return
			}
		case ev, ok := <-events:
			if !ok {
				if stopped() {
					return
				}
				_ = sse.event("end", "")
				_ = sse.flush()
				return
			}
			if sse.event(ev.name, ev.data) != nil {
				return
			}
			// Flush once a burst is written rather than after every line.
			if len(events) == 0 && sse.flush() != nil {
				return
			}
		}
	}
}

// run opens the feed's log and sends its lines, then an error event if
// the log failed. It returns when the log ends or ctx is done.
func (f logFeed) run(ctx context.Context, events chan<- logEvent) {
	send := func(ev logEvent) bool {
		select {
		case events <- ev:
			return true
		case <-ctx.Done():
			return false
		}
	}
	prefix := f.prefix()
	body, err := f.open(ctx)
	if err != nil {
		send(logEvent{name: "error", data: prefix + err.Error()})
		return
	}
	defer body.Close()
	// Closing the body ends a read in progress, also for a log opened
	// before the stream started (under a longer-lived context).
	stop := context.AfterFunc(ctx, func() { _ = body.Close() })
	defer stop()
	err = readLines(body, maxLogEvent, func(line string) bool {
		return send(logEvent{data: prefix + line})
	})
	if err != nil && ctx.Err() == nil {
		send(logEvent{name: "error", data: prefix + err.Error()})
	}
}

// readLines calls emit with every line of r, without its line break and
// with carriage returns removed (SSE reads them as line breaks). A line
// longer than max bytes is emitted in parts of at most max bytes, cut at
// UTF-8 character boundaries. It stops early when emit returns false.
func readLines(r io.Reader, max int, emit func(string) bool) error {
	br := bufio.NewReaderSize(r, max)
	var line []byte
	for {
		chunk, err := br.ReadSlice('\n')
		line = append(line, chunk...)
		// A line is complete at its line break, and also when the log ends
		// or fails after a last line without one.
		complete := len(line) > 0 && !errors.Is(err, bufio.ErrBufferFull)
		if complete && line[len(line)-1] == '\n' {
			line = line[:len(line)-1]
		}
		for len(line) > max {
			cut := runeCut(line, max)
			if !emit(stripCR(line[:cut])) {
				return nil
			}
			line = line[:copy(line, line[cut:])]
		}
		if complete {
			if !emit(stripCR(line)) {
				return nil
			}
			line = line[:0]
		}
		switch {
		case err == nil, errors.Is(err, bufio.ErrBufferFull):
		case errors.Is(err, io.EOF):
			return nil
		default:
			return err
		}
	}
}

// runeCut is the largest cut point ≤ n that does not split a UTF-8
// character of b (n itself when b is not valid UTF-8 there).
func runeCut(b []byte, n int) int {
	if n >= len(b) {
		return len(b)
	}
	for cut := n; cut > 0 && cut > n-utf8.UTFMax; cut-- {
		if utf8.RuneStart(b[cut]) {
			return cut
		}
	}
	return n
}

func stripCR(b []byte) string {
	out := make([]byte, 0, len(b))
	for _, c := range b {
		if c != '\r' {
			out = append(out, c)
		}
	}
	return string(out)
}
