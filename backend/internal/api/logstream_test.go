package api

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// addPod creates a pod with the given containers and labels; the fake
// clientset answers every log request of it with "fake logs".
func addPod(t *testing.T, e *testEnv, name string, labels map[string]string, containers ...string) {
	t.Helper()
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default", Labels: labels}}
	for _, c := range containers {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: c})
	}
	if _, err := e.cs.CoreV1().Pods("default").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

// logRequests returns the options of the log requests sent to the cluster
// (the fake records no pod name for them).
func logRequests(e *testEnv) []*corev1.PodLogOptions {
	var out []*corev1.PodLogOptions
	for _, a := range e.cs.Actions() {
		if a.GetSubresource() == "log" {
			out = append(out, a.(k8stesting.GenericAction).GetValue().(*corev1.PodLogOptions))
		}
	}
	return out
}

// onlyLogRequest is the options of the one log request sent to the cluster.
func onlyLogRequest(t *testing.T, e *testEnv) *corev1.PodLogOptions {
	t.Helper()
	reqs := logRequests(e)
	if len(reqs) != 1 {
		t.Fatalf("log requests = %d, want 1", len(reqs))
	}
	return reqs[0]
}

// logReply makes the fake cluster answer log requests with body, or err.
func logReply(e *testEnv, body string, err error) {
	e.cs.PrependReactor("get", "pods/log", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &runtime.Unknown{Raw: []byte(body)}, err
	})
}

func TestLogStreamOnePod(t *testing.T) {
	e := newTestEnv(t, nil)
	addPod(t, e, "api-1", nil, "app", "sidecar")

	rec := e.do(http.MethodGet, "/api/logs/stream?ns=default&pod=api-1", "bob", nil) // viewers may read logs
	expect(t, rec, http.StatusOK, "")
	for header, want := range map[string]string{
		"Content-Type": "text/event-stream", "Cache-Control": "no-cache", "X-Accel-Buffering": "no",
	} {
		if got := rec.Header().Get(header); got != want {
			t.Errorf("%s = %q, want %q", header, got, want)
		}
	}
	want := "event: pods\ndata: {\"pods\":[\"api-1/app\"],\"total\":1}\n\n" +
		"data: fake logs\n\n" +
		"event: end\ndata: \n\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("stream =\n%q\nwant\n%q", got, want)
	}
	opts := onlyLogRequest(t, e)
	if !opts.Follow || opts.Container != "app" || opts.TailLines == nil || *opts.TailLines != defaultStreamTail ||
		opts.Previous || opts.Timestamps || opts.SinceSeconds != nil {
		t.Fatalf("log request = %+v, want a follow of the default container with the default tail", opts)
	}
	if rows := e.audit(); len(rows) != 0 {
		t.Fatalf("a log stream was audited: %+v", rows)
	}
}

