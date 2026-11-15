package api

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"runtime"
	"strings"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"

	"kubebench/internal/store"
	"kubebench/internal/watch"
)

var watchPodsGVR = schema.GroupVersionResource{Version: "v1", Resource: "pods"}

// watchEnv is a test env whose cluster runtime watches a dynamic fake
// serving pods, with short stream timings. It returns the fake and a
// server running the API.
func watchEnv(t *testing.T) (*testEnv, *dynamicfake.FakeDynamicClient, *httptest.Server) {
	t.Helper()
	e := newTestEnv(t, nil)
	e.srv.streams.watchTimings = watchTimings{heartbeat: 50 * time.Millisecond, debounce: 20 * time.Millisecond}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(k8sruntime.NewScheme(),
		map[schema.GroupVersionResource]string{watchPodsGVR: "PodList"})
	cluster, err := e.store.GetCluster(e.cluster)
	if err != nil {
		t.Fatal(err)
	}
	rt, err := e.srv.manager.Runtime(cluster)
	if err != nil {
		t.Fatal(err)
	}
	rt.SetWatchForTest(watch.NewHub(dyn, func(topic string) (schema.GroupVersionResource, error) {
		if topic == "pods" {
			return watchPodsGVR, nil
		}
		return schema.GroupVersionResource{}, &unknownTopic{topic}
	}, watch.Options{Logf: func(string, ...any) {}}))
	srv := httptest.NewServer(e.handler)
	t.Cleanup(srv.Close)
	t.Cleanup(rt.Close)
	return e, dyn, srv
}

type unknownTopic struct{ topic string }

func (u *unknownTopic) Error() string { return "the cluster does not serve " + u.topic }

// sseEvent is one event read from a stream.
type sseEvent struct{ name, data string }

// openWatch opens GET /api/watch as user and returns its events, closed
// when the stream ends.
func openWatch(t *testing.T, e *testEnv, srv *httptest.Server, user, topics string) <-chan sseEvent {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/watch?topics="+topics, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.tokens[user]})
	resp, err := srv.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" {
		t.Fatalf("status %d, Content-Type %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	events := make(chan sseEvent, 64)
	go func() {
		defer close(events)
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		var ev sseEvent
		for scanner.Scan() {
			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "event: "):
				ev.name = strings.TrimPrefix(line, "event: ")
			case strings.HasPrefix(line, "data: "):
				ev.data = strings.TrimPrefix(line, "data: ")
			case line == "" && ev.name != "":
				events <- ev
				ev = sseEvent{}
			}
		}
	}()
	return events
}

// nextEvent returns the next event named name, skipping others.
func nextEvent(t *testing.T, events <-chan sseEvent, name string) string {
	t.Helper()
	timeout := time.After(5 * time.Second)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				t.Fatalf("the stream ended before an %q event", name)
			}
			if ev.name == name {
				return ev.data
			}
		case <-timeout:
			t.Fatalf("no %q event", name)
		}
	}
}

func createPod(t *testing.T, dyn *dynamicfake.FakeDynamicClient, namespace, name string) {
	t.Helper()
	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetNamespace(namespace)
	pod.SetName(name)
	if _, err := dyn.Resource(watchPodsGVR).Namespace(namespace).Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
}

