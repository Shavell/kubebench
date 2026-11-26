package kube

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"kubebench/internal/store"
	"kubebench/internal/watch"
)

func testRuntime(t *testing.T) (*Runtime, *fakediscovery.FakeDiscovery) {
	t.Helper()
	cs := fake.NewClientset()
	disc := cs.Discovery().(*fakediscovery.FakeDiscovery)
	disc.Resources = []*metav1.APIResourceList{
		{GroupVersion: "v1", APIResources: []metav1.APIResource{
			{Name: "pods", Kind: "Pod", Namespaced: true},
			{Name: "namespaces", Kind: "Namespace"},
		}},
		{GroupVersion: "apps/v1", APIResources: []metav1.APIResource{
			{Name: "deployments", Kind: "Deployment", Namespaced: true},
		}},
	}
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	return NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, dyn), disc
}

func TestResolveKind(t *testing.T) {
	rt, _ := testRuntime(t)
	for _, tc := range []struct {
		apiVersion, kind string
		want             schema.GroupVersionResource
		namespaced       bool
	}{
		{"apps/v1", "Deployment", schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}, true},
		{"v1", "Namespace", schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}, false},
		{"", "Pod", schema.GroupVersionResource{Version: "v1", Resource: "pods"}, true},
	} {
		gvr, namespaced, err := rt.ResolveKind(tc.apiVersion, tc.kind)
		if err != nil || gvr != tc.want || namespaced != tc.namespaced {
			t.Errorf("ResolveKind(%q, %q) = %v, %v, %v", tc.apiVersion, tc.kind, gvr, namespaced, err)
		}
	}
	if _, _, err := rt.ResolveKind("a/b/c", "Pod"); err == nil {
		t.Error("an invalid apiVersion must fail")
	}
	if _, _, err := rt.ResolveKind("apps/v1", "Nope"); !meta.IsNoMatchError(err) {
		t.Errorf("unknown kind: %v", err)
	}
}

func TestResolveRetriesWithFreshDiscovery(t *testing.T) {
	rt, disc := testRuntime(t)
	if _, _, err := rt.ResolveKind("apps/v1", "Deployment"); err != nil {
		t.Fatal(err) // fills the discovery cache
	}
	// A CRD installed after the cache was filled.
	disc.Resources = append(disc.Resources, &metav1.APIResourceList{
		GroupVersion: "argoproj.io/v1alpha1",
		APIResources: []metav1.APIResource{{Name: "applications", Kind: "Application", Namespaced: true}},
	})
	gvr, namespaced, err := rt.ResolveKind("argoproj.io/v1alpha1", "Application")
	if err != nil || gvr.Resource != "applications" || !namespaced {
		t.Fatalf("new kind after reset: %v %v %v", gvr, namespaced, err)
	}
	gvr, namespaced, err = rt.ResolveResource("argoproj.io", "applications")
	if err != nil || gvr.Version != "v1alpha1" || !namespaced {
		t.Fatalf("ResolveResource: %v %v %v", gvr, namespaced, err)
	}
	if gvr, namespaced, err := rt.ResolveResource("", "namespaces"); err != nil || gvr.Version != "v1" || namespaced {
		t.Fatalf("ResolveResource(core): %v %v %v", gvr, namespaced, err)
	}
}

// Lookups of unknown types refresh discovery at most once per
// mapperResetInterval, however many there are (e.g. 32 watch topics).
func TestUnknownLookupsShareOneDiscoveryRefresh(t *testing.T) {
	rt, _ := testRuntime(t)
	defer rt.Close()
	cs := rt.CS.(*fake.Clientset)
	if _, _, err := rt.ResolveResource("apps", "deployments"); err != nil {
		t.Fatal(err) // fills the discovery cache
	}
	cs.ClearActions()
	topics := make([]string, 32)
	for i := range topics {
		topics[i] = fmt.Sprintf("widgets.g%d.example.com", i)
	}
	sub, err := rt.Watch().Subscribe(1, topics)
	if err != nil {
		t.Fatal(err)
	}
	defer sub.Close()
	refresh := len(cs.Actions())
	if refresh == 0 {
		t.Fatal("the first miss did not refresh discovery")
	}
	if _, _, err := rt.ResolveResource("example.com", "gadgets"); !meta.IsNoMatchError(err) {
		t.Fatalf("unknown resource: %v", err)
	}
	if n := len(cs.Actions()); n != refresh {
		t.Fatalf("%d discovery requests for 33 misses, want the %d of one refresh", n, refresh)
	}
	// Once the interval passed, a miss refreshes again.
	rt.resetMu.Lock()
	rt.lastReset = time.Now().Add(-mapperResetInterval)
	rt.resetMu.Unlock()
	_, _, _ = rt.ResolveResource("example.com", "gadgets")
	if n := len(cs.Actions()); n != 2*refresh {
		t.Fatalf("%d discovery requests after the interval, want %d", n, 2*refresh)
	}
}

