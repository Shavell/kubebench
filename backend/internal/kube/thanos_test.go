package kube

import (
	"context"
	"encoding/pem"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	"k8s.io/client-go/rest"

	"kubebench/internal/reason"
	"kubebench/internal/store"
)

const thanosToken = "sha256~test-token"

// fakeThanos is a Thanos Querier behind kube-rbac-proxy: it wants the
// bearer token and answers every query with one sample of 42.
type fakeThanos struct {
	srv      *httptest.Server
	status   atomic.Int32 // answer this status instead, when set
	lastHost atomic.Value // Host header of the latest request
}

func newFakeThanos(t *testing.T) *fakeThanos {
	t.Helper()
	f := &fakeThanos{}
	f.srv = httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.lastHost.Store(r.Host)
		if code := f.status.Load(); code != 0 {
			if code == http.StatusFound {
				http.Redirect(w, r, "https://elsewhere.example/", http.StatusFound)
				return
			}
			http.Error(w, "Forbidden (user=system:serviceaccount:kb:kubebench, verb=get, resource=namespaces)", int(code))
			return
		}
		if r.Header.Get("Authorization") != "Bearer "+thanosToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		if strings.HasSuffix(r.URL.Path, "/query_range") {
			_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"matrix","result":[{"metric":{},"values":[[1763467200,"42.04"]]}]}}`))
			return
		}
		_, _ = w.Write([]byte(`{"status":"success","data":{"resultType":"vector","result":[{"metric":{},"value":[1763467200,"42"]}]}}`))
	}))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeThanos) caPEM() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: f.srv.Certificate().Raw}))
}

func (f *fakeThanos) hostPort() string {
	u, _ := url.Parse(f.srv.URL)
	return u.Host
}

// openshiftCluster is a runtime on a cluster with OpenShift monitoring:
// the thanos-querier Service and a Route admitted under routeHost, plus
// the default ingress CA when ingressCA is set.
func openshiftCluster(t *testing.T, cluster store.Cluster, config *rest.Config, routeHost, ingressCA string) *Runtime {
	t.Helper()
	objects := []runtime.Object{&corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: thanosService, Namespace: monitoringNamespace},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "tenancy", Port: 9092}, {Name: thanosPortName, Port: 9091}}},
	}}
	if ingressCA != "" {
		objects = append(objects, &corev1.ConfigMap{
			ObjectMeta: metav1.ObjectMeta{Name: ingressCAConfigMap, Namespace: ingressCANamespace},
			Data:       map[string]string{"ca-bundle.crt": ingressCA},
		})
	}
	cs := fake.NewClientset(objects...)
	route := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "route.openshift.io/v1", "kind": "Route",
		"metadata": map[string]any{"name": thanosRoute, "namespace": monitoringNamespace},
		"spec":     map[string]any{"host": "ignored.example"},
		"status":   map[string]any{"ingress": []any{map[string]any{"host": routeHost}}},
	}}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{routeGVR: "RouteList"}, route)
	rt := NewRuntimeForTest(cluster, cs, dyn)
	rt.Config = config
	return rt
}

var kubeconfigCluster = store.Cluster{ID: 1, Name: "crc", Source: store.SourceKubeconfig}

func TestThanosThroughRoute(t *testing.T) {
	thanos := newFakeThanos(t)
	rt := openshiftCluster(t, kubeconfigCluster, &rest.Config{BearerToken: thanosToken}, thanos.hostPort(), thanos.caPEM())
	rt.DiscoverPromForTest(Options{})
	prom := rt.Prom()
	if prom == nil {
		t.Fatalf("no Prometheus: %v", rt.PromProblem())
	}
	if prom.Location() != "openshift-monitoring/thanos-querier" {
		t.Errorf("location = %q", prom.Location())
	}
	points, err := prom.ClusterSeries(context.Background(), "cpu", 15)
	if err != nil || len(points) != 1 || points[0].Value != 42 {
		t.Fatalf("cluster cpu = %v, %v", points, err)
	}

	// Without a metrics-server sample, the overview's use comes from Thanos.
	rt.Sampler.status = reason.New(reasonMetricsMissing, nil)
	m, err := rt.measureUsage(context.Background(), usage{cpu: 10, memMi: 1024})
	if err != nil || m.cpuPercent != 42 || m.cpuCores != 4.2 {
		t.Fatalf("usage = %+v, %v", m, err)
	}
}

// The Route's certificate must be trusted: without the ingress CA (and
// without explicit insecure mode) Thanos is not used.
func TestThanosVerifiesTLS(t *testing.T) {
	thanos := newFakeThanos(t)
	rt := openshiftCluster(t, kubeconfigCluster, &rest.Config{BearerToken: thanosToken}, thanos.hostPort(), "")
	rt.DiscoverPromForTest(Options{})
	if rt.Prom() != nil || codeOf(rt.PromProblem()) != "metrics.thanos_failed" {
		t.Fatalf("untrusted certificate: prom=%v problem=%v", rt.Prom(), rt.PromProblem())
	}
	rt.DiscoverPromForTest(Options{Monitoring: MonitoringOptions{InsecureSkipVerify: true}})
	if rt.Prom() == nil {
		t.Fatalf("insecure mode: %v", rt.PromProblem())
	}
}

// Address replaces where KubeBench connects; the Route's host still names
// the server.
func TestThanosAddressOverride(t *testing.T) {
	thanos := newFakeThanos(t)
	rt := openshiftCluster(t, kubeconfigCluster, &rest.Config{BearerToken: thanosToken}, "example.com", thanos.caPEM())
	rt.DiscoverPromForTest(Options{Monitoring: MonitoringOptions{Address: thanos.hostPort()}})
	if rt.Prom() == nil {
		t.Fatalf("no Prometheus: %v", rt.PromProblem())
	}
	if host := thanos.lastHost.Load(); host != "example.com" {
		t.Errorf("Host = %v, want the Route's host", host)
	}
}

// In a pod KubeBench uses the Service, not the Route.
func TestThanosInCluster(t *testing.T) {
	thanos := newFakeThanos(t)
	inCluster := store.Cluster{ID: 1, Name: "in-cluster", Source: store.SourceInCluster}
	tokenFile := t.TempDir() + "/token"
	if err := os.WriteFile(tokenFile, []byte(thanosToken), 0o600); err != nil {
		t.Fatal(err)
	}
	rt := openshiftCluster(t, inCluster, &rest.Config{BearerTokenFile: tokenFile}, "unused.example", "")

	old := serviceCAFile
	serviceCAFile = t.TempDir() + "/missing.crt"
	t.Cleanup(func() { serviceCAFile = old })
	rt.DiscoverPromForTest(Options{Monitoring: MonitoringOptions{Address: thanos.hostPort()}})
	if codeOf(rt.PromProblem()) != "metrics.thanos_no_service_ca" {
		t.Fatalf("without the service CA: %v", rt.PromProblem())
	}

	rt.DiscoverPromForTest(Options{Monitoring: MonitoringOptions{Address: thanos.hostPort(), InsecureSkipVerify: true}})
	if rt.Prom() == nil {
		t.Fatalf("no Prometheus: %v", rt.PromProblem())
	}
	if host := thanos.lastHost.Load(); host != "thanos-querier.openshift-monitoring.svc:9091" {
		t.Errorf("Host = %v, want the Service's web port", host)
	}
}

func TestThanosRefusals(t *testing.T) {
	thanos := newFakeThanos(t)
	discover := func(config *rest.Config) *Runtime {
		rt := openshiftCluster(t, kubeconfigCluster, config, thanos.hostPort(), thanos.caPEM())
		rt.DiscoverPromForTest(Options{})
		return rt
	}

	rt := discover(&rest.Config{TLSClientConfig: rest.TLSClientConfig{CertData: []byte("cert"), KeyData: []byte("key")}})
	if codeOf(rt.PromProblem()) != "metrics.thanos_no_token" {
		t.Errorf("client certificate credential: %v", rt.PromProblem())
	}

	thanos.status.Store(http.StatusForbidden)
	rt = discover(&rest.Config{BearerToken: thanosToken})
	if problem := rt.PromProblem(); codeOf(problem) != "metrics.thanos_forbidden" || !strings.Contains(problem.Error(), "cluster-monitoring-view") {
		t.Errorf("forbidden: %v", problem)
	}

	// A redirect is not followed: the token goes to the Thanos Querier only.
	thanos.status.Store(http.StatusFound)
	rt = discover(&rest.Config{BearerToken: thanosToken})
	if rt.Prom() != nil || codeOf(rt.PromProblem()) != "metrics.thanos_failed" {
		t.Errorf("redirect: prom=%v problem=%v", rt.Prom(), rt.PromProblem())
	}
}

// Without OpenShift monitoring there is nothing to explain.
func TestNoOpenShiftMonitoring(t *testing.T) {
	rt := NewRuntimeForTest(kubeconfigCluster, fake.NewClientset(), dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()))
	rt.DiscoverPromForTest(Options{})
	if rt.Prom() != nil || rt.PromProblem() != nil {
		t.Fatalf("prom=%v problem=%v", rt.Prom(), rt.PromProblem())
	}
}
