package watch

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

var (
	podsGVR        = schema.GroupVersionResource{Version: "v1", Resource: "pods"}
	deploymentsGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	namespacesGVR  = schema.GroupVersionResource{Version: "v1", Resource: "namespaces"}
)

// testResolver knows pods, deployments (also as "deployments.apps") and
// namespaces.
func testResolver(topic string) (schema.GroupVersionResource, error) {
	switch topic {
	case "pods":
		return podsGVR, nil
	case "deployments", "deployments.apps":
		return deploymentsGVR, nil
	case "namespaces":
		return namespacesGVR, nil
	}
	return schema.GroupVersionResource{}, &NotServedError{Topic: topic}
}

func object(gvr schema.GroupVersionResource, kind, namespace, name string) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetAPIVersion(gvr.GroupVersion().String())
	u.SetKind(kind)
	u.SetNamespace(namespace)
	u.SetName(name)
	return u
}

func newFakeDynamic(objects ...k8sruntime.Object) *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(k8sruntime.NewScheme(),
		map[schema.GroupVersionResource]string{
			podsGVR: "PodList", deploymentsGVR: "DeploymentList", namespacesGVR: "NamespaceList",
		}, objects...)
}

// logs collects a hub's log lines.
type logs struct {
	mu    sync.Mutex
	lines []string
}

func (l *logs) logf(format string, args ...any) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.lines = append(l.lines, fmt.Sprintf(format, args...))
}

func (l *logs) contains(s string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return slices.ContainsFunc(l.lines, func(line string) bool { return strings.Contains(line, s) })
}

func newTestHub(t *testing.T, dyn *dynamicfake.FakeDynamicClient, opts Options) (*Hub, *logs) {
	t.Helper()
	l := &logs{}
	opts.Cluster = "test"
	opts.Logf = l.logf
	h := NewHub(dyn, testResolver, opts)
	t.Cleanup(h.Close)
	return h, l
}

func subscribe(t *testing.T, h *Hub, topics ...string) *Subscription {
	t.Helper()
	sub, err := h.Subscribe(1, topics)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(sub.Close)
	return sub
}

// waitStatus collects statuses until topic reaches mode.
func waitStatus(t *testing.T, sub *Subscription, topic string, mode Mode) TopicStatus {
	t.Helper()
	deadline := time.After(5 * time.Second)
	for {
		for _, st := range sub.TakeStatuses() {
			if st.Topic == topic && st.Mode == mode {
				return st
			}
		}
		select {
		case <-sub.Notify():
		case <-deadline:
			t.Fatalf("%s never became %s", topic, mode)
		}
	}
}

// waitChanges collects changes until at least n were taken.
func waitChanges(t *testing.T, sub *Subscription, n int) []Change {
	t.Helper()
	deadline := time.After(5 * time.Second)
	var got []Change
	for {
		got = append(got, sub.TakeChanges()...)
		if len(got) >= n {
			return got
		}
		select {
		case <-sub.Notify():
		case <-deadline:
			t.Fatalf("got changes %v, want %d", got, n)
		}
	}
}

