package kube

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
)

// client-go adds the stored credential to every request its transport
// sends, redirected ones included. None of the runtime's clients may follow
// a redirect, or a proxied Service (the Prometheus queries) or an
// aggregated API could send the credential to any host.
func TestClientsDoNotFollowRedirects(t *testing.T) {
	var leaked atomic.Int32
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "" {
			leaked.Add(1)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"kind":"List","items":[]}`))
	}))
	defer sink.Close()
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL+"/latest/meta-data", http.StatusFound)
	}))
	defer api.Close()
	config := &rest.Config{Host: api.URL, BearerToken: "stored-cluster-credential"}

	c, err := clientsFor(config)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	if _, err := c.cs.CoreV1().Services("monitoring").ProxyGet("http", "prometheus", "9090", "api/v1/query", nil).DoRaw(ctx); err == nil {
		t.Error("the typed client accepted a redirect")
	}
	if _, err := c.dyn.Resource(Catalog["configmaps"].GVR).List(ctx, metav1.ListOptions{}); err == nil {
		t.Error("the dynamic client accepted a redirect")
	}
	if _, err := c.meta.Resource(Catalog["configmaps"].GVR).List(ctx, metav1.ListOptions{}); err == nil {
		t.Error("the metadata client accepted a redirect")
	}
	if _, err := c.metrics.MetricsV1beta1().NodeMetricses().List(ctx, metav1.ListOptions{}); err == nil {
		t.Error("the metrics client accepted a redirect")
	}

	rt, _ := testRuntime(t)
	rt.Config = config
	res, err := rt.RawGet(ctx, "apis/evil.example.com/v1/x", nil, 1024)
	if err != nil || res.Status != http.StatusFound {
		t.Errorf("RawGet must return the redirect itself: %+v, %v", res, err)
	}
	if n := leaked.Load(); n != 0 {
		t.Fatalf("the credential reached the redirect target %d times", n)
	}
}