func TestLogStreamOptions(t *testing.T) {
	e := newTestEnv(t, nil)
	addPod(t, e, "api-1", nil, "app", "sidecar")
	ptr := func(n int64) *int64 { return &n }
	for _, tc := range []struct {
		query string
		want  corev1.PodLogOptions
	}{
		{"tail=0", corev1.PodLogOptions{Container: "app", Follow: true}},
		{"tail=20000", corev1.PodLogOptions{Container: "app", Follow: true, TailLines: ptr(maxStreamTail)}},
		{"tail=100&previous=true&timestamps=true&sinceSeconds=60&container=sidecar",
			corev1.PodLogOptions{Container: "sidecar", Follow: true, TailLines: ptr(100), Previous: true, Timestamps: true, SinceSeconds: ptr(60)}},
	} {
		e.cs.ClearActions()
		expect(t, e.do(http.MethodGet, "/api/logs/stream?ns=default&pod=api-1&"+tc.query, "alice", nil), http.StatusOK, "")
		got := onlyLogRequest(t, e)
		if got.Container != tc.want.Container || got.Follow != tc.want.Follow ||
			fmt.Sprint(deref(got.TailLines)) != fmt.Sprint(deref(tc.want.TailLines)) || got.Previous != tc.want.Previous ||
			got.Timestamps != tc.want.Timestamps || fmt.Sprint(deref(got.SinceSeconds)) != fmt.Sprint(deref(tc.want.SinceSeconds)) {
			t.Errorf("%s: log request = %+v, want %+v", tc.query, got, tc.want)
		}
	}

	for _, query := range []string{
		"pod=api-1",                             // no namespace
		"ns=default",                            // neither pod nor selector
		"ns=default&pod=api-1&selector=app=api", // both
		"ns=default&selector=app+in+(",          // invalid selector
		"ns=default&pod=api-1&tail=-1",
		"ns=default&pod=api-1&tail=lots",
		"ns=default&pod=api-1&previous=maybe",
		"ns=default&pod=api-1&timestamps=1x",
		"ns=default&pod=api-1&sinceSeconds=0",
	} {
		e.cs.ClearActions()
		expect(t, e.do(http.MethodGet, "/api/logs/stream?"+query, "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
		if actions := e.cs.Actions(); len(actions) != 0 {
			t.Errorf("%s reached the cluster: %v", query, actions)
		}
	}
}

func deref(p *int64) any {
	if p == nil {
		return nil
	}
	return *p
}

func TestLogStreamFraming(t *testing.T) {
	e := newTestEnv(t, nil)
	addPod(t, e, "api-1", nil, "app")
	logReply(e, "2026-11-15T10:00:00.000000001Z first\r\n\nprogress 10%\rprogress 20%\nno final newline", nil)
	rec := e.do(http.MethodGet, "/api/logs/stream?ns=default&pod=api-1&timestamps=true", "alice", nil)
	want := []string{
		"event: pods\ndata: {\"pods\":[\"api-1/app\"],\"total\":1}",
		"data: 2026-11-15T10:00:00.000000001Z first",
		"data: ",
		"data: progress 10%progress 20%",
		"data: no final newline",
		"event: end\ndata: ",
	}
	if got := events(rec.Body.String()); !slices.Equal(got, want) {
		t.Fatalf("events =\n%q\nwant\n%q", got, want)
	}
}

func TestLogStreamErrorsBeforeTheStream(t *testing.T) {
	e := newTestEnv(t, nil)
	addPod(t, e, "api-1", map[string]string{"app": "api"}, "app")
	expect(t, e.do(http.MethodGet, "/api/logs/stream?ns=default&pod=gone", "alice", nil), http.StatusNotFound, plugin.CodeKubernetes)
	expect(t, e.do(http.MethodGet, "/api/logs/stream?ns=default&pod=api-1&container=nope", "alice", nil), http.StatusBadRequest, plugin.CodeKubernetes)
	expect(t, e.do(http.MethodGet, "/api/logs/stream?ns=default&selector=app%3Dnone", "alice", nil), http.StatusNotFound, plugin.CodeNotFound)
	expect(t, e.do(http.MethodGet, "/api/logs/stream?ns=default&selector=app%3Dapi&container=nope", "alice", nil), http.StatusNotFound, plugin.CodeNotFound)
	expect(t, e.do(http.MethodGet, "/api/logs/stream?ns=default&pod=api-1", "", nil), http.StatusUnauthorized, plugin.CodeUnauthenticated)

	// The log of one pod is opened before the response starts, so its
	// failure keeps the Kubernetes status.
	logReply(e, "", apierrors.NewBadRequest(`previous terminated container "app" in pod "api-1" not found`))
	rec := e.do(http.MethodGet, "/api/logs/stream?ns=default&pod=api-1&previous=true", "alice", nil)
	expect(t, rec, http.StatusBadRequest, plugin.CodeKubernetes)
	if !strings.Contains(rec.Body.String(), "previous terminated container") {
		t.Fatalf("body = %s", rec.Body.String())
	}
	// Selector streams report a failing pod inside the stream.
	rec = e.do(http.MethodGet, "/api/logs/stream?ns=default&selector=app%3Dapi&previous=true", "alice", nil)
	got := events(rec.Body.String())
	if len(got) != 3 || !strings.HasPrefix(got[1], "event: error\ndata: {\"error\":\"[api-1/app] ") || !strings.Contains(got[1], "previous terminated container") {
		t.Fatalf("events = %q", got)
	}
}

// events splits an event stream into its events (without the trailing blank line).
func events(stream string) []string {
	return slices.DeleteFunc(strings.Split(stream, "\n\n"), func(s string) bool { return s == "" })
}

// streamUntil serves a log stream request as user until the stream's
// body satisfies done (or ends), then cancels it like a leaving client
// and returns the body.
func streamUntil(t *testing.T, e *testEnv, target, user string, done func(body string) bool) string {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, target, nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.tokens[user]})
	w := &syncRecorder{header: http.Header{}}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		e.handler.ServeHTTP(w, req)
	}()
	deadline := time.After(5 * time.Second)
	for !done(w.String()) {
		select {
		case <-finished:
			return w.String()
		case <-deadline:
			cancel()
			<-finished
			t.Fatalf("stream %s: condition not met in %q", target, w.String())
		case <-time.After(5 * time.Millisecond):
		}
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream outlived its client")
	}
	return w.String()
}

