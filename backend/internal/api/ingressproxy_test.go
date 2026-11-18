package api

import (
	"context"
	"net/http"
	"net/url"
	"reflect"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/plugin"
)

func ingressPath(path, service string, port networkingv1.ServiceBackendPort) networkingv1.HTTPIngressPath {
	prefix := networkingv1.PathTypePrefix
	return networkingv1.HTTPIngressPath{Path: path, PathType: &prefix, Backend: networkingv1.IngressBackend{
		Service: &networkingv1.IngressServiceBackend{Name: service, Port: port},
	}}
}

// shopIngress routes web.example.com/app to web:80 and /admin to web's
// port "http", any host
// of *.example.com to web:80, /static to a resource backend, and has web:80
// as its default backend.
func shopIngress() *networkingv1.Ingress {
	group := "storage.example.com"
	return &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{Name: "shop", Namespace: "default"},
		Spec: networkingv1.IngressSpec{
			DefaultBackend: &networkingv1.IngressBackend{Service: &networkingv1.IngressServiceBackend{
				Name: "web", Port: networkingv1.ServiceBackendPort{Number: 80},
			}},
			Rules: []networkingv1.IngressRule{
				{Host: "web.example.com", IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{
						ingressPath("/app", "web", networkingv1.ServiceBackendPort{Number: 80}),
						ingressPath("/admin", "web", networkingv1.ServiceBackendPort{Name: "http"}),
						{Path: "/static", Backend: networkingv1.IngressBackend{Resource: &corev1.TypedLocalObjectReference{
							APIGroup: &group, Kind: "Bucket", Name: "assets",
						}}},
					},
				}}},
				{Host: "*.example.com", IngressRuleValue: networkingv1.IngressRuleValue{HTTP: &networkingv1.HTTPIngressRuleValue{
					Paths: []networkingv1.HTTPIngressPath{ingressPath("/", "web", networkingv1.ServiceBackendPort{Number: 80})},
				}}},
			},
		},
	}
}

func TestIngressRoutes(t *testing.T) {
	got := ingressRoutes(shopIngress())
	want := []ingressRoute{
		{Host: "web.example.com", Path: "/app", Service: "web", Port: "80"},
		{Host: "web.example.com", Path: "/admin", Service: "web", Port: "http"},
		{Host: "*.example.com", Path: "/", Service: "web", Port: "80"},
		{Service: "web", Port: "80"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("routes = %+v", got)
	}
}

func ingressURL(host, path, scheme string) string {
	q := url.Values{"host": {host}, "path": {path}}
	if scheme != "" {
		q.Set("scheme", scheme)
	}
	return "/api/proxy/ingresses/default/shop?" + q.Encode()
}

// Opening an Ingress resolves the rule to its Service port; the grant sends
// the rule's host as the Host header and is audited once on the Ingress.
func TestOpenIngressProxy(t *testing.T) {
	e, upstream := proxyEnv(t, nil)
	if _, err := e.cs.NetworkingV1().Ingresses("default").Create(context.Background(), shopIngress(), metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}

	expect(t, e.do(http.MethodPost, ingressURL("web.example.com", "/app", ""), "bob", nil), http.StatusForbidden, plugin.CodeForbiddenRole)
	expect(t, e.do(http.MethodPost, "/api/proxy/ingresses/default/nope?host=a&path=/", "alice", nil), http.StatusNotFound, plugin.CodeKubernetes)
	for path, want := range map[string]string{"/other": "proxy.no_ingress_route", "/static": "proxy.no_ingress_route"} {
		body := decode[plugin.ErrorBody](t, expectRec(t, e.do(http.MethodPost, ingressURL("web.example.com", path, ""), "alice", nil),
			http.StatusBadRequest, plugin.CodeBadRequest))
		if body.Reason != want || body.Params["path"] != path {
			t.Errorf("%s: error = %+v, want reason %s", path, body, want)
		}
	}
	body := decode[plugin.ErrorBody](t, expectRec(t, e.do(http.MethodPost, ingressURL("web.example.com", "/app", "ftp"), "alice", nil),
		http.StatusBadRequest, plugin.CodeBadRequest))
	if body.Reason != "proxy.target_scheme" {
		t.Errorf("ftp scheme: error = %+v", body)
	}

	s := e.openProxy("alice", ingressURL("web.example.com", "/app", ""))
	if s.Target.Kind != "services" || s.Target.Name != "web" || s.Target.Port != "80" || s.Target.Host != "web.example.com" || s.Target.Scheme != "http" {
		t.Fatalf("session = %+v", s)
	}
	e.browse(http.MethodGet, s.Base+"/app/", nil)
	got := upstream.last()
	if got == nil || got.Host != "web.example.com" || got.URL.Path != webProxyPath+"/app/" {
		t.Fatalf("upstream request = %+v", got)
	}
	// A redirect the backend builds from the Host it got stays in the grant.
	rec := e.browse(http.MethodGet, s.Base+"/redir", nil)
	if loc := rec.Header().Get("Location"); loc != s.Base+"/login" {
		t.Fatalf("redirect = %q", loc)
	}

	if named := e.openProxy("alice", ingressURL("web.example.com", "/admin", "")); named.Target.Port != "http" {
		t.Fatalf("named port session = %+v", named)
	}

	// A wildcard rule names no host: the request keeps the API server's.
	wild := e.openProxy("alice", ingressURL("*.example.com", "/", ""))
	if wild.Target.Host != "" || wild.Target.Port != "80" {
		t.Fatalf("wildcard session = %+v", wild)
	}
	e.browse(http.MethodGet, wild.Base+"/", nil)
	if host := upstream.last().Host; host == "web.example.com" || host == "" {
		t.Fatalf("wildcard Host = %q", host)
	}
	if def := e.openProxy("admin", ingressURL("", "", "https")); def.Target.Host != "" || def.Target.Scheme != "https" {
		t.Fatalf("default backend session = %+v", def)
	}

	rows := e.proxyAudits()
	if len(rows) != 9 {
		t.Fatalf("audit rows = %d, want one per attempt: %+v", len(rows), rows)
	}
	for _, row := range rows {
		if row.Action != "ingress.proxy" || row.Target != "ingress/default/shop" && row.Target != "ingress/default/nope" {
			t.Errorf("row = %+v", row)
		}
	}
	// Newest first: the default backend, the wildcard, /admin, then /app.
	if d := rows[3].Detail; rows[3].Status != http.StatusOK || !strings.Contains(d, `"host":"web.example.com"`) ||
		!strings.Contains(d, `"path":"/app"`) || !strings.Contains(d, `"service":"web:80"`) {
		t.Fatalf("grant row = %+v", rows[3])
	}
}
