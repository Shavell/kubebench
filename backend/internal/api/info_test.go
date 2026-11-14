package api

import (
	"net/http"
	"testing"

	"kubebench/internal/store"
)

func TestVersionAndHealthzArePublic(t *testing.T) {
	e := newTestEnv(t, nil)
	if got := decode[BuildInfo](t, e.do(http.MethodGet, "/api/version", "", nil)); got != (BuildInfo{Version: "1.2.3", Commit: "abc1234", Date: "2026-11-14"}) {
		t.Fatalf("version = %+v", got)
	}
	if got := decode[map[string]string](t, e.do(http.MethodGet, "/healthz", "", nil)); got["status"] != "ok" {
		t.Fatalf("healthz = %v", got)
	}
}

func TestInfoPermissions(t *testing.T) {
	e := newTestEnv(t, nil)
	for _, tc := range []struct {
		user string
		want permissions
	}{
		{"admin", permissions{Role: "admin", CanRead: true, CanWrite: true, CanExec: true, IsAdmin: true}},
		{"alice", permissions{Role: "operator", CanRead: true, CanWrite: true, CanExec: true}},
		{"bob", permissions{Role: "viewer", CanRead: true}},
	} {
		info := decode[infoView](t, e.do(http.MethodGet, "/api/info", tc.user, nil))
		if info.Permissions != tc.want {
			t.Errorf("%s: permissions = %+v, want %+v", tc.user, info.Permissions, tc.want)
		}
		if info.User != tc.user || info.Version != "1.2.3" || info.Commit != "abc1234" || info.ClusterError != "" {
			t.Errorf("%s: info = %+v", tc.user, info)
		}
		if info.Cluster == nil || info.Cluster.Name != "test" || info.Context != "test" || info.Server == "" {
			t.Errorf("%s: cluster = %+v, context %q, server %q", tc.user, info.Cluster, info.Context, info.Server)
		}
	}

	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"readOnly": true, "environment": "production"}), http.StatusOK, "")
	info := decode[infoView](t, e.do(http.MethodGet, "/api/info", "admin", nil))
	want := permissions{Role: "admin", CanRead: true, IsAdmin: true, ReadOnly: true}
	if info.Permissions != want || !info.Cluster.ReadOnly || info.Cluster.Environment != "production" {
		t.Fatalf("read-only cluster: %+v / %+v", info.Permissions, info.Cluster)
	}
}

func TestInfoSelectsAClusterOnlyWhenNoneIsUsable(t *testing.T) {
	e := newTestEnv(t, nil)
	other, err := e.store.AddCluster(store.Cluster{Name: "other", Source: store.SourceKubeconfig, Kubeconfig: testKubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	// alice loses the active cluster but has another one.
	if err := e.store.SetAssignments(e.users["alice"], []store.Assignment{{ClusterID: other, Role: store.ClusterRoleViewer}}); err != nil {
		t.Fatal(err)
	}
	info := decode[infoView](t, e.do(http.MethodGet, "/api/info", "alice", nil))
	if info.Cluster == nil || info.Cluster.ID != other || info.Permissions.Role != "viewer" {
		t.Fatalf("info = %+v", info)
	}
	expect(t, e.do(http.MethodGet, "/api/pods", "alice", nil), http.StatusOK, "")

	// Without any cluster the session still works and explains why.
	if err := e.store.SetAssignments(e.users["bob"], nil); err != nil {
		t.Fatal(err)
	}
	info = decode[infoView](t, e.do(http.MethodGet, "/api/info", "bob", nil))
	if info.Cluster != nil || info.ClusterError == "" || info.Permissions != (permissions{Role: "viewer"}) {
		t.Fatalf("no-cluster info = %+v", info)
	}
	if info.Capabilities.CRDs || info.Capabilities.Helm || info.Capabilities.Exec {
		t.Fatalf("capabilities without a cluster = %+v", info.Capabilities)
	}
}
