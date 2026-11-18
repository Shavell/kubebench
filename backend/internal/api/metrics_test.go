package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	restclient "k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"
	metricsv1beta1 "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsfake "k8s.io/metrics/pkg/client/clientset/versioned/fake"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// proxyAnswer is the fake clientset's answer to a service proxy request.
type proxyAnswer struct {
	body []byte
	err  error
}

func (p proxyAnswer) DoRaw(context.Context) ([]byte, error) { return p.body, p.err }

func (p proxyAnswer) Stream(context.Context) (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(p.body)), p.err
}

// withMetricsServer gives the test cluster a working metrics API (one
// node, n1, using 1 of 4 cores and 2 of 8 Gi; pod default/web-1) and
// samples it once.
func (e *testEnv) withMetricsServer() {
	e.t.Helper()
	if _, err := e.cs.CoreV1().Nodes().Create(context.Background(), &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "n1"},
		Status: corev1.NodeStatus{Capacity: corev1.ResourceList{
			corev1.ResourceCPU: resource.MustParse("4"), corev1.ResourceMemory: resource.MustParse("8Gi"),
		}},
	}, metav1.CreateOptions{}); err != nil {
		e.t.Fatal(err)
	}
	usage := func(cpu, mem string) corev1.ResourceList {
		return corev1.ResourceList{corev1.ResourceCPU: resource.MustParse(cpu), corev1.ResourceMemory: resource.MustParse(mem)}
	}
	metrics := metricsfake.NewSimpleClientset()
	metrics.PrependReactor("list", "nodes", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &metricsv1beta1.NodeMetricsList{Items: []metricsv1beta1.NodeMetrics{
			{ObjectMeta: metav1.ObjectMeta{Name: "n1"}, Usage: usage("1", "2Gi")},
		}}, nil
	})
	metrics.PrependReactor("list", "pods", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, &metricsv1beta1.PodMetricsList{Items: []metricsv1beta1.PodMetrics{{
			ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "default"},
			Containers: []metricsv1beta1.ContainerMetrics{{Name: "app", Usage: usage("500m", "1Gi")}},
		}}}, nil
	})
	rt := e.runtime()
	rt.Metrics = metrics
	rt.SampleForTest(context.Background(), e.clock.Now())
}

// runtime is the test cluster's runtime, as requests get it.
func (e *testEnv) runtime() *kube.Runtime {
	e.t.Helper()
	cluster, err := e.store.GetCluster(e.cluster)
	if err != nil {
		e.t.Fatal(err)
	}
	rt, err := e.srv.manager.Runtime(cluster)
	if err != nil {
		e.t.Fatal(err)
	}
	return rt
}

// When a Prometheus that was found fails, its series are reported as
// unavailable; only source=auto falls back to metrics-server, and only for
// the metrics it has. Nothing ever answers network or disk with memory.
func TestMetricSeriesAreNeverSubstituted(t *testing.T) {
	e := newTestEnv(t, nil)
	var failing atomic.Bool
	e.cs.PrependProxyReactor("services", func(k8stesting.Action) (bool, restclient.ResponseWrapper, error) {
		if failing.Load() {
			return true, proxyAnswer{err: apierrors.NewServiceUnavailable(`no endpoints available for service "prometheus"`)}, nil
		}
		return true, proxyAnswer{body: []byte(`{"status":"success","data":{"result":[{"values":[[1763200000,"42"]]}]}}`)}, nil
	})
	e.runtime().SetPromForTest("monitoring", "prometheus", "9090")

	series := func(target string) []kube.MetricPoint {
		t.Helper()
		rec := e.do(http.MethodGet, target, "bob", nil)
		expect(t, rec, http.StatusOK, "")
		return decode[[]kube.MetricPoint](t, rec)
	}
	if points := series("/api/metrics/cluster?metric=network"); len(points) != 1 || points[0].Value != 42 {
		t.Fatalf("a working Prometheus serves the series: %+v", points)
	}

	failing.Store(true)
	for _, target := range []string{
		"/api/metrics/cluster?metric=network",
		"/api/metrics/cluster?metric=fs&source=auto",
		"/api/metrics/cluster?metric=cpu&source=prometheus",
		"/api/metrics/node?node=n1&metric=network",
		"/api/metrics/pod?ns=default&pod=web-1&metric=network",
	} {
		rec := e.do(http.MethodGet, target, "bob", nil)
		expect(t, rec, http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable)
		if msg := decode[plugin.ErrorBody](t, rec).Error; !strings.Contains(msg, "Prometheus monitoring/prometheus") {
			t.Errorf("%s: the reason must name the failing Prometheus, got %q", target, msg)
		}
	}
	// metrics-server has CPU and memory, so auto may use it for those.
	e.withMetricsServer()
	series("/api/metrics/cluster?metric=cpu")
	series("/api/metrics/pod?ns=default&pod=web-1&metric=memory&source=auto")
	series("/api/metrics/node?node=n1&metric=memory&source=metrics-server")

	expect(t, e.do(http.MethodGet, "/api/metrics/cluster?metric=network&source=metrics-server", "bob", nil),
		http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable)
	expect(t, e.do(http.MethodGet, "/api/metrics/pod?ns=default&pod=web-1&metric=fs", "bob", nil),
		http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodGet, "/api/metrics/cluster?metric=disk", "bob", nil),
		http.StatusBadRequest, plugin.CodeBadRequest)
}