func TestWaitProbed(t *testing.T) {
	rt, _ := testRuntime(t)
	if err := rt.WaitProbed(context.Background()); err != nil {
		t.Fatalf("a test runtime counts as probed: %v", err)
	}
	pending := &Runtime{probed: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := pending.WaitProbed(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending probe: %v", err)
	}
	close(pending.probed)
	if err := pending.WaitProbed(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestWaitPromDiscovered(t *testing.T) {
	rt, _ := testRuntime(t)
	if err := rt.WaitPromDiscovered(context.Background()); err != nil || rt.Prom() != nil {
		t.Fatalf("a test runtime counts as discovered without Prometheus: %v", err)
	}
	pending := &Runtime{promDone: make(chan struct{})}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	if err := pending.WaitPromDiscovered(ctx); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("pending discovery: %v", err)
	}
	close(pending.promDone)
	if err := pending.WaitPromDiscovered(context.Background()); err != nil {
		t.Fatal(err)
	}
}

func TestRawGet(t *testing.T) {
	var gotPath, gotQuery string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath, gotQuery = r.URL.Path, r.URL.RawQuery
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Path == "/prefix/api/v1/big" {
			_, _ = w.Write([]byte(strings.Repeat("x", 64)))
			return
		}
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"kind":"Status","message":"forbidden"}`))
	}))
	defer api.Close()
	rt, _ := testRuntime(t)
	// A server URL with a path prefix (e.g. Rancher's cluster proxy) is kept.
	rt.Config = &rest.Config{Host: api.URL + "/prefix"}

	res, err := rt.RawGet(context.Background(), "api/v1/namespaces", url.Values{"limit": {"5"}}, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if gotPath != "/prefix/api/v1/namespaces" || gotQuery != "limit=5" {
		t.Fatalf("upstream request = %s?%s", gotPath, gotQuery)
	}
	if res.Status != http.StatusForbidden || res.ContentType != "application/json" || !strings.Contains(string(res.Body), "forbidden") {
		t.Fatalf("response = %+v", res)
	}
	if _, err := rt.RawGet(context.Background(), "api/v1/big", nil, 32); !errors.Is(err, ErrResponseTooLarge) {
		t.Fatalf("oversized response: %v", err)
	}
	if res, err := rt.RawGet(context.Background(), "api/v1/big", nil, 64); err != nil || len(res.Body) != 64 {
		t.Fatalf("a response of exactly the limit must pass: %v", err)
	}
}

func TestManagerRebuildsOnChangedRecord(t *testing.T) {
	builds := 0
	m := NewManagerWith(func(c store.Cluster) (*Runtime, error) {
		builds++
		rt, _ := testRuntime(t)
		rt.Cluster = c
		return rt, nil
	})
	cluster := store.Cluster{ID: 7, Name: "prod", Source: store.SourceKubeconfig}
	first, _ := m.Runtime(&cluster)
	if again, _ := m.Runtime(&cluster); again != first || builds != 1 {
		t.Fatalf("an unchanged record must reuse the runtime (builds=%d)", builds)
	}
	cluster.ReadOnly = true
	rebuilt, _ := m.Runtime(&cluster)
	if rebuilt == first || !rebuilt.Cluster.ReadOnly || builds != 2 {
		t.Fatalf("a changed record must rebuild the runtime (builds=%d)", builds)
	}
	m.Drop(cluster.ID)
	if _, _ = m.Runtime(&cluster); builds != 3 {
		t.Fatalf("a dropped runtime must be rebuilt (builds=%d)", builds)
	}
}

// A request that read a cluster's record before the cluster was deleted
// must not cache a new runtime for it: nothing would ever close it, and
// its sampler would keep using the removed cluster's credentials.
func TestManagerRefusesRemovedClusters(t *testing.T) {
	builds := 0
	m := NewManagerWith(func(c store.Cluster) (*Runtime, error) {
		builds++
		rt, _ := testRuntime(t)
		rt.Cluster = c
		return rt, nil
	})
	cluster := store.Cluster{ID: 7, Name: "prod", Source: store.SourceKubeconfig}
	if _, err := m.Runtime(&cluster); err != nil {
		t.Fatal(err)
	}
	stale := cluster
	m.Remove(cluster.ID)
	if rt, err := m.Runtime(&stale); !errors.Is(err, ErrClusterRemoved) || rt != nil {
		t.Fatalf("Runtime of a removed cluster = %v, %v", rt, err)
	}
	if m.Peek(&cluster) != nil || builds != 1 {
		t.Fatalf("a runtime was built again for the removed cluster (builds=%d)", builds)
	}
}

// Closing a runtime may take long (its watch hub waits for goroutines that
// may sit in a call to an unreachable cluster); other clusters' requests
// must not wait for it.
func TestManagerClosesRuntimesOutsideTheLock(t *testing.T) {
	m := NewManagerWith(func(c store.Cluster) (*Runtime, error) {
		rt, _ := testRuntime(t)
		rt.Cluster = c
		return rt, nil
	})
	slow := store.Cluster{ID: 1, Name: "unreachable", Source: store.SourceKubeconfig}
	rt, err := m.Runtime(&slow)
	if err != nil {
		t.Fatal(err)
	}
	// The first resolution fails; the retry hangs like discovery against
	// a black-holed API server, so Close waits for it.
	hanging, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	rt.watch.Close()
	rt.watch = watch.NewHub(rt.meta, rt.Dyn, func(string) (schema.GroupVersionResource, error) {
		if calls.Add(1) == 2 { // the retry
			close(hanging)
			<-release
		}
		return schema.GroupVersionResource{}, errors.New("connection timed out")
	}, watch.Options{Cluster: "unreachable", Retry: time.Millisecond, Logf: func(string, ...any) {}})
	defer close(release)
	if _, err := rt.watch.Subscribe(1, []string{"widgets.example.com"}); err != nil {
		t.Fatal(err)
	}
	<-hanging
	go m.Remove(slow.ID)
	// Wait until Close has begun (the hub refuses subscriptions), so that
	// Remove is past taking the manager's lock.
	for {
		if _, err := rt.watch.Subscribe(2, []string{"pods"}); errors.Is(err, watch.ErrClosed) {
			break
		}
		time.Sleep(time.Millisecond)
	}

	other := store.Cluster{ID: 2, Name: "fine", Source: store.SourceKubeconfig}
	got := make(chan error, 1)
	go func() {
		_, err := m.Runtime(&other)
		got <- err
	}()
	select {
	case err := <-got:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Runtime of another cluster waited for a runtime being closed")
	}
}