func TestWatchStreamsChanges(t *testing.T) {
	e, dyn, srv := watchEnv(t)
	events := openWatch(t, e, srv, "bob", "pods,widgets.example.com,pods")

	statuses := map[string]watch.TopicStatus{}
	for len(statuses) < 2 || statuses["pods"].Mode != watch.ModeLive {
		var st watch.TopicStatus
		if err := json.Unmarshal([]byte(nextEvent(t, events, "status")), &st); err != nil {
			t.Fatal(err)
		}
		statuses[st.Topic] = st
	}
	if st := statuses["widgets.example.com"]; st.Mode != watch.ModePolling || st.Reason != "the cluster does not serve widgets.example.com" {
		t.Fatalf("unknown topic status = %+v", st)
	}

	// A burst becomes one event per namespace.
	createPod(t, dyn, "shop", "a")
	createPod(t, dyn, "shop", "b")
	if got := nextEvent(t, events, "change"); got != `{"topic":"pods","namespace":"shop"}` {
		t.Fatalf("change = %s", got)
	}
	createPod(t, dyn, "blog", "c")
	if got := nextEvent(t, events, "change"); got != `{"topic":"pods","namespace":"blog"}` {
		t.Fatalf("change after the burst = %s", got)
	}

	rec := e.do(http.MethodGet, "/api/watch/status", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	status := decode[watchStatus](t, rec)
	if status.Subscribers != 1 || len(status.Informers) != 1 || status.Informers[0].Resource != "v1/pods" ||
		status.Informers[0].Mode != watch.ModeLive || status.Informers[0].Subscribers != 1 {
		t.Fatalf("watch status = %+v", status)
	}
}

func TestWatchRequests(t *testing.T) {
	e := newTestEnv(t, nil)
	expect(t, e.do(http.MethodGet, "/api/watch?topics=pods", "", nil), http.StatusUnauthorized, "unauthenticated")
	expect(t, e.do(http.MethodGet, "/api/watch", "bob", nil), http.StatusBadRequest, "bad_request")
	expect(t, e.do(http.MethodGet, "/api/watch?topics=Pods", "bob", nil), http.StatusBadRequest, "bad_request")
	if err := e.store.SetAssignments(e.users["bob"], nil); err != nil {
		t.Fatal(err)
	}
	expect(t, e.do(http.MethodGet, "/api/watch?topics=pods", "bob", nil), http.StatusConflict, "no_cluster")
}

func TestParseWatchTopics(t *testing.T) {
	got, err := parseWatchTopics(" pods, deployments.apps,,pods ")
	if err != nil || strings.Join(got, "|") != "pods|deployments.apps" {
		t.Fatalf("topics = %v, %v", got, err)
	}
	var distinct []string
	for i := range maxWatchTopics + 1 {
		distinct = append(distinct, fmt.Sprintf("t%d", i))
	}
	many := strings.Join(distinct, ",")
	for _, raw := range []string{"", " , ", "pods/log", many} {
		if _, err := parseWatchTopics(raw); !hasCode(err, "bad_request") {
			t.Errorf("parseWatchTopics(%q): %v", raw, err)
		}
	}
}

// A watch stream ends within a heartbeat once its user loses access, and
// when the cluster connection is rebuilt; its subscription goes with it.
func TestWatchEnds(t *testing.T) {
	cases := map[string]struct {
		end    func(e *testEnv) error
		reason string
	}{
		"logout": {func(e *testEnv) error { return e.store.DeleteSession(e.tokens["bob"]) }, "the session was closed"},
		"unassigned": {
			func(e *testEnv) error { return e.store.SetAssignments(e.users["bob"], nil) },
			"the session was closed: you no longer have access",
		},
		"rebuilt": {func(e *testEnv) error {
			name := "renamed"
			if err := e.store.UpdateCluster(e.cluster, store.ClusterUpdate{Name: &name}); err != nil {
				return err
			}
			// The next request builds the runtime anew and closes the old one.
			if rec := e.do(http.MethodGet, "/api/namespaces", "alice", nil); rec.Code != http.StatusOK {
				t.Errorf("namespaces: %d %s", rec.Code, rec.Body.String())
			}
			return nil
		}, "the cluster connection was rebuilt"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			e, _, srv := watchEnv(t)
			events := openWatch(t, e, srv, "bob", "pods")
			nextEvent(t, events, "status")
			if err := tc.end(e); err != nil {
				t.Fatal(err)
			}
			if got := nextEvent(t, events, "end"); !strings.Contains(got, tc.reason) {
				t.Fatalf("end = %q, want %q", got, tc.reason)
			}
		})
	}
}

// Closing streams releases their subscriptions and stream slots, and
// closing the runtime stops its informers: nothing keeps running.
func TestWatchLeavesNothingRunning(t *testing.T) {
	e, _, srv := watchEnv(t)
	before := runtime.NumGoroutine()
	for range 3 {
		ctx, cancel := context.WithCancel(context.Background())
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL+"/api/watch?topics=pods", nil)
		req.AddCookie(&http.Cookie{Name: sessionCookie, Value: e.tokens["bob"]})
		resp, err := srv.Client().Do(req)
		if err != nil {
			t.Fatal(err)
		}
		cancel()
		resp.Body.Close()
	}
	cluster, _ := e.store.GetCluster(e.cluster)
	rt, _ := e.srv.manager.Runtime(cluster)
	deadline := time.Now().Add(5 * time.Second)
	for {
		_, subscribers := rt.Watch().Stats()
		if subscribers == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("%d subscribers left after the streams closed", subscribers)
		}
		time.Sleep(10 * time.Millisecond)
	}
	e.srv.streamLimits.mu.Lock()
	open := len(e.srv.streamLimits.open)
	e.srv.streamLimits.mu.Unlock()
	if open != 0 {
		t.Fatalf("stream slots still taken: %d", open)
	}
	srv.CloseClientConnections()
	rt.Close()
	deadline = time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<20)
			t.Fatalf("%d goroutines, %d before:\n%s", runtime.NumGoroutine(), before, buf[:runtime.Stack(buf, true)])
		}
		time.Sleep(10 * time.Millisecond)
	}
}