// A cluster without a metrics API reports metrics-server unavailable with
// the reason, answers every metrics-server series with 503
// capability_unavailable, and leaves the overview's and the allocation's
// usage null instead of 0 (the counts it does know stay).
func TestMetricsWithoutMetricsAPI(t *testing.T) {
	e := newTestEnv(t, nil)
	rec := e.do(http.MethodGet, "/api/metrics/sources", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	var sources map[string]struct {
		Available   bool            `json:"available"`
		Unavailable *reason.Message `json:"unavailable"`
	}
	decodeInto(t, rec, &sources)
	if ms := sources["metricsServer"]; ms.Available || ms.Unavailable == nil || ms.Unavailable.Code != "metrics.api_missing" {
		t.Fatalf("metricsServer = %+v", ms)
	}
	if p := sources["prometheus"]; p.Available || p.Unavailable == nil || p.Unavailable.Code != "metrics.no_prometheus" {
		t.Fatalf("prometheus = %+v", p)
	}

	for _, target := range []string{
		"/api/metrics/cluster?metric=cpu",
		"/api/metrics/cluster?metric=memory&source=metrics-server",
		"/api/metrics/node?node=n1&metric=cpu",
		"/api/metrics/pod?ns=default&pod=web-1&metric=memory",
	} {
		rec := e.do(http.MethodGet, target, "bob", nil)
		expect(t, rec, http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable)
		if body := decode[plugin.ErrorBody](t, rec); body.Reason != "metrics.api_missing" {
			t.Errorf("%s: reason = %q", target, body.Reason)
		}
	}
	expect(t, e.do(http.MethodGet, "/api/metrics/cluster?metric=cpu&source=prometheus", "bob", nil),
		http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable)

	rec = e.do(http.MethodGet, "/api/metrics/overview", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	var overview map[string]any
	decodeInto(t, rec, &overview)
	for _, field := range []string{"cpuUsagePercent", "memoryUsagePercent"} {
		if v, present := overview[field]; !present || v != nil {
			t.Errorf("overview %s = %v (present %v), want null", field, v, present)
		}
	}
	if why, _ := overview["usageUnavailable"].(map[string]any); why["reason"] != "metrics.api_missing" {
		t.Errorf("overview usageUnavailable = %v", overview["usageUnavailable"])
	}
	if overview["podCount"] != float64(2) {
		t.Errorf("overview podCount = %v", overview["podCount"])
	}

	rec = e.do(http.MethodGet, "/api/metrics/allocation", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	var allocation struct {
		CPU              map[string]any  `json:"cpu"`
		Memory           map[string]any  `json:"memory"`
		UsageUnavailable *reason.Message `json:"usageUnavailable"`
	}
	decodeInto(t, rec, &allocation)
	if allocation.CPU["usage"] != nil || allocation.Memory["usage"] != nil || allocation.UsageUnavailable == nil {
		t.Errorf("allocation = %+v", allocation)
	}

	// OpenShift monitoring that cannot be used (here: KubeBench runs
	// outside the cluster and finds no thanos-querier Route) explains the
	// missing Prometheus, and a series neither source has.
	if _, err := e.cs.CoreV1().Services("openshift-monitoring").Create(context.Background(), &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "thanos-querier", Namespace: "openshift-monitoring"},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	e.runtime().DiscoverPromForTest(kube.Options{})
	rec = e.do(http.MethodGet, "/api/metrics/sources", "bob", nil)
	decodeInto(t, rec, &sources)
	if p := sources["prometheus"]; p.Available || p.Unavailable == nil || p.Unavailable.Code != "metrics.thanos_no_route" {
		t.Fatalf("prometheus = %+v", p)
	}
	for target, want := range map[string]string{
		"/api/metrics/cluster?metric=network":                       "metrics.thanos_no_route",
		"/api/metrics/cluster?metric=cpu&source=prometheus":         "metrics.thanos_no_route",
		"/api/metrics/cluster?metric=network&source=metrics-server": "metrics.needs_prometheus",
	} {
		rec := e.do(http.MethodGet, target, "bob", nil)
		expect(t, rec, http.StatusServiceUnavailable, plugin.CodeCapabilityUnavailable)
		if body := decode[plugin.ErrorBody](t, rec); body.Reason != want {
			t.Errorf("%s: reason = %q, want %s", target, body.Reason, want)
		}
	}
}

// With a working metrics API the same endpoints carry measured values.
func TestMetricsWithMetricsServer(t *testing.T) {
	e := newTestEnv(t, nil)
	e.withMetricsServer()
	rec := e.do(http.MethodGet, "/api/metrics/overview", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	overview := decode[kube.ClusterOverview](t, rec)
	if overview.CPUUsagePercent == nil || *overview.CPUUsagePercent != 25 ||
		overview.MemoryUsagePercent == nil || *overview.MemoryUsagePercent != 25 || overview.UsageUnavailable != nil {
		t.Fatalf("overview = %+v", overview)
	}
	rec = e.do(http.MethodGet, "/api/metrics/allocation", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	allocation := decode[kube.ClusterAllocation](t, rec)
	if allocation.CPU.Usage == nil || *allocation.CPU.Usage != 1 || allocation.Memory.Usage == nil || *allocation.Memory.Usage != 2 {
		t.Fatalf("allocation = %+v", allocation)
	}
	rec = e.do(http.MethodGet, "/api/metrics/sources", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	if !strings.Contains(rec.Body.String(), `"metricsServer":{"available":true,"location":null}`) {
		t.Fatalf("sources = %s", rec.Body)
	}
	rec = e.do(http.MethodGet, "/api/metrics/pod?ns=default&pod=web-1&metric=cpu", "bob", nil)
	expect(t, rec, http.StatusOK, "")
	if points := decode[[]kube.MetricPoint](t, rec); len(points) != 1 || points[0].Value != 12.5 {
		t.Fatalf("pod cpu = %+v", points)
	}
}

func decodeInto(t *testing.T, rec *httptest.ResponseRecorder, v any) {
	t.Helper()
	if err := json.Unmarshal(rec.Body.Bytes(), v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
}
