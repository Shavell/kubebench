package api

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

func TestClusterListAndUpdate(t *testing.T) {
	e := newTestEnv(t, nil)
	list := decode[[]clusterView](t, e.do(http.MethodGet, "/api/clusters", "bob", nil))
	if len(list) != 1 {
		t.Fatalf("clusters = %+v", list)
	}
	if c := list[0]; !c.Active || c.Role != "viewer" || c.ReadOnly || c.Environment != "" ||
		c.Server != "https://test.example:6443" || c.Context != "test" {
		t.Fatalf("bob's view = %+v", c)
	}
	if c := decode[[]clusterView](t, e.do(http.MethodGet, "/api/clusters", "admin", nil))[0]; c.Role != "admin" {
		t.Fatalf("admin's role = %q", c.Role)
	}

	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin",
		map[string]any{"name": "prod", "readOnly": true, "environment": "production"}), http.StatusOK, "")
	c := decode[[]clusterView](t, e.do(http.MethodGet, "/api/clusters", "alice", nil))[0]
	if c.Name != "prod" || !c.ReadOnly || c.Environment != "production" || c.Role != "operator" {
		t.Fatalf("after PATCH = %+v", c)
	}
	rows := e.audit()
	if rows[0].Action != "PATCH /api/clusters/{id}" || rows[0].Target != "cluster/test" ||
		!strings.Contains(rows[0].Detail, `"readOnly":true`) {
		t.Fatalf("audit row = %+v", rows[0])
	}

	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"environment": "qa"}), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"name": " "}), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{}), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPatch, "/api/clusters/42", "admin", map[string]any{"readOnly": true}), http.StatusNotFound, plugin.CodeNotFound)
	if _, err := e.store.AddCluster(store.Cluster{Name: "dev", Source: store.SourceAddress, Address: "https://dev:6443"}); err != nil {
		t.Fatal(err)
	}
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"name": "dev"}), http.StatusConflict, plugin.CodeConflict)
}

func TestDeleteActiveCluster(t *testing.T) {
	e := newTestEnv(t, nil)
	expect(t, e.do(http.MethodDelete, "/api/clusters?name=test", "admin", nil), http.StatusOK, "")
	expect(t, e.do(http.MethodGet, "/api/pods", "alice", nil), http.StatusConflict, plugin.CodeNoCluster)
	expect(t, e.do(http.MethodDelete, "/api/clusters?name=test", "admin", nil), http.StatusNotFound, plugin.CodeNotFound)
}

func TestActivateCluster(t *testing.T) {
	e := newTestEnv(t, nil)
	other, err := e.store.AddCluster(store.Cluster{Name: "other", Source: store.SourceKubeconfig, Kubeconfig: testKubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	expect(t, e.do(http.MethodPost, "/api/clusters/activate?name=other", "bob", nil), http.StatusForbidden, plugin.CodeForbidden)
	expect(t, e.do(http.MethodPost, "/api/clusters/activate?name=missing", "admin", nil), http.StatusNotFound, plugin.CodeNotFound)
	expect(t, e.do(http.MethodPost, "/api/clusters/activate?name=other", "admin", nil), http.StatusOK, "")
	if u, _ := e.store.GetUser(e.users["admin"]); u.ActiveClusterID == nil || *u.ActiveClusterID != other {
		t.Fatal("the active cluster did not change")
	}
}

func kubeconfigWithExec() string {
	return strings.Replace(testKubeconfig, "    token: not-a-real-token\n",
		"    exec:\n      apiVersion: client.authentication.k8s.io/v1\n      command: /bin/sh\n", 1)
}

func kubeconfigWithTokenFile() string {
	return strings.Replace(testKubeconfig, "    token: not-a-real-token\n", "    tokenFile: /etc/kubebench/kubebench.env\n", 1)
}

func TestAddClusterGatesServerSideFiles(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config")
	if err := os.WriteFile(path, []byte(testKubeconfig), 0o600); err != nil {
		t.Fatal(err)
	}
	add := func(e *testEnv, body map[string]string) int {
		return e.do(http.MethodPost, "/api/clusters", "admin", body).Code
	}

	locked := newTestEnv(t, map[string]string{"KUBEBENCH_ALLOW_PATH_KUBECONFIG": "false"})
	expect(t, locked.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "p", "path": path}),
		http.StatusForbidden, plugin.CodeForbidden)
	expect(t, locked.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "x", "content": kubeconfigWithExec()}),
		http.StatusForbidden, plugin.CodeForbidden)
	if code := add(locked, map[string]string{"name": "pasted", "content": testKubeconfig}); code != http.StatusOK {
		t.Fatalf("a self-contained kubeconfig must be accepted, got %d", code)
	}
	if info := decode[infoView](t, locked.do(http.MethodGet, "/api/info", "admin", nil)); info.Features.PathKubeconfig {
		t.Fatal("features.pathKubeconfig must follow the configuration")
	}

	// Outside a cluster paths are allowed, but pasted content still may not
	// run commands or read the server's files: that needs its own opt-in.
	open := newTestEnv(t, nil)
	if code := add(open, map[string]string{"name": "from-path", "path": path}); code != http.StatusOK {
		t.Fatalf("path kubeconfig: %d", code)
	}
	for name, content := range map[string]string{"exec": kubeconfigWithExec(), "token-file": kubeconfigWithTokenFile()} {
		expect(t, open.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": name, "content": content}),
			http.StatusForbidden, plugin.CodeForbidden)
	}
	// A kubeconfig file on the server is the server's own configuration.
	execPath := filepath.Join(dir, "exec-config")
	if err := os.WriteFile(execPath, []byte(kubeconfigWithExec()), 0o600); err != nil {
		t.Fatal(err)
	}
	if code := add(open, map[string]string{"name": "exec-path", "path": execPath}); code != http.StatusOK {
		t.Fatalf("an exec plugin in a kubeconfig added by path: %d", code)
	}
	trusting := newTestEnv(t, map[string]string{"KUBEBENCH_ALLOW_KUBECONFIG_EXEC": "true"})
	if code := add(trusting, map[string]string{"name": "exec", "content": kubeconfigWithExec()}); code != http.StatusOK {
		t.Fatalf("KUBEBENCH_ALLOW_KUBECONFIG_EXEC must allow pasted exec plugins: %d", code)
	}
	expect(t, open.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "bad", "content": "{nope"}),
		http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, open.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "ctx", "content": testKubeconfig, "context": "nope"}),
		http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, open.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "from-path", "content": testKubeconfig}),
		http.StatusConflict, plugin.CodeConflict)
}
