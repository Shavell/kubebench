package api

import (
	"net/http"
	"testing"

	"kubebench/internal/plugin"
	"kubebench/internal/plugins/argocd"
)

// A viewer's Argo CD mutations are refused by the router, before the
// plugin's Detect or handler sends anything to the cluster, and the
// refusals are audited.
func TestArgoCDMutationsNeedWrite(t *testing.T) {
	e := newTestEnv(t, nil, &argocd.Plugin{})
	requests := []struct{ method, path string }{
		{http.MethodPost, "/api/plugins/argocd/applications/argocd/web/sync"},
		{http.MethodPost, "/api/plugins/argocd/applications/argocd/web/rollback"},
		{http.MethodPost, "/api/plugins/argocd/applications/argocd/web/refresh"},
		{http.MethodDelete, "/api/plugins/argocd/applications/argocd/web?cascade=true"},
	}
	for _, r := range requests {
		expect(t, e.do(r.method, r.path, "bob", map[string]any{}), http.StatusForbidden, plugin.CodeForbiddenRole)
	}
	if actions := e.cs.Actions(); len(actions) != 0 {
		t.Fatalf("refused requests reached the cluster: %v", actions)
	}
	rows := e.audit()
	if len(rows) != len(requests) {
		t.Fatalf("audit rows = %+v", rows)
	}
	for _, row := range rows {
		if row.Status != http.StatusForbidden || row.Target != "application/argocd/web" {
			t.Fatalf("audit row = %+v", row)
		}
	}
}
