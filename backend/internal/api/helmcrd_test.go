package api

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// A fresh runtime (after a restart or a cluster edit) is still probing
// when the first Helm request arrives: Helm waits for the probe instead of
// being refused as unavailable.
func TestHelmWaitsForTheCapabilityProbe(t *testing.T) {
	e := newTestEnv(t, nil)
	rt := e.runtime()
	finish := rt.ProbeForTest()
	go func() {
		time.Sleep(100 * time.Millisecond)
		finish(kube.Capabilities{Helm: true})
	}()
	req := httptest.NewRequest(http.MethodGet, "/api/helm/releases", nil)
	if err := requireHelm(req, rt); err != nil {
		t.Fatalf("Helm refused while the probe ran: %v", err)
	}

	// A probe that does not finish in time reports Helm as unavailable.
	rt.ProbeForTest()
	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	if err := requireHelm(req.WithContext(ctx), rt); !hasCode(err, plugin.CodeCapabilityUnavailable) {
		t.Fatalf("unfinished probe: %v", err)
	}
}