// hasEvents reports whether body holds at least n events.
func hasEvents(n int) func(string) bool {
	return func(body string) bool { return len(events(body)) >= n }
}

func TestLogStreamSelectorFanOut(t *testing.T) {
	e := newTestEnv(t, nil)
	addPod(t, e, "api-2", map[string]string{"app": "api"}, "app")
	addPod(t, e, "api-1", map[string]string{"app": "api"}, "app", "sidecar")
	addPod(t, e, "worker", map[string]string{"app": "worker"}, "app")

	// A live selector stream does not end with its logs: new pods may come.
	body := streamUntil(t, e, "/api/logs/stream?ns=default&selector=app%3Dapi", "bob", hasEvents(3))
	got := events(body)
	if len(got) != 3 || got[0] != `event: pods`+"\n"+`data: {"pods":["api-1/app","api-2/app"],"total":2}` {
		t.Fatalf("events = %q", got)
	}
	lines := []string{got[1], got[2]}
	slices.Sort(lines)
	if lines[0] != "data: [api-1/app] fake logs" || lines[1] != "data: [api-2/app] fake logs" {
		t.Fatalf("lines = %q, want one prefixed line per pod", lines)
	}

	// With a container, pods without it do not match.
	body = streamUntil(t, e, "/api/logs/stream?ns=default&selector=app%3Dapi&container=sidecar", "bob", hasEvents(2))
	if got := events(body); len(got) != 2 || got[1] != "data: [api-1/sidecar] fake logs" {
		t.Fatalf("events = %q", got)
	}

	// A previous container's log does not grow: that stream ends.
	rec := e.do(http.MethodGet, "/api/logs/stream?ns=default&selector=app%3Dapi&previous=true", "bob", nil)
	if got := events(rec.Body.String()); len(got) != 4 || got[3] != "event: end\ndata: " {
		t.Fatalf("previous: events = %q", got)
	}

	// At most maxStreamPods pods are followed; the first event tells.
	for i := range 12 {
		addPod(t, e, fmt.Sprintf("many-%02d", i), map[string]string{"app": "many"}, "app")
	}
	e.cs.ClearActions()
	body = streamUntil(t, e, "/api/logs/stream?ns=default&selector=app%3Dmany", "bob", hasEvents(1+maxStreamPods))
	got = events(body)
	if !strings.HasPrefix(got[0], `event: pods`+"\n"+`data: {"pods":["many-00/app",`) || !strings.HasSuffix(got[0], `"many-09/app"],"total":12}`) {
		t.Fatalf("first event = %q", got[0])
	}
	if n := len(logRequests(e)); n != maxStreamPods {
		t.Fatalf("followed %d pods, want %d", n, maxStreamPods)
	}
}

// A selector stream follows pods that start to match later and a pod
// recreated under the same name.
func TestLogStreamSelectorFollowsNewPods(t *testing.T) {
	e := newTestEnv(t, nil)
	e.srv.streams.logResolve = 10 * time.Millisecond
	addPod(t, e, "api-1", map[string]string{"app": "api"}, "app")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest(http.MethodGet, "/api/logs/stream?ns=default&selector=app%3Dapi", nil).WithContext(ctx)
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.tokens["bob"]})
	w := &syncRecorder{header: http.Header{}}
	finished := make(chan struct{})
	go func() {
		defer close(finished)
		e.handler.ServeHTTP(w, req)
	}()
	waitFor := func(what string) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); !strings.Contains(w.String(), what); time.Sleep(5 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("no %q in %q", what, w.String())
			}
		}
	}
	waitFor("data: [api-1/app] fake logs")

	addPod(t, e, "api-2", map[string]string{"app": "api"}, "app")
	waitFor(`data: {"pods":["api-2/app"],"total":2}`)
	waitFor("data: [api-2/app] fake logs")

	// Deleted and recreated under the same name: a new incarnation.
	if err := e.cs.CoreV1().Pods("default").Delete(ctx, "api-1", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	waitFor(`"total":1}`)
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default", UID: "second",
		Labels: map[string]string{"app": "api"}}, Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "app"}}}}
	if _, err := e.cs.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	waitFor(`data: {"pods":["api-1/app"],"total":2}`)
	for deadline := time.Now().Add(5 * time.Second); strings.Count(w.String(), "data: [api-1/app] fake logs") < 2; time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the recreated pod was not followed: %q", w.String())
		}
	}
	if strings.Contains(w.String(), "event: end") {
		t.Fatalf("a live selector stream ended: %q", w.String())
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream outlived its client")
	}
}