func TestChangesAfterSync(t *testing.T) {
	dyn := newFakeDynamic(object(podsGVR, "Pod", "default", "existing"))
	h, l := newTestHub(t, dyn, Options{})
	sub := subscribe(t, h, "pods")

	if st := sub.TakeStatuses(); len(st) != 1 || !reflect.DeepEqual(st[0], TopicStatus{Topic: "pods", Mode: ModeStarting}) {
		t.Fatalf("first statuses = %v", st)
	}
	waitStatus(t, sub, "pods", ModeLive)
	if sub.HasChanges() {
		t.Fatalf("objects of the initial list were reported: %v", sub.TakeChanges())
	}

	ctx := context.Background()
	if _, err := dyn.Resource(podsGVR).Namespace("shop").Create(ctx, object(podsGVR, "Pod", "shop", "web"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if err := dyn.Resource(podsGVR).Namespace("default").Delete(ctx, "existing", metav1.DeleteOptions{}); err != nil {
		t.Fatal(err)
	}
	got := waitChanges(t, sub, 2)
	want := []Change{{Topic: "pods", Namespace: "default"}, {Topic: "pods", Namespace: "shop"}}
	slices.SortFunc(got, func(a, b Change) int { return strings.Compare(a.Namespace, b.Namespace) })
	if !slices.Equal(got, want) {
		t.Fatalf("changes = %v, want %v", got, want)
	}
	if !l.contains("starting informer v1/pods") || !l.contains("informer v1/pods synced") {
		t.Fatalf("logs = %v", l.lines)
	}
}

func TestTopicsShareInformers(t *testing.T) {
	dyn := newFakeDynamic()
	h, _ := newTestHub(t, dyn, Options{})
	a := subscribe(t, h, "deployments", "deployments.apps")
	b := subscribe(t, h, "deployments")
	waitStatus(t, a, "deployments.apps", ModeLive)
	waitStatus(t, b, "deployments", ModeLive)

	informers, subscribers := h.Stats()
	if subscribers != 2 || len(informers) != 1 || informers[0].Subscribers != 2 || informers[0].Resource != "apps/v1/deployments" {
		t.Fatalf("stats = %+v, %d", informers, subscribers)
	}

	if _, err := dyn.Resource(deploymentsGVR).Namespace("x").Create(context.Background(),
		object(deploymentsGVR, "Deployment", "x", "web"), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := waitChanges(t, a, 2); len(got) != 2 {
		t.Fatalf("a: changes = %v", got)
	}
	if got := waitChanges(t, b, 1); !slices.Equal(got, []Change{{Topic: "deployments", Namespace: "x"}}) {
		t.Fatalf("b: changes = %v", got)
	}
}

func TestUnknownTopicIsPolled(t *testing.T) {
	h, _ := newTestHub(t, newFakeDynamic(), Options{})
	sub := subscribe(t, h, "widgets.example.com")
	st := sub.TakeStatuses()
	if len(st) != 1 || st[0].Mode != ModePolling || st[0].Reason != "the cluster does not serve widgets.example.com" {
		t.Fatalf("statuses = %v", st)
	}
	if informers, _ := h.Stats(); len(informers) != 0 {
		t.Fatalf("an informer started: %v", informers)
	}
}

func TestListDeniedIsPolled(t *testing.T) {
	dyn := newFakeDynamic()
	dyn.PrependReactor("list", "pods", func(k8stesting.Action) (bool, k8sruntime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("RBAC"))
	})
	h, _ := newTestHub(t, dyn, Options{})
	sub := subscribe(t, h, "pods")
	st := waitStatus(t, sub, "pods", ModePolling)
	if st.Reason != "the cluster credential may not list and watch pods" {
		t.Fatalf("reason = %q", st.Reason)
	}
}

func TestWatchDeniedIsPolled(t *testing.T) {
	dyn := newFakeDynamic()
	dyn.PrependWatchReactor("pods", func(k8stesting.Action) (bool, watch.Interface, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "pods"}, "", errors.New("RBAC"))
	})
	h, l := newTestHub(t, dyn, Options{})
	sub := subscribe(t, h, "pods")
	waitStatus(t, sub, "pods", ModePolling)
	// The informer gave up: it neither retries nor turns live again.
	deadline := time.Now().Add(5 * time.Second)
	for !l.contains("informer v1/pods denied") {
		if time.Now().After(deadline) {
			t.Fatalf("logs = %v", l.lines)
		}
		time.Sleep(10 * time.Millisecond)
	}
	if informers, _ := h.Stats(); len(informers) != 1 || informers[0].Mode != ModePolling {
		t.Fatalf("stats = %+v", informers)
	}
}

