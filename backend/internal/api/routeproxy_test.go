package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	discoveryv1 "k8s.io/api/discovery/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/util/intstr"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube"
	"kubebench/internal/openshift"
	"kubebench/internal/plugin"
)

func (e *testEnv) createRoute(name string, spec map[string]any) {
	e.t.Helper()
	obj := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "route.openshift.io/v1", "kind": "Route",
		"metadata": map[string]any{"name": name, "namespace": "default"},
		"spec":     spec,
	}}
	if _, err := e.dyn.Resource(kube.Catalog["routes"].GVR).Namespace("default").Create(context.Background(), obj, metav1.CreateOptions{}); err != nil {
		e.t.Fatal(err)
	}
}

func TestRouteServicePort(t *testing.T) {
	svc := &corev1.Service{Spec: corev1.ServiceSpec{Ports: []corev1.ServicePort{
		{Name: "http", Port: 80, TargetPort: intstr.FromInt32(8080)},
		{Name: "metrics", Port: 9090, TargetPort: intstr.FromString("prom")},
		{Port: 443, TargetPort: intstr.FromInt32(8443)},
	}}}
	// The metrics port's named targetPort "prom" is container port 9100.
	endpoints := func() []discoveryv1.EndpointSlice {
		return []discoveryv1.EndpointSlice{{Ports: []discoveryv1.EndpointPort{
			{Name: ptr("http"), Port: ptr(int32(8080))}, {Name: ptr("metrics"), Port: ptr(int32(9100))},
		}}}
	}
	str, num := intstr.FromString, intstr.FromInt32
	tests := []struct {
		name   string
		target *intstr.IntOrString
		want   string
	}{
		{"no targetPort: the first port", nil, "http"},
		{"service port name", ptr(str("metrics")), "metrics"},
		{"named targetPort", ptr(str("prom")), "metrics"},
		{"number is the targetPort", ptr(num(8443)), "443"},
		{"number is an endpoint port behind a named targetPort", ptr(num(9100)), "metrics"},
		{"number falls back to the port", ptr(num(9090)), "metrics"},
		{"no match", ptr(str("grpc")), ""},
		{"no numeric match", ptr(num(7000)), ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := routeServicePort(svc, tt.target, endpoints); got != tt.want {
				t.Fatalf("routeServicePort = %q, want %q", got, tt.want)
			}
		})
	}
}

func ptr[T any](v T) *T { return &v }

// Opening a Route resolves it to its Service port, sends the Route's host
// and is audited once on the Route; viewers are refused.
func TestOpenRouteProxy(t *testing.T) {
	e, upstream := proxyEnv(t, nil)
	e.createRoute("web", map[string]any{
		"host": "web-default.apps.example.com", "path": "/app",
		"to":   map[string]any{"kind": "Service", "name": "web"},
		"port": map[string]any{"targetPort": "http"},
		"tls":  map[string]any{"termination": "edge"},
	})
	e.createRoute("other", map[string]any{
		"host": "other.apps.example.com",
		"to":   map[string]any{"kind": "Service", "name": "web"},
		"port": map[string]any{"targetPort": "grpc"},
	})

	expect(t, e.do(http.MethodPost, "/api/proxy/routes/default/web", "bob", nil), http.StatusForbidden, plugin.CodeForbiddenRole)
	expect(t, e.do(http.MethodPost, "/api/proxy/routes/default/nope", "alice", nil), http.StatusNotFound, plugin.CodeKubernetes)
	body := decode[plugin.ErrorBody](t, expectRec(t, e.do(http.MethodPost, "/api/proxy/routes/default/other", "alice", nil),
		http.StatusBadRequest, plugin.CodeBadRequest))
	if body.Reason != "proxy.route_no_port" || body.Params["port"] != "grpc" {
		t.Fatalf("unmatched port: %+v", body)
	}

	s := e.openProxy("alice", "/api/proxy/routes/default/web?scheme=http")
	if s.Target.Kind != "services" || s.Target.Name != "web" || s.Target.Port != "http" || s.Target.Host != "web-default.apps.example.com" {
		t.Fatalf("session = %+v", s)
	}
	e.browse(http.MethodGet, s.Base+"/app/", nil)
	if got := upstream.last(); got == nil || got.Host != "web-default.apps.example.com" || !strings.HasSuffix(got.URL.Path, "/app/") {
		t.Fatalf("upstream request = %+v", got)
	}

	rows := e.proxyAudits()
	if len(rows) != 4 {
		t.Fatalf("audit rows = %d, want one per attempt: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.Action != "route.proxy" || !strings.HasPrefix(row.Target, "route/default/") {
			t.Errorf("row = %+v", row)
		}
	}
	if d := rows[0].Detail; rows[0].Status != http.StatusOK || !strings.Contains(d, `"service":"web:http"`) ||
		!strings.Contains(d, `"host":"web-default.apps.example.com"`) {
		t.Fatalf("grant row = %+v", rows[0])
	}
}

