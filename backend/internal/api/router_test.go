package api

import (
	"net/http"
	"testing"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

func TestPermissionMatrix(t *testing.T) {
	e := newTestEnv(t, nil)
	for _, tc := range []struct {
		user, method, target string
		status               int
		code                 string
	}{
		// Reads: any role on the cluster.
		{"bob", http.MethodGet, "/api/pods?ns=default", http.StatusOK, ""},
		{"alice", http.MethodGet, "/api/deployments", http.StatusOK, ""},
		{"admin", http.MethodGet, "/api/namespaces", http.StatusOK, ""},
		// Writes and exec: operator or admin.
		{"bob", http.MethodDelete, "/api/pods?ns=default&name=web-1", http.StatusForbidden, plugin.CodeForbiddenRole},
		{"bob", http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=5", http.StatusForbidden, plugin.CodeForbiddenRole},
		{"bob", http.MethodPost, "/api/deployments/restart?ns=default&name=web", http.StatusForbidden, plugin.CodeForbiddenRole},
		{"bob", http.MethodPost, "/api/exec?ns=default&pod=web-1&cmd=id", http.StatusForbidden, plugin.CodeForbiddenRole},
		{"bob", http.MethodPost, "/api/helm/rollback?ns=default&name=web&revision=1", http.StatusForbidden, plugin.CodeForbiddenRole},
		// Admin routes: global admins only, whatever the cluster role.
		{"alice", http.MethodGet, "/api/users", http.StatusForbidden, plugin.CodeAdminRequired},
		{"bob", http.MethodGet, "/api/audit", http.StatusForbidden, plugin.CodeAdminRequired},
		{"alice", http.MethodPatch, "/api/clusters/1", http.StatusForbidden, plugin.CodeAdminRequired},
		{"alice", http.MethodPut, "/api/plugins/argocd", http.StatusForbidden, plugin.CodeAdminRequired},
		{"admin", http.MethodGet, "/api/users", http.StatusOK, ""},
		// Sessions.
		{"", http.MethodGet, "/api/pods", http.StatusUnauthorized, plugin.CodeUnauthenticated},
		{"", http.MethodGet, "/api/info", http.StatusUnauthorized, plugin.CodeUnauthenticated},
	} {
		t.Run(tc.user+" "+tc.method+" "+tc.target, func(t *testing.T) {
			expect(t, e.do(tc.method, tc.target, tc.user, nil), tc.status, tc.code)
		})
	}
	// The denials above were decided before any Kubernetes request.
	if m := e.mutations(); len(m) != 0 {
		t.Fatalf("denied requests reached the cluster: %v", m)
	}

	expect(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=5", "alice", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodDelete, "/api/pods?ns=default&name=web-1", "alice", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodPost, "/api/deployments/restart?ns=default&name=web", "admin", nil), http.StatusOK, "")
	if got := e.replicas(); got != 5 {
		t.Fatalf("operator scale: replicas = %d", got)
	}
	if n := len(e.mutations()); n != 3 {
		t.Fatalf("allowed mutations reached the cluster %d times, want 3", n)
	}
}

func TestReadOnlyClusterRefusesEveryone(t *testing.T) {
	e := newTestEnv(t, nil)
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"readOnly": true}), http.StatusOK, "")

	for _, user := range []string{"admin", "alice", "bob"} {
		expect(t, e.do(http.MethodDelete, "/api/pods?ns=default&name=web-1", user, nil), http.StatusForbidden, plugin.CodeClusterReadOnly)
		expect(t, e.do(http.MethodPost, "/api/deployments/scale?ns=default&name=web&replicas=0", user, nil), http.StatusForbidden, plugin.CodeClusterReadOnly)
		expect(t, e.do(http.MethodPost, "/api/exec?ns=default&pod=web-1&cmd=id", user, nil), http.StatusForbidden, plugin.CodeClusterReadOnly)
		expect(t, e.do(http.MethodGet, "/api/pods", user, nil), http.StatusOK, "")
	}
	if m := e.mutations(); len(m) != 0 || e.replicas() != 2 {
		t.Fatalf("a read-only cluster was changed: %v", m)
	}
	// Admin routes manage KubeBench itself and stay available.
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"readOnly": false}), http.StatusOK, "")
	expect(t, e.do(http.MethodDelete, "/api/pods?ns=default&name=web-1", "alice", nil), http.StatusOK, "")
}

