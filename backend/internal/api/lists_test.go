package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"testing"

	"kubebench/internal/plugin"
	"kubebench/internal/watch"
)

// The list routes take limit: the body stays a JSON array of at most
// limit objects, and a cut list says how many were left out.
func TestListLimit(t *testing.T) {
	e := newTestEnv(t, nil)
	for _, route := range []string{"/api/pods?", "/api/nodes?", "/api/namespaces?", "/api/deployments?", "/api/services?"} {
		rec := e.do(http.MethodGet, route, "bob", nil)
		expect(t, rec, http.StatusOK, "")
		total := len(decode[[]map[string]any](t, rec))
		if total == 0 && route == "/api/pods?" {
			t.Fatal("the test cluster has no pods")
		}
		if h := rec.Header().Get(headerListRemaining); h != "" {
			t.Fatalf("%s without limit: %s %q", route, headerListRemaining, h)
		}
		rec = e.do(http.MethodGet, route+"limit=1", "bob", nil)
		expect(t, rec, http.StatusOK, "")
		if got := len(decode[[]map[string]any](t, rec)); got != min(total, 1) {
			t.Fatalf("%slimit=1: %d objects", route, got)
		}
		want := ""
		if total > 1 {
			want = strconv.Itoa(total - 1)
		}
		if h := rec.Header().Get(headerListRemaining); h != want {
			t.Fatalf("%slimit=1: %s %q, want %q", route, headerListRemaining, h, want)
		}
		for _, bad := range []string{"0", "-1", "x"} {
			expect(t, e.do(http.MethodGet, route+"limit="+bad, "bob", nil), http.StatusBadRequest, plugin.CodeBadRequest)
		}
	}
}

// GET /api/count answers from the live informer's cache.
func TestCountFromLiveInformer(t *testing.T) {
	e, meta, srv := watchEnv(t)
	createPod(t, meta, "shop", "a")
	createPod(t, meta, "shop", "b")
	createPod(t, meta, "blog", "c")
	events := openWatch(t, e, srv, "bob", "pods")
	for {
		var st watch.TopicStatus
		if err := json.Unmarshal([]byte(nextEvent(t, events, "status")), &st); err != nil {
			t.Fatal(err)
		}
		if st.Mode == watch.ModeLive {
			break
		}
	}
	for query, want := range map[string]int{"topic=pods": 3, "topic=pods&ns=shop": 2, "topic=pods&ns=none": 0} {
		rec := e.do(http.MethodGet, "/api/count?"+query, "bob", nil)
		expect(t, rec, http.StatusOK, "")
		if got := decode[map[string]int](t, rec)["count"]; got != want {
			t.Errorf("%s: count %d, want %d", query, got, want)
		}
	}
	expect(t, e.do(http.MethodGet, "/api/count", "bob", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodGet, "/api/count?topic=Pods!", "bob", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodGet, "/api/count?topic=pods", "", nil), http.StatusUnauthorized, plugin.CodeUnauthenticated)
}
