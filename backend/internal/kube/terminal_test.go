package kube

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"

	"kubebench/internal/store"
)

// An API server (or proxy) that refuses the WebSocket upgrade must get the
// SPDY attempt next, as with kubectl exec.
func TestTerminalExecutorFallsBackToSPDY(t *testing.T) {
	var mu sync.Mutex
	var attempts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		attempts = append(attempts, r.Method+" "+r.URL.Path+" "+r.Header.Get("Upgrade"))
		mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		_, _ = io.WriteString(w, `{"kind":"Status","apiVersion":"v1","status":"Failure",`+
			`"message":"pods \"web\" is forbidden: exec is not allowed","reason":"Forbidden","code":403}`)
	}))
	defer srv.Close()

	config := &rest.Config{Host: srv.URL}
	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		t.Fatal(err)
	}
	rt := NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()))
	rt.Config = config

	executor, err := rt.TerminalExecutor(ExecTarget{Namespace: "shop", Pod: "web", Container: "app", Command: []string{"sh"}})
	if err != nil {
		t.Fatal(err)
	}
	err = executor.StreamWithContext(context.Background(), remotecommand.StreamOptions{
		Stdin: strings.NewReader(""), Stdout: io.Discard, Tty: true,
	})
	if err == nil || !strings.Contains(err.Error(), "exec is not allowed") {
		t.Fatalf("error = %v, want the API server's reason", err)
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{
		"GET /api/v1/namespaces/shop/pods/web/exec websocket",
		"POST /api/v1/namespaces/shop/pods/web/exec SPDY/3.1",
	}
	if strings.Join(attempts, "\n") != strings.Join(want, "\n") {
		t.Fatalf("attempts =\n%s\nwant\n%s", strings.Join(attempts, "\n"), strings.Join(want, "\n"))
	}
}
