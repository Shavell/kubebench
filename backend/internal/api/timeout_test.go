package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"kubebench/internal/plugin"
)

// A request to a hung API server ends at the request timeout with 504,
// while long-running routes get no deadline.
func TestRequestTimeout(t *testing.T) {
	e := newTestEnv(t, map[string]string{"KUBEBENCH_REQUEST_TIMEOUT": "1s"})
	hung := make(chan struct{})
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-hung:
		case <-r.Context().Done():
		}
	}))
	t.Cleanup(api.Close)
	t.Cleanup(func() { close(hung) })
	cs, err := kubernetes.NewForConfig(&rest.Config{Host: api.URL})
	if err != nil {
		t.Fatal(err)
	}
	e.srv.handle("GET /api/test/hung", accessRead, func(w http.ResponseWriter, r *http.Request, c *call) {
		_, err := cs.CoreV1().Pods("").List(r.Context(), metav1.ListOptions{})
		plugin.WriteK8sError(w, err)
	})
	deadlines := map[string]bool{}
	probe := func(name string) handlerFunc {
		return func(w http.ResponseWriter, r *http.Request, c *call) {
			_, deadlines[name] = r.Context().Deadline()
			writeOK(w)
		}
	}
	e.srv.handle("GET /api/test/read", accessRead, probe("read"))
	e.srv.handle("POST /api/test/write", accessWrite, probe("write"))
	e.srv.handle("GET /api/test/stream", accessRead|longRunning, probe("stream"))
	e.srv.handle("GET /api/test/admin", accessAdmin, probe("admin"))

	start := time.Now()
	expect(t, e.do(http.MethodGet, "/api/test/hung", "bob", nil), http.StatusGatewayTimeout, plugin.CodeKubernetes)
	if d := time.Since(start); d < time.Second || d > 10*time.Second {
		t.Fatalf("the hung request took %v, want about the 1s timeout", d)
	}

	expect(t, e.do(http.MethodGet, "/api/test/read", "bob", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodPost, "/api/test/write", "alice", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/test/stream", "bob", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/test/admin", "admin", nil), http.StatusOK, "")
	want := map[string]bool{"read": true, "write": true, "stream": false, "admin": false}
	for name, bounded := range want {
		if deadlines[name] != bounded {
			t.Errorf("%s: deadline = %v, want %v", name, deadlines[name], bounded)
		}
	}
}

func TestRequestTimeoutDisabled(t *testing.T) {
	e := newTestEnv(t, map[string]string{"KUBEBENCH_REQUEST_TIMEOUT": "0"})
	var bounded bool
	e.srv.handle("GET /api/test/read", accessRead, func(w http.ResponseWriter, r *http.Request, c *call) {
		_, bounded = r.Context().Deadline()
		writeOK(w)
	})
	expect(t, e.do(http.MethodGet, "/api/test/read", "bob", nil), http.StatusOK, "")
	if bounded {
		t.Fatal("KUBEBENCH_REQUEST_TIMEOUT=0 must not set a deadline")
	}
}

func TestIsDeadline(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()
	for _, err := range []error{ctx.Err(), &rateLimitErr{}} {
		if !plugin.IsDeadline(err) {
			t.Errorf("IsDeadline(%v) = false", err)
		}
	}
	if plugin.IsDeadline(context.Canceled) || plugin.IsDeadline(nil) {
		t.Error("IsDeadline must not match cancellation or nil")
	}
}

// rateLimitErr is what client-go returns when its rate limiter would wait
// past the deadline.
type rateLimitErr struct{}

func (*rateLimitErr) Error() string {
	return "client rate limiter Wait returned an error: rate: Wait(n=1) would exceed context deadline"
}
