package api

import (
	"context"
	"net/http"
	"strings"
	"testing"

	"kubebench/internal/plugin"
	"kubebench/internal/plugins/helm"
)

// The Helm plugin's mutations go through the router's gates: viewers are
// refused before the handler runs, repositories need an admin, dry runs
// need only read access, and every attempt is audited without the
// request body.
func TestHelmPluginGates(t *testing.T) {
	p := helm.New()
	if err := plugin.StartAll(context.Background(), t.TempDir(), []plugin.Plugin{p}); err != nil {
		t.Fatal(err)
	}
	e := newTestEnv(t, nil, p)
	install := map[string]any{
		"name": "web", "namespace": "shop",
		"chart": map[string]string{"uploadId": strings.Repeat("0", 32)},
	}

	for _, tc := range []struct {
		method, target string
		body           any
		auditTarget    string
	}{
		{http.MethodPost, "/api/plugins/helm/install", install, "release"},
		{http.MethodPost, "/api/plugins/helm/releases/shop/web/upgrade", map[string]any{}, "release/shop/web"},
		{http.MethodPost, "/api/plugins/helm/releases/shop/web/rollback", map[string]int{"revision": 1}, "release/shop/web"},
		{http.MethodDelete, "/api/plugins/helm/releases/shop/web?keepHistory=true", nil, "release/shop/web"},
		{http.MethodPost, "/api/plugins/helm/repos/update", nil, "repo"},
		{http.MethodPost, "/api/plugins/helm/charts/upload", nil, "chart"},
	} {
		expect(t, e.do(tc.method, tc.target, "bob", tc.body), http.StatusForbidden, plugin.CodeForbiddenRole)
		row := e.lastAudit()
		if row.Username != "bob" || row.Status != http.StatusForbidden || row.Target != tc.auditTarget {
			t.Fatalf("%s %s: audit row %+v", tc.method, tc.target, row)
		}
	}
	for _, user := range []string{"alice", "bob"} {
		expect(t, e.do(http.MethodPost, "/api/plugins/helm/repos", user, map[string]string{"name": "r", "url": "https://example.com"}),
			http.StatusForbidden, plugin.CodeAdminRequired)
		expect(t, e.do(http.MethodDelete, "/api/plugins/helm/repos/r", user, nil), http.StatusForbidden, plugin.CodeAdminRequired)
	}
	if m := e.mutations(); len(m) != 0 {
		t.Fatalf("refused requests reached the cluster: %v", m)
	}

	// A viewer's dry run reaches the handler (which finds no such upload)
	// and is not audited.
	rows := len(e.audit())
	expect(t, e.do(http.MethodPost, "/api/plugins/helm/install?dryRun=true", "bob", install), http.StatusNotFound, plugin.CodeNotFound)
	if len(e.audit()) != rows {
		t.Fatal("a dry run was audited")
	}

	// The handler names the release in the row; credentials never appear.
	expect(t, e.do(http.MethodPost, "/api/plugins/helm/install", "alice", install), http.StatusNotFound, plugin.CodeNotFound)
	if row := e.lastAudit(); row.Target != "release/shop/web" || !strings.Contains(row.Detail, "not_found") {
		t.Fatalf("install row %+v", row)
	}
	repo := map[string]string{"name": "private", "url": "ftp://example.com", "username": "u", "password": "hunter2-secret"}
	expect(t, e.do(http.MethodPost, "/api/plugins/helm/repos", "admin", repo), http.StatusBadRequest, plugin.CodeBadRequest)
	if row := e.lastAudit(); row.Target != "repo/private" || row.Status != http.StatusBadRequest {
		t.Fatalf("repo row %+v", row)
	}
	for _, row := range e.audit() {
		if strings.Contains(row.Detail, "hunter2") || strings.Contains(row.Target, "hunter2") {
			t.Fatalf("audit row with a password: %+v", row)
		}
	}

	// The old routes are gone.
	expect(t, e.do(http.MethodGet, "/api/helm/releases", "admin", nil), http.StatusNotFound, plugin.CodeNotFound)
}
