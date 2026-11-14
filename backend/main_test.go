package main

import (
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"k8s.io/client-go/tools/clientcmd"

	"kubebench/internal/config"
	"kubebench/internal/store"
)

func testConfig(t *testing.T, env map[string]string) *config.Config {
	t.Helper()
	vars := map[string]string{"KUBEBENCH_DB": filepath.Join(t.TempDir(), "kubebench.db")}
	for k, v := range env {
		vars[k] = v
	}
	cfg, err := config.FromEnv(func(k string) (string, bool) { v, ok := vars[k]; return v, ok }, false)
	if err != nil {
		t.Fatal(err)
	}
	return cfg
}

func openStore(t *testing.T, cfg *config.Config) *store.Store {
	t.Helper()
	st, err := store.Open(cfg.DBPath)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	return st
}

func TestBootstrapForcesChangingTheDefaultPassword(t *testing.T) {
	cfg := testConfig(t, nil)
	st := openStore(t, cfg)
	if err := bootstrap(cfg, st); err != nil {
		t.Fatal(err)
	}
	admin, err := st.Authenticate("admin", config.DefaultAdminPassword)
	if err != nil || admin.Role != store.RoleAdmin || !admin.MustChangePassword {
		t.Fatalf("admin = %+v (err %v)", admin, err)
	}

	custom := testConfig(t, map[string]string{"KUBEBENCH_ADMIN_PASSWORD": "chosen-by-the-operator"})
	st = openStore(t, custom)
	if err := bootstrap(custom, st); err != nil {
		t.Fatal(err)
	}
	if admin, err := st.Authenticate("admin", "chosen-by-the-operator"); err != nil || admin.MustChangePassword {
		t.Fatalf("a configured password needs no change: %+v (err %v)", admin, err)
	}
}

func TestBootstrapSeedsOnlyANewDatabase(t *testing.T) {
	dir := t.TempDir()
	writeTestFile(t, filepath.Join(dir, "ca.crt"), "CA-DATA")
	kubeconfig := filepath.Join(dir, "config")
	writeTestFile(t, kubeconfig, `apiVersion: v1
kind: Config
clusters:
- name: lab
  cluster: {server: "https://lab:6443", certificate-authority: ca.crt}
contexts:
- name: lab
  context: {cluster: lab, user: lab}
current-context: lab
users:
- name: lab
  user: {token: abc}
`)
	cfg := testConfig(t, map[string]string{"KUBECONFIG": kubeconfig})
	st := openStore(t, cfg)
	if err := bootstrap(cfg, st); err != nil {
		t.Fatal(err)
	}
	cluster, err := st.ClusterByName("lab")
	if err != nil {
		t.Fatal(err)
	}
	// The referenced CA file is embedded, so the stored copy is self-contained.
	kc, err := clientcmd.Load([]byte(cluster.Kubeconfig))
	if err != nil || string(kc.Clusters["lab"].CertificateAuthorityData) != "CA-DATA" || kc.Clusters["lab"].CertificateAuthority != "" {
		t.Fatalf("stored kubeconfig = %s (err %v)", cluster.Kubeconfig, err)
	}

	// Later starts leave the inventory alone: a removed seed stays removed.
	if err := st.DeleteCluster(cluster.ID); err != nil {
		t.Fatal(err)
	}
	if err := bootstrap(cfg, st); err != nil {
		t.Fatal(err)
	}
	if _, err := st.ClusterByName("lab"); err == nil {
		t.Fatal("the seed cluster came back on a later start")
	}
}

func writeTestFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

func TestHealthcheck(t *testing.T) {
	healthy := true
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" || !healthy {
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	}))
	defer srv.Close()
	_, port, _ := net.SplitHostPort(strings.TrimPrefix(srv.URL, "http://"))

	cfg := &config.Config{ListenAddr: ":" + port}
	if code := healthcheck(cfg); code != 0 {
		t.Fatalf("healthy server: exit %d", code)
	}
	healthy = false
	if code := healthcheck(cfg); code != 1 {
		t.Fatalf("unhealthy server: exit %d", code)
	}
	srv.Close()
	if code := healthcheck(cfg); code != 1 {
		t.Fatalf("stopped server: exit %d", code)
	}
}

func TestHealthURL(t *testing.T) {
	for addr, want := range map[string]string{
		":9876":          "http://127.0.0.1:9876/healthz",
		"0.0.0.0:9876":   "http://127.0.0.1:9876/healthz",
		"[::]:9876":      "http://127.0.0.1:9876/healthz",
		"10.0.0.5:9876":  "http://10.0.0.5:9876/healthz",
		"localhost:8080": "http://localhost:8080/healthz",
	} {
		if got := healthURL(&config.Config{ListenAddr: addr}); got != want {
			t.Errorf("healthURL(%q) = %q, want %q", addr, got, want)
		}
	}
	tlsCfg := &config.Config{ListenAddr: ":9443", TLSCert: "c", TLSKey: "k"}
	if got := healthURL(tlsCfg); got != "https://127.0.0.1:9443/healthz" {
		t.Errorf("TLS healthURL = %q", got)
	}
}
