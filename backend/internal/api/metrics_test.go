package api

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	restclient "k8s.io/client-go/rest"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
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
