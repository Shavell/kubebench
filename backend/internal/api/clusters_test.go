package api

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
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

// The allowed namespaces (§7.16) are validated, stored and listed; [] clears them.
func TestClusterAllowedNamespaces(t *testing.T) {
	e := newTestEnv(t, nil)
	if c := decode[[]clusterView](t, e.do(http.MethodGet, "/api/clusters", "bob", nil))[0]; c.AllowedNamespaces == nil || len(c.AllowedNamespaces) != 0 {
		t.Fatalf("unset = %#v", c.AllowedNamespaces)
	}
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin",
		map[string]any{"allowedNamespaces": []string{" team-a", "team-b", "team-a", ""}}), http.StatusOK, "")
	c := decode[[]clusterView](t, e.do(http.MethodGet, "/api/clusters", "bob", nil))[0]
	if strings.Join(c.AllowedNamespaces, ",") != "team-a,team-b" {
		t.Fatalf("stored = %v", c.AllowedNamespaces)
	}
	if rows := e.audit(); !strings.Contains(rows[0].Detail, `"allowedNamespaces":["team-a","team-b"]`) {
		t.Fatalf("audit row = %+v", rows[0])
	}
	rec := e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"allowedNamespaces": []string{"Team_A"}})
	if body := decode[plugin.ErrorBody](t, expectRec(t, rec, http.StatusBadRequest, plugin.CodeBadRequest)); body.Reason != "field.not_namespace" {
		t.Fatalf("invalid name: %+v", body)
	}
	many := make([]string, 21)
	for i := range many {
		many[i] = "ns" + strings.Repeat("a", i+1)
	}
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"allowedNamespaces": many}),
		http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "bob", map[string]any{"allowedNamespaces": []string{}}),
		http.StatusForbidden, plugin.CodeAdminRequired)
	expect(t, e.do(http.MethodPatch, "/api/clusters/1", "admin", map[string]any{"allowedNamespaces": []string{}}), http.StatusOK, "")
	if c := decode[[]clusterView](t, e.do(http.MethodGet, "/api/clusters", "bob", nil))[0]; len(c.AllowedNamespaces) != 0 {
		t.Fatalf("cleared = %v", c.AllowedNamespaces)
	}
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

