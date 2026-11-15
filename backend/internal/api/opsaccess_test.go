package api

import (
	"net/http"
	"testing"

	"kubebench/internal/plugin"
)

func TestOperationsNeedTheOperatorRole(t *testing.T) {
	e := newTestEnv(t, nil)
	for _, r := range opsRoutes {
		t.Run(r.method+" "+r.target, func(t *testing.T) {
			expect(t, e.do(r.method, r.target, "bob", map[string]any{}), http.StatusForbidden, plugin.CodeForbiddenRole)
			if row := e.lastAudit(); row.Username != "bob" || row.Status != http.StatusForbidden {
				t.Fatalf("a refused attempt is audited: %+v", row)
			}
		})
	}
	// Refused before any Kubernetes request, reads included.
	if n := len(e.cs.Actions()); n != 0 {
		t.Fatalf("a viewer's operations sent %d requests to the cluster: %v", n, e.cs.Actions())
	}
	if n := len(e.dynamicFake().Actions()); n != 0 {
		t.Fatalf("a viewer's operations sent %d dynamic requests", n)
	}
	// Reading the rollout history is fine.
	expect(t, e.do(http.MethodGet, "/api/deployments/history?ns=default&name=web", "bob", nil), http.StatusOK, "")
}

func TestOperationsOnAReadOnlyCluster(t *testing.T) {
	e := newTestEnv(t, nil)
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"readOnly": true}), http.StatusOK, "")
	for _, r := range opsRoutes {
		expect(t, e.do(r.method, r.target, "admin", map[string]any{}), http.StatusForbidden, plugin.CodeClusterReadOnly)
	}
	if m := e.mutations(); len(m) != 0 {
		t.Fatalf("a read-only cluster was changed: %v", m)
	}
}