// syncRecorder is a ResponseWriter whose body may be read while a stream
// is being written.
type syncRecorder struct {
	mu     sync.Mutex
	header http.Header
	body   bytes.Buffer
}

func (r *syncRecorder) Header() http.Header { return r.header }
func (r *syncRecorder) WriteHeader(int)     {}
func (r *syncRecorder) Flush()              {}
func (r *syncRecorder) Write(b []byte) (int, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.Write(b)
}
func (r *syncRecorder) String() string {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.body.String()
}

// failingReader returns its text, then err.
type failingReader struct {
	text string
	err  error
}

func (f *failingReader) Read(p []byte) (int, error) {
	if f.text == "" {
		return 0, f.err
	}
	n := copy(p, f.text)
	f.text = f.text[n:]
	return n, nil
}

func feed(pod string, open func(context.Context) (io.ReadCloser, error)) logFeed {
	return logFeed{source: kube.LogSource{Pod: pod, Container: "app"}, prefixed: true, open: open}
}

func TestStreamLogsReportsFailingLogs(t *testing.T) {
	w := &syncRecorder{header: http.Header{}}
	sse, err := startSSE(w, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	streamLogs(context.Background(), sse, []logFeed{
		feed("a", func(context.Context) (io.ReadCloser, error) {
			return nil, errors.New(`container "app" in pod "a" is waiting to start: ContainerCreating`)
		}),
	}, 1, time.Hour, nil, nil)
	streamLogs(context.Background(), sse, []logFeed{
		feed("b", func(context.Context) (io.ReadCloser, error) {
			return io.NopCloser(&failingReader{text: "one\ntwo\n", err: io.ErrUnexpectedEOF}), nil
		}),
	}, 1, time.Hour, nil, nil)

	want := []string{
		"event: pods\ndata: {\"pods\":[\"a/app\"],\"total\":1}",
		`event: error` + "\n" + `data: {"error":"[a/app] container \"app\" in pod \"a\" is waiting to start: ContainerCreating"}`,
		"event: end\ndata: ",
		"event: pods\ndata: {\"pods\":[\"b/app\"],\"total\":1}",
		"data: [b/app] one",
		"data: [b/app] two",
		"event: error\ndata: {\"error\":\"[b/app] unexpected EOF\"}",
		"event: end\ndata: ",
	}
	if got := events(w.String()); !slices.Equal(got, want) {
		t.Fatalf("events =\n%q\nwant\n%q", got, want)
	}
}

func TestStreamLogsHeartbeatUntilCancelled(t *testing.T) {
	w := &syncRecorder{header: http.Header{}}
	sse, err := startSSE(w, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	logR, logW := io.Pipe()
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamLogs(ctx, sse, []logFeed{feed("a", func(context.Context) (io.ReadCloser, error) {
			return logR, nil
		})}, 1, 10*time.Millisecond, nil, nil)
	}()
	if _, err := io.WriteString(logW, "hello\n"); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !strings.Contains(w.String(), ": ping\n\n") || !strings.Contains(w.String(), "data: [a/app] hello\n\n") {
		if time.Now().After(deadline) {
			t.Fatalf("no line and heartbeat in %q", w.String())
		}
		time.Sleep(5 * time.Millisecond)
	}
	cancel() // the client went away
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream outlived its client")
	}
	if _, err := logW.Write([]byte("more\n")); err == nil {
		t.Fatal("the log was left open")
	}
	if strings.Contains(w.String(), "event: end") {
		t.Fatal("a cancelled stream reported its end")
	}
}

