package api

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

func findUser(t *testing.T, e *testEnv, name string) userView {
	t.Helper()
	for _, u := range decode[[]userView](t, e.do(http.MethodGet, "/api/users", "admin", nil)) {
		if u.Username == name {
			return u
		}
	}
	t.Fatalf("user %s not listed", name)
	return userView{}
}

func TestUserListShape(t *testing.T) {
	e := newTestEnv(t, nil)
	alice := findUser(t, e, "alice")
	if alice.Role != "user" || len(alice.Clusters) != 1 || alice.Clusters[0] != "test" {
		t.Fatalf("alice = %+v", alice)
	}
	if len(alice.Assignments) != 1 || alice.Assignments[0] != (assignmentView{ClusterID: e.cluster, Cluster: "test", Role: "operator"}) {
		t.Fatalf("assignments = %+v", alice.Assignments)
	}
	if admin := findUser(t, e, "admin"); admin.Clusters == nil || admin.Assignments == nil {
		t.Fatal("empty lists must encode as [] rather than null")
	}
}

func TestCreateAndUpdateUsers(t *testing.T) {
	e := newTestEnv(t, nil)
	create := func(name, password, role string) *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/api/users", "admin",
			map[string]string{"username": name, "password": password, "role": role})
	}
	expect(t, create("carol", "carol-password", ""), http.StatusOK, "")
	expect(t, create("carol", "carol-password", "user"), http.StatusConflict, plugin.CodeConflict)
	expect(t, create("dave", "short", "user"), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, create("eve", "eve-password", "root"), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, create("bad name", "a-password", "user"), http.StatusBadRequest, plugin.CodeBadRequest)
	carol := findUser(t, e, "carol")
	if carol.Role != store.RoleUser || !carol.MustChangePassword {
		t.Fatalf("an admin-set password must be changed at first sign-in: %+v", carol)
	}

	patch := func(id int64, body map[string]any) *httptest.ResponseRecorder {
		return e.do(http.MethodPatch, fmt.Sprintf("/api/users/%d", id), "admin", body)
	}
	expect(t, patch(carol.ID, map[string]any{"role": "admin"}), http.StatusOK, "")
	if findUser(t, e, "carol").Role != store.RoleAdmin {
		t.Fatal("role not changed")
	}
	expect(t, patch(carol.ID, map[string]any{}), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, patch(carol.ID, map[string]any{"password": "tiny"}), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, patch(999, map[string]any{"role": "user"}), http.StatusNotFound, plugin.CodeNotFound)

	// A password reset ends the user's sessions and forces a change.
	expect(t, patch(e.users["bob"], map[string]any{"password": "reset-password"}), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/pods", "bob", nil), http.StatusUnauthorized, plugin.CodeSessionExpired)
	if !findUser(t, e, "bob").MustChangePassword {
		t.Fatal("reset did not force a password change")
	}
	rows := e.audit()
	if rows[0].Target != "user/bob" || rows[0].Detail != `{"passwordReset":true}` {
		t.Fatalf("reset audit row = %+v (the password itself is never recorded)", rows[0])
	}
}

func TestUserProtections(t *testing.T) {
	e := newTestEnv(t, nil)
	admin := e.users["admin"]
	expect(t, e.do(http.MethodDelete, fmt.Sprintf("/api/users/%d", admin), "admin", nil), http.StatusConflict, plugin.CodeConflict)
	expect(t, e.do(http.MethodPatch, fmt.Sprintf("/api/users/%d", admin), "admin", map[string]any{"role": "user"}),
		http.StatusConflict, plugin.CodeConflict)

	expect(t, e.do(http.MethodPatch, fmt.Sprintf("/api/users/%d", e.users["alice"]), "admin", map[string]any{"role": "admin"}),
		http.StatusOK, "")
	// With a second admin, an admin may step down, but the last one stays.
	expect(t, e.do(http.MethodPatch, fmt.Sprintf("/api/users/%d", admin), "admin", map[string]any{"role": "user"}), http.StatusOK, "")
	expect(t, e.do(http.MethodPatch, fmt.Sprintf("/api/users/%d", e.users["alice"]), "alice", map[string]any{"role": "user"}),
		http.StatusConflict, plugin.CodeConflict)
	expect(t, e.do(http.MethodDelete, fmt.Sprintf("/api/users/%d", e.users["bob"]), "alice", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodDelete, fmt.Sprintf("/api/users/%d", e.users["bob"]), "alice", nil), http.StatusNotFound, plugin.CodeNotFound)
}

func TestSetAssignments(t *testing.T) {
	e := newTestEnv(t, nil)
	other, err := e.store.AddCluster(store.Cluster{Name: "other", Source: store.SourceKubeconfig, Kubeconfig: testKubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	target := fmt.Sprintf("/api/users/%d/clusters", e.users["bob"])

	expect(t, e.do(http.MethodPut, target, "admin", map[string]any{"assignments": []map[string]any{
		{"clusterId": e.cluster, "role": "operator"}, {"clusterId": other, "role": "viewer"},
	}}), http.StatusOK, "")
	got := findUser(t, e, "bob").Assignments
	if len(got) != 2 || got[0].Role != "operator" || got[1] != (assignmentView{ClusterID: other, Cluster: "other", Role: "viewer"}) {
		t.Fatalf("assignments = %+v", got)
	}
	expect(t, e.do(http.MethodDelete, "/api/pods?ns=default&name=web-1", "bob", nil), http.StatusOK, "")

	// The v0.4 shape grants operator.
	expect(t, e.do(http.MethodPut, target, "admin", map[string]any{"clusterIds": []int64{other}}), http.StatusOK, "")
	if got := findUser(t, e, "bob").Assignments; len(got) != 1 || got[0].ClusterID != other || got[0].Role != "operator" {
		t.Fatalf("legacy assignments = %+v", got)
	}

	expect(t, e.do(http.MethodPut, target, "admin", map[string]any{"assignments": []map[string]any{{"clusterId": other, "role": "owner"}}}),
		http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPut, target, "admin", map[string]any{"assignments": []map[string]any{{"clusterId": 999, "role": "viewer"}}}),
		http.StatusNotFound, plugin.CodeNotFound)
	expect(t, e.do(http.MethodPut, target, "admin", map[string]any{}), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPut, target, "admin", map[string]any{"assignments": []any{}}), http.StatusOK, "")
	if got := findUser(t, e, "bob").Assignments; len(got) != 0 {
		t.Fatalf("an empty list must clear the assignments: %+v", got)
	}
}
