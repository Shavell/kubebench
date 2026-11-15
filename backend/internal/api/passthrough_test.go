package api

import (
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"

	"kubebench/internal/plugin"
)

// fakeAPIServer answers like the Kubernetes API and records what reached it.
type fakeAPIServer struct {
	*httptest.Server
	mu   sync.Mutex
	hits []string
}

func newFakeAPIServer(t *testing.T) *fakeAPIServer {
	f := &fakeAPIServer{}
	f.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		f.mu.Lock()
		f.hits = append(f.hits, r.URL.RequestURI())
		f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/v1/namespaces", "/version", "/apis", "/api/v1/namespaces/secrets/pods", "/api/v1/namespaces/default/status":
			_, _ = w.Write([]byte(`{"kind":"List","items":[]}`))
		case "/api/v1/namespaces/default/pods/gone":
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"kind":"Status","message":"pods \"gone\" not found","code":404}`))
		case "/api/v1/nodes":
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"kind":"Status","message":"Unauthorized","code":401}`))
		default:
			w.WriteHeader(http.StatusTeapot)
		}
	}))
	t.Cleanup(f.Close)
	return f
}

func (f *fakeAPIServer) requests() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string(nil), f.hits...)
}

func TestPassthrough(t *testing.T) {
	e := newTestEnv(t, nil)
	upstream := newFakeAPIServer(t)
	e.apiServer = upstream.URL

	for _, tc := range []struct {
		target string
		status int
		code   string
	}{
		{"/api/k8s/api/v1/namespaces?limit=5", http.StatusOK, ""},
		{"/api/k8s/version", http.StatusOK, ""},
		{"/api/k8s/apis", http.StatusOK, ""},
		{"/api/k8s/api/v1/namespaces/secrets/pods", http.StatusOK, ""}, // a namespace named "secrets"
		{"/api/k8s/api/v1/namespaces/default/status", http.StatusOK, ""},
		{"/api/k8s/api/v1/namespaces/default/pods/gone", http.StatusNotFound, plugin.CodeKubernetes},
		// The cluster rejecting KubeBench's credential is not a KubeBench 401.
		{"/api/k8s/api/v1/nodes", http.StatusBadGateway, plugin.CodeKubernetes},

		{"/api/k8s/api/v1/secrets", http.StatusForbidden, plugin.CodeForbidden},
		{"/api/k8s/api/v1/namespaces/default/secrets", http.StatusForbidden, plugin.CodeForbidden},
		{"/api/k8s/api/v1/namespaces/default/secrets/db", http.StatusForbidden, plugin.CodeForbidden},
		{"/api/k8s/api/v1/namespaces/default/Secrets", http.StatusForbidden, plugin.CodeForbidden},
		{"/api/k8s/api/v1/namespaces/default/sec%72ets", http.StatusForbidden, plugin.CodeForbidden},
		{"/api/k8s/api/v1/namespaces/default%2Fsecrets", http.StatusForbidden, plugin.CodeForbidden},
		{"/api/k8s/api/v1/namespaces/default/pods/web-1/exec?command=sh", http.StatusForbidden, plugin.CodeForbidden},
		{"/api/k8s/api/v1/namespaces/default/services/web:80/proxy/admin", http.StatusForbidden, plugin.CodeForbidden},
		{"/api/k8s/api/v1/nodes/n1/proxy/metrics", http.StatusForbidden, plugin.CodeForbidden},

		{"/api/k8s/api/v1/pods?watch=true", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/k8s/api/v1/pods?watch=1", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/k8s/api/v1/pods?watch=f", http.StatusBadRequest, plugin.CodeBadRequest}, // the API server reads "f" as true
		{"/api/k8s/api/v1/pods?watch", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/k8s/api/v1/watch/namespaces/default/pods", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/k8s/api/v1/namespaces/default/pods/web-1/log?follow=true", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/k8s/healthz", http.StatusBadRequest, plugin.CodeBadRequest},
		{"/api/k8s/metrics", http.StatusBadRequest, plugin.CodeBadRequest},
	} {
		t.Run(tc.target, func(t *testing.T) {
			expect(t, e.do(http.MethodGet, tc.target, "bob", nil), tc.status, tc.code)
		})
	}

	got := upstream.requests()
	want := []string{
		"/api/v1/namespaces?limit=5", "/version", "/apis", "/api/v1/namespaces/secrets/pods",
		"/api/v1/namespaces/default/status", "/api/v1/namespaces/default/pods/gone", "/api/v1/nodes",
	}
	if len(got) != len(want) {
		t.Fatalf("upstream saw %v, want only %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("upstream request %d = %q, want %q", i, got[i], want[i])
		}
	}

	expect(t, e.do(http.MethodPost, "/api/k8s/api/v1/namespaces", "admin", nil), http.StatusMethodNotAllowed, plugin.CodeBadRequest)
	ok := e.do(http.MethodGet, "/api/k8s/api/v1/namespaces", "bob", nil)
	if ok.Header().Get("Content-Type") != "application/json" || ok.Body.String() != `{"kind":"List","items":[]}` {
		t.Fatalf("raw body = %q (%s)", ok.Body.String(), ok.Header().Get("Content-Type"))
	}
}

// Whatever an aggregated API or a proxy in front of the cluster answers,
// the passthrough serves data, never a page of KubeBench's origin, and it
// never follows a redirect with the stored credential.
func TestPassthroughServesOnlyData(t *testing.T) {
	var sinkHits atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		sinkHits.Add(1)
		_, _ = w.Write([]byte(`{"internal":"metadata"}`))
	}))
	t.Cleanup(sink.Close)
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/apis/evil.example.com/v1/page":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			_, _ = w.Write([]byte(`<script>alert(document.cookie)</script>`))
		case "/apis/evil.example.com/v1/image":
			w.Header().Set("Content-Type", "image/svg+xml")
			_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" onload="alert(1)"/>`))
		case "/apis/evil.example.com/v1/redirect":
			http.Redirect(w, r, sink.URL+"/latest/meta-data/iam", http.StatusFound)
		case "/api/v1/namespaces/default/pods/web-1/log":
			w.Header().Set("Content-Type", "text/plain")
			_, _ = w.Write([]byte("line 1\n"))
		case "/apis/metrics.k8s.io/v1beta1/nodes":
			w.Header().Set("Content-Type", "application/json;charset=utf-8")
			_, _ = w.Write([]byte(`{"kind":"NodeMetricsList","items":[]}`))
		}
	}))
	t.Cleanup(upstream.Close)
	e := newTestEnv(t, nil)
	e.apiServer = upstream.URL

	for path, want := range map[string]string{
		"/api/k8s/apis/evil.example.com/v1/page":            "application/octet-stream",
		"/api/k8s/apis/evil.example.com/v1/image":           "application/octet-stream",
		"/api/k8s/api/v1/namespaces/default/pods/web-1/log": "text/plain; charset=utf-8",
		"/api/k8s/apis/metrics.k8s.io/v1beta1/nodes":        "application/json",
	} {
		rec := e.do(http.MethodGet, path, "bob", nil)
		expect(t, rec, http.StatusOK, "")
		h := rec.Header()
		if h.Get("Content-Type") != want || h.Get("X-Content-Type-Options") != "nosniff" ||
			h.Get("Content-Security-Policy") != "sandbox; default-src 'none'" {
			t.Errorf("%s: headers %v, want Content-Type %q, nosniff and a sandbox CSP", path, h, want)
		}
	}

	rec := e.do(http.MethodGet, "/api/k8s/apis/evil.example.com/v1/redirect", "bob", nil)
	expect(t, rec, http.StatusBadGateway, plugin.CodeKubernetes)
	if rec.Header().Get("Location") != "" || sinkHits.Load() != 0 {
		t.Fatalf("the redirect was followed or passed on (sink hits %d, Location %q)", sinkHits.Load(), rec.Header().Get("Location"))
	}
}