func TestReadLines(t *testing.T) {
	const max = 8
	for _, tc := range []struct {
		name, in string
		want     []string
	}{
		{"lines", "a\nb\n", []string{"a", "b"}},
		{"no final line break", "a\nb", []string{"a", "b"}},
		{"blank lines", "a\n\n\nb\n", []string{"a", "", "", "b"}},
		{"carriage returns", "a\r\nb\rc\r\n", []string{"a", "bc"}},
		{"exactly max", "12345678\nx\n", []string{"12345678", "x"}},
		{"long line", "1234567890123456789\n", []string{"12345678", "90123456", "789"}},
		{"utf-8 kept whole", "abcdefgé€\n", []string{"abcdefg", "é€"}},
		{"empty", "", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var got []string
			err := readLines(strings.NewReader(tc.in), max, func(line string) bool {
				got = append(got, line)
				return true
			})
			if err != nil || !slices.Equal(got, tc.want) {
				t.Fatalf("readLines(%q) = %q, %v; want %q", tc.in, got, err, tc.want)
			}
		})
	}

	var got []string
	err := readLines(strings.NewReader("a\nb\nc\n"), max, func(line string) bool {
		got = append(got, line)
		return len(got) < 2
	})
	if err != nil || !slices.Equal(got, []string{"a", "b"}) {
		t.Fatalf("stopping early: %q, %v", got, err)
	}
	if err := readLines(&failingReader{text: "a\n", err: io.ErrClosedPipe}, max, func(string) bool { return true }); !errors.Is(err, io.ErrClosedPipe) {
		t.Fatalf("read error = %v", err)
	}
}

func TestSSEEventFraming(t *testing.T) {
	rec := httptest.NewRecorder()
	sse, err := startSSE(rec, time.Second)
	if err != nil || !rec.Flushed {
		t.Fatalf("startSSE: %v, flushed %v", err, rec.Flushed)
	}
	_ = sse.event("error", "first line\r\nsecond line")
	_ = sse.event("", "plain")
	_ = sse.comment("ping")
	want := "event: error\ndata: first line\ndata: second line\n\ndata: plain\n\n: ping\n\n"
	if got := rec.Body.String(); got != want {
		t.Fatalf("stream = %q, want %q", got, want)
	}
}

// With follow, the logs of pods that no longer match are closed and new
// matches are started, at most maxStreamPods at once.
func TestStreamLogsFollowClosesDeletedPods(t *testing.T) {
	w := &syncRecorder{header: http.Header{}}
	sse, err := startSSE(w, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	type pipe struct {
		r *io.PipeReader
		w *io.PipeWriter
	}
	var mu sync.Mutex
	pipes := map[string]pipe{}
	openFeed := func(pod string) logFeed {
		return feed(pod, func(context.Context) (io.ReadCloser, error) {
			r, pw := io.Pipe()
			mu.Lock()
			pipes[pod] = pipe{r, pw}
			mu.Unlock()
			go func() { _, _ = io.WriteString(pw, "hi from "+pod+"\n") }()
			return r, nil
		})
	}
	current := []string{"a", "b"}
	resolve := func(context.Context) ([]logFeed, int, error) {
		mu.Lock()
		defer mu.Unlock()
		var out []logFeed
		for _, p := range current {
			out = append(out, openFeed(p))
		}
		return out, len(current), nil
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	go func() {
		defer close(done)
		feeds, matched, _ := resolve(ctx)
		streamLogs(ctx, sse, feeds, matched, time.Hour, nil, &selectorFollow{every: 5 * time.Millisecond, resolve: resolve})
	}()
	waitFor := func(what string) {
		t.Helper()
		for deadline := time.Now().Add(5 * time.Second); !strings.Contains(w.String(), what); time.Sleep(5 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("no %q in %q", what, w.String())
			}
		}
	}
	waitFor("data: [b/app] hi from b")
	waitFor("data: [a/app] hi from a")

	mu.Lock()
	current = []string{"b", "c"} // a deleted, c new
	mu.Unlock()
	waitFor(`data: {"pods":["b/app","c/app"],"total":2}`)
	waitFor("data: [c/app] hi from c")
	mu.Lock()
	a := pipes["a"]
	mu.Unlock()
	if _, err := a.w.Write([]byte("late\n")); err == nil {
		t.Fatal("the log of the deleted pod was left open")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream outlived its client")
	}
	mu.Lock()
	defer mu.Unlock()
	for pod, p := range pipes {
		if _, err := p.w.Write([]byte("x\n")); err == nil {
			t.Errorf("the log of %s was left open", pod)
		}
	}
	if strings.Contains(w.String(), "event: end") {
		t.Fatal("a followed selector stream reported its end")
	}
}