func TestIdleInformerStops(t *testing.T) {
	h, l := newTestHub(t, newFakeDynamic(), Options{Idle: 50 * time.Millisecond})
	first, err := h.Subscribe(1, []string{"pods"})
	if err != nil {
		t.Fatal(err)
	}
	waitStatus(t, first, "pods", ModeLive)
	first.Close()
	first.Close() // a second close is harmless

	// A subscriber arriving within the idle time keeps the informer.
	second := subscribe(t, h, "pods")
	if st := second.TakeStatuses(); len(st) != 1 || st[0].Mode != ModeLive {
		t.Fatalf("statuses = %v", st)
	}
	time.Sleep(100 * time.Millisecond)
	if informers, _ := h.Stats(); len(informers) != 1 {
		t.Fatalf("informer stopped while subscribed: %v", informers)
	}
	second.Close()
	if informers, _ := h.Stats(); len(informers) != 1 || informers[0].IdleSince == nil {
		t.Fatalf("stats while idle = %+v", informers)
	}

	deadline := time.Now().Add(5 * time.Second)
	for {
		if informers, _ := h.Stats(); len(informers) == 0 {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the idle informer did not stop")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !l.contains("stopped informer v1/pods (no subscribers for 50ms)") {
		t.Fatalf("logs = %v", l.lines)
	}
}

func TestCloseEndsSubscriptionsAndInformers(t *testing.T) {
	before := runtime.NumGoroutine()
	dyn := newFakeDynamic()
	h := NewHub(dyn, testResolver, Options{Logf: func(string, ...any) {}})
	sub, err := h.Subscribe(1, []string{"pods", "deployments", "namespaces"})
	if err != nil {
		t.Fatal(err)
	}
	waitAllLive(t, h, 3)
	h.Close()
	h.Close()

	select {
	case <-sub.Done():
	default:
		t.Fatal("Done is not closed")
	}
	sub.Close() // after the hub closed
	if _, err := h.Subscribe(1, []string{"pods"}); !errors.Is(err, ErrClosed) {
		t.Fatalf("Subscribe after Close: %v", err)
	}
	// Close waited for the informers; their goroutines are gone.
	assertNoLeak(t, before)
}

// waitAllLive waits until the hub runs n informers, all of them live.
func waitAllLive(t *testing.T, h *Hub, n int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for {
		informers, _ := h.Stats()
		live := 0
		for _, inf := range informers {
			if inf.Mode == ModeLive {
				live++
			}
		}
		if live == n {
			return
		}
		if time.Now().After(deadline) {
			t.Fatalf("informers = %+v, want %d live", informers, n)
		}
		time.Sleep(10 * time.Millisecond)
	}
}

// assertNoLeak fails when more goroutines run than before, once
// goroutines that are about to return had the time to.
func assertNoLeak(t *testing.T, before int) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for runtime.NumGoroutine() > before {
		if time.Now().After(deadline) {
			buf := make([]byte, 1<<20)
			t.Fatalf("%d goroutines, %d before:\n%s", runtime.NumGoroutine(), before, buf[:runtime.Stack(buf, true)])
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestLimits(t *testing.T) {
	h, _ := newTestHub(t, newFakeDynamic(), Options{MaxSubscribers: 1, MaxInformers: 1})
	sub := subscribe(t, h, "pods", "deployments")
	st := sub.TakeStatuses()
	if len(st) != 2 || st[0].Topic != "deployments" || st[0].Mode != ModePolling || st[1].Mode != ModeStarting {
		t.Fatalf("statuses = %v", st)
	}
	if _, err := h.Subscribe(1, []string{"pods"}); !errors.Is(err, ErrTooManySubscribers) {
		t.Fatalf("second subscription: %v", err)
	}
}

func TestPendingChangesCollapse(t *testing.T) {
	h := NewHub(newFakeDynamic(), testResolver, Options{})
	sub := newSubscription(h, 1)
	for i := range maxPendingChanges {
		sub.addChange(Change{Topic: "pods", Namespace: fmt.Sprintf("ns-%d", i)})
	}
	sub.addChange(Change{Topic: "deployments", Namespace: "a"})
	sub.addChange(Change{Topic: "pods", Namespace: "ns-0"}) // pending already
	got := sub.TakeChanges()
	want := []Change{{Topic: "deployments"}, {Topic: "pods"}}
	if !slices.Equal(got, want) {
		t.Fatalf("changes = %v, want %v", got, want)
	}
	if sub.HasChanges() {
		t.Fatal("changes left after a take")
	}
}

func TestValidTopic(t *testing.T) {
	for topic, want := range map[string]bool{
		"pods":                     true,
		"deployments.apps":         true,
		"applications.argoproj.io": true,
		"volumesnapshots.snapshot.storage.k8s.io": true,
		"":                       false,
		"Pods":                   false,
		"pods,":                  false,
		"pods..apps":             false,
		".pods":                  false,
		"pods/log":               false,
		strings.Repeat("a", 318): false,
	} {
		if got := ValidTopic(topic); got != want {
			t.Errorf("ValidTopic(%q) = %v", topic, got)
		}
	}
	if r, g := SplitTopic("volumesnapshots.snapshot.storage.k8s.io"); r != "volumesnapshots" || g != "snapshot.storage.k8s.io" {
		t.Errorf("SplitTopic = %q, %q", r, g)
	}
}

// A topic that failed to resolve for another reason than NotServedError
// (discovery failed) is resolved again and then watched.
func TestTransientResolveFailureIsRetried(t *testing.T) {
	var mu sync.Mutex
	fails := 1
	resolve := func(topic string) (schema.GroupVersionResource, error) {
		mu.Lock()
		defer mu.Unlock()
		if fails > 0 {
			fails--
			return schema.GroupVersionResource{}, errors.New("cannot resolve pods: connection refused")
		}
		return testResolver(topic)
	}
	h := NewHub(newFakeDynamic(), resolve, Options{Retry: 10 * time.Millisecond, Logf: func(string, ...any) {}})
	t.Cleanup(h.Close)
	sub := subscribe(t, h, "pods", "widgets.example.com")
	st := sub.TakeStatuses()
	if len(st) != 2 || st[0].Topic != "pods" || st[0].Mode != ModePolling || st[0].Reason != "cannot resolve pods: connection refused" {
		t.Fatalf("statuses = %v", st)
	}
	waitStatus(t, sub, "pods", ModeLive)
	if informers, _ := h.Stats(); len(informers) != 1 {
		t.Fatalf("informers = %v", informers)
	}
}

// The retry of a failed resolution ends with its subscription.
func TestResolveRetryEndsWithSubscription(t *testing.T) {
	before := runtime.NumGoroutine()
	failing := func(string) (schema.GroupVersionResource, error) {
		return schema.GroupVersionResource{}, errors.New("connection refused")
	}
	h := NewHub(newFakeDynamic(), failing, Options{Retry: 5 * time.Millisecond, Logf: func(string, ...any) {}})
	sub, err := h.Subscribe(1, []string{"pods"})
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(20 * time.Millisecond)
	sub.Close()
	assertNoLeak(t, before)
	h.Close()
}

// One owner watches at most MaxOwnerTypes resource types across its
// subscriptions; other owners are not limited by it.
func TestOwnerTypeLimit(t *testing.T) {
	h, _ := newTestHub(t, newFakeDynamic(), Options{MaxOwnerTypes: 2})
	first, err := h.Subscribe(1, []string{"pods", "deployments"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(first.Close)
	first.TakeStatuses()
	// A second stream of the same owner: a type it watches already is
	// fine, a third one is polled.
	second, err := h.Subscribe(1, []string{"deployments.apps", "namespaces"})
	if err != nil {
		t.Fatal(err)
	}
	st := second.TakeStatuses()
	if len(st) != 2 || st[0].Topic != "deployments.apps" || st[0].Mode == ModePolling ||
		st[1].Mode != ModePolling || st[1].Reason != "you watch too many resource types on this cluster at once" {
		t.Fatalf("statuses = %v", st)
	}
	other, err := h.Subscribe(2, []string{"namespaces"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(other.Close)
	if st := other.TakeStatuses(); len(st) != 1 || st[0].Mode == ModePolling {
		t.Fatalf("another owner: %v", st)
	}
	// Closing a stream frees its types for the owner.
	second.Close()
	first.Close()
	again, err := h.Subscribe(1, []string{"namespaces"})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(again.Close)
	if st := again.TakeStatuses(); len(st) != 1 || st[0].Mode == ModePolling {
		t.Fatalf("after close: %v", st)
	}
}