// A path kubeconfig is trusted as one of the server's own files, so it may
// not be one KubeBench wrote from a request: a Helm repository's index
// (bytes of the repository's choosing) in the helm plugin's cache would
// otherwise run its exec plugin as the server's user.
func TestAddClusterByPathRefusesRequestWrittenFiles(t *testing.T) {
	e := newTestEnv(t, nil, newFakePlugin("helm"))
	cache := filepath.Join(e.srv.cfg.DataDir, "helm", "cache")
	if err := os.MkdirAll(cache, 0o700); err != nil {
		t.Fatal(err)
	}
	index := filepath.Join(cache, "evil-index.yaml")
	if err := os.WriteFile(index, []byte(kubeconfigWithExec()), 0o600); err != nil {
		t.Fatal(err)
	}
	// The same file through a symbolic link elsewhere, and a plugin file.
	link := filepath.Join(t.TempDir(), "config")
	if err := os.Symlink(index, link); err != nil {
		t.Fatal(err)
	}
	external := filepath.Join(e.srv.cfg.PluginsDir, "x", "plugin.json")
	if err := os.MkdirAll(filepath.Dir(external), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(external, []byte(kubeconfigWithExec()), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{index, link, filepath.Join(cache, "..", "cache", "evil-index.yaml"), external} {
		expect(t, e.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "evil", "path": path}),
			http.StatusForbidden, plugin.CodeForbidden)
	}
	if _, err := e.store.ClusterByName("evil"); !errors.Is(err, store.ErrNotFound) {
		t.Fatalf("a refused path was added: %v", err)
	}

	// The server's other files, next to its data directory, still work.
	own := filepath.Join(e.srv.cfg.DataDir, "kubeconfig")
	if err := os.WriteFile(own, []byte(kubeconfigWithExec()), 0o600); err != nil {
		t.Fatal(err)
	}
	expect(t, e.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": "own", "path": own}), http.StatusOK, "")
}

// kubeconfigWithAuthProvider replaces the test user's token with an auth
// provider named name and the given settings.
func kubeconfigWithAuthProvider(name string, cfg map[string]string) string {
	var b strings.Builder
	b.WriteString("    auth-provider:\n      name: " + name + "\n      config:\n")
	keys := make([]string, 0, len(cfg))
	for k := range cfg {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		b.WriteString("        " + k + ": " + cfg[k] + "\n")
	}
	return strings.Replace(testKubeconfig, "    token: not-a-real-token\n", b.String(), 1)
}

// The legacy oidc auth provider runs nothing, so a pasted kubeconfig may
// use it as long as its CA is embedded; other providers are refused
// whatever the source, with a reason that says so.
func TestAddClusterAuthProviders(t *testing.T) {
	oidc := map[string]string{
		"idp-issuer-url": "https://idp.example.com/realms/k8s",
		"client-id":      "kubernetes",
		"id-token":       "eyJhbGciOiJSUzI1NiJ9.e30.sig",
		"refresh-token":  "abc",
	}
	with := func(extra map[string]string) map[string]string {
		m := map[string]string{}
		for k, v := range oidc {
			m[k] = v
		}
		for k, v := range extra {
			if v == "" {
				delete(m, k)
			} else {
				m[k] = v
			}
		}
		return m
	}
	add := func(e *testEnv, name, content string) *httptest.ResponseRecorder {
		return e.do(http.MethodPost, "/api/clusters", "admin", map[string]string{"name": name, "content": content})
	}
	reasonOf := func(rec *httptest.ResponseRecorder) string {
		return decode[plugin.ErrorBody](t, rec).Reason
	}

	e := newTestEnv(t, nil)
	expect(t, add(e, "oidc", kubeconfigWithAuthProvider("oidc", oidc)), http.StatusOK, "")
	expect(t, add(e, "oidc-ca-data", kubeconfigWithAuthProvider("oidc", with(map[string]string{
		"idp-certificate-authority-data": "LS0tLS1CRUdJTiBDRVJUSUZJQ0FURS0tLS0tCg==",
	}))), http.StatusOK, "")

	rec := add(e, "oidc-ca-file", kubeconfigWithAuthProvider("oidc", with(map[string]string{
		"idp-certificate-authority": "/var/run/secrets/kubernetes.io/serviceaccount/ca.crt",
	})))
	expect(t, rec, http.StatusForbidden, plugin.CodeForbidden)
	if got := reasonOf(rec); got != "clusters.server_side_references" ||
		!strings.Contains(rec.Body.String(), "idp-certificate-authority file") {
		t.Fatalf("CA file refusal: %s", rec.Body.String())
	}

	for name, content := range map[string]string{
		"gcp":       kubeconfigWithAuthProvider("gcp", map[string]string{"cmd-path": "/usr/bin/gcloud"}),
		"no-issuer": kubeconfigWithAuthProvider("oidc", with(map[string]string{"idp-issuer-url": ""})),
		"no-token":  kubeconfigWithAuthProvider("oidc", with(map[string]string{"id-token": "", "refresh-token": ""})),
	} {
		rec := add(e, name, content)
		expect(t, rec, http.StatusBadRequest, plugin.CodeBadRequest)
		want := "clusters.oidc_incomplete"
		if name == "gcp" {
			want = "clusters.auth_provider_unsupported"
		}
		if got := reasonOf(rec); got != want {
			t.Errorf("%s: reason %q, want %q (%s)", name, got, want, rec.Body.String())
		}
	}

	// The CA file is fine where reading the server's files is allowed.
	trusting := newTestEnv(t, map[string]string{"KUBEBENCH_ALLOW_KUBECONFIG_EXEC": "true"})
	expect(t, add(trusting, "oidc-ca-file", kubeconfigWithAuthProvider("oidc", with(map[string]string{
		"idp-certificate-authority": "/etc/ssl/idp.pem",
	}))), http.StatusOK, "")
	// An unsupported provider fails whatever the opt-in.
	expect(t, add(trusting, "gcp", kubeconfigWithAuthProvider("gcp", nil)), http.StatusBadRequest, plugin.CodeBadRequest)
}