func TestPasswordChangeRequiredGating(t *testing.T) {
	e := newTestEnv(t, nil)
	if err := e.store.SetMustChangePassword(e.users["alice"], true); err != nil {
		t.Fatal(err)
	}
	for _, target := range []string{"/api/pods", "/api/nodes", "/api/k8s/api/v1/namespaces"} {
		expect(t, e.do(http.MethodGet, target, "alice", nil), http.StatusForbidden, plugin.CodePasswordChangeRequired)
	}
	expect(t, e.do(http.MethodDelete, "/api/pods?ns=default&name=web-1", "alice", nil), http.StatusForbidden, plugin.CodePasswordChangeRequired)
	// /api/auth/* and /api/info stay open, so the app can ask for a new password.
	info := decode[map[string]any](t, e.do(http.MethodGet, "/api/info", "alice", nil))
	if info["mustChangePassword"] != true {
		t.Fatalf("info = %v", info)
	}
	expect(t, e.do(http.MethodGet, "/api/auth/me", "alice", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodPost, "/api/auth/password", "alice",
		map[string]string{"currentPassword": testPassword, "newPassword": "a-brand-new-one"}), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/pods", "alice", nil), http.StatusOK, "")

	// Admins are gated too.
	if err := e.store.SetMustChangePassword(e.users["admin"], true); err != nil {
		t.Fatal(err)
	}
	expect(t, e.do(http.MethodGet, "/api/users", "admin", nil), http.StatusForbidden, plugin.CodePasswordChangeRequired)
}

func TestSessionErrors(t *testing.T) {
	e := newTestEnv(t, nil)
	e.tokens["ghost"] = "no-such-session"
	expect(t, e.do(http.MethodGet, "/api/pods", "ghost", nil), http.StatusUnauthorized, plugin.CodeSessionExpired)
	expect(t, e.do(http.MethodPost, "/api/auth/logout", "bob", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/pods", "bob", nil), http.StatusUnauthorized, plugin.CodeSessionExpired)
}

func TestActiveClusterProblems(t *testing.T) {
	e := newTestEnv(t, nil)
	// Assignment revoked after activation: requests refuse rather than
	// switching to another cluster behind the UI's back.
	if err := e.store.SetAssignments(e.users["alice"], nil); err != nil {
		t.Fatal(err)
	}
	expect(t, e.do(http.MethodGet, "/api/pods", "alice", nil), http.StatusConflict, plugin.CodeNoCluster)
	expect(t, e.do(http.MethodDelete, "/api/pods?ns=default&name=web-1", "alice", nil), http.StatusConflict, plugin.CodeNoCluster)

	e.addUser("carol", store.RoleUser, "")
	if err := e.store.SetAssignments(e.users["carol"], nil); err != nil {
		t.Fatal(err)
	}
	expect(t, e.do(http.MethodGet, "/api/pods", "carol", nil), http.StatusConflict, plugin.CodeNoCluster)
	if len(e.mutations()) != 0 {
		t.Fatal("a request without a usable cluster reached Kubernetes")
	}
}

func TestUnknownAPIRoutesAnswerJSON(t *testing.T) {
	e := newTestEnv(t, nil)
	expect(t, e.do(http.MethodGet, "/api/nope", "admin", nil), http.StatusNotFound, plugin.CodeNotFound)
	rec := e.do(http.MethodPut, "/api/pods", "admin", nil)
	expect(t, rec, http.StatusMethodNotAllowed, plugin.CodeBadRequest)
	if allow := rec.Header().Get("Allow"); allow != "GET, DELETE" {
		t.Fatalf("Allow = %q", allow)
	}
}

func TestSPAServesOtherPaths(t *testing.T) {
	e := newTestEnv(t, nil)
	spa := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { _, _ = w.Write([]byte("app")) })
	e.handler = e.srv.Handler(spa)
	if rec := e.do(http.MethodGet, "/workloads/pods", "", nil); rec.Code != http.StatusOK || rec.Body.String() != "app" {
		t.Fatalf("SPA path: %d %s", rec.Code, rec.Body.String())
	}
	expect(t, e.do(http.MethodGet, "/api/nope", "admin", nil), http.StatusNotFound, plugin.CodeNotFound)
	expect(t, e.do(http.MethodGet, "/healthz", "", nil), http.StatusOK, "")
}