// A Route whose numeric targetPort is the container port behind a named
// Service targetPort opens like the router serves it: by the endpoint
// port, read from the Service's EndpointSlices.
func TestOpenRouteProxyByEndpointPort(t *testing.T) {
	e, _ := proxyEnv(t, nil)
	ctx := context.Background()
	if _, err := e.cs.CoreV1().Services("default").Create(ctx, &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{Name: "kb", Namespace: "default"},
		Spec:       corev1.ServiceSpec{Ports: []corev1.ServicePort{{Name: "http", Port: 80, TargetPort: intstr.FromString("http")}}},
	}, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	for _, slice := range []discoveryv1.EndpointSlice{
		{ObjectMeta: metav1.ObjectMeta{Name: "kb-x1", Namespace: "default", Labels: map[string]string{discoveryv1.LabelServiceName: "kb"}},
			Ports: []discoveryv1.EndpointPort{{Name: ptr("http"), Port: ptr(int32(9876))}}},
		// Another Service's slice with the same port number does not count.
		{ObjectMeta: metav1.ObjectMeta{Name: "web-x1", Namespace: "default", Labels: map[string]string{discoveryv1.LabelServiceName: "web"}},
			Ports: []discoveryv1.EndpointPort{{Name: ptr("metrics"), Port: ptr(int32(9000))}}},
	} {
		if _, err := e.cs.DiscoveryV1().EndpointSlices("default").Create(ctx, &slice, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	e.createRoute("num", map[string]any{"host": "kb.apps.example.com", "to": map[string]any{"kind": "Service", "name": "kb"},
		"port": map[string]any{"targetPort": int64(9876)}})
	e.createRoute("foreign", map[string]any{"host": "kb2.apps.example.com", "to": map[string]any{"kind": "Service", "name": "kb"},
		"port": map[string]any{"targetPort": int64(9000)}})

	if s := e.openProxy("alice", "/api/proxy/routes/default/num"); s.Target.Name != "kb" || s.Target.Port != "http" {
		t.Fatalf("session = %+v", s)
	}
	body := decode[plugin.ErrorBody](t, expectRec(t, e.do(http.MethodPost, "/api/proxy/routes/default/foreign", "alice", nil),
		http.StatusBadRequest, plugin.CodeBadRequest))
	if body.Reason != "proxy.route_no_port" || body.Params["port"] != "9000" {
		t.Fatalf("a port of another Service's endpoints: %+v", body)
	}
}

// A Route to something other than a Service cannot be browsed.
func TestOpenRouteProxyNotService(t *testing.T) {
	e, _ := proxyEnv(t, nil)
	e.createRoute("odd", map[string]any{"host": "odd.example.com", "to": map[string]any{"kind": "Deployment", "name": "web"}})
	body := decode[plugin.ErrorBody](t, expectRec(t, e.do(http.MethodPost, "/api/proxy/routes/default/odd", "alice", nil),
		http.StatusBadRequest, plugin.CodeBadRequest))
	if body.Reason != "proxy.route_not_service" {
		t.Fatalf("error = %+v", body)
	}
}

// plainDynamicClient is the dynamic client of a plain Kubernetes test
// cluster: it lists nothing of OpenShift's (the SCC list answers 404, as
// the API server does for a group it does not serve).
func plainDynamicClient() *dynamicfake.FakeDynamicClient {
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{openshift.SCCGVR: "SecurityContextConstraintsList"})
	dyn.PrependReactor("list", openshift.SCCGVR.Resource, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(openshift.SCCGVR.GroupResource(), "")
	})
	return dyn
}
