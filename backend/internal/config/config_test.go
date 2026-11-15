package config

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func env(vars map[string]string) func(string) (string, bool) {
	return func(key string) (string, bool) {
		v, ok := vars[key]
		return v, ok
	}
}

func TestDefaults(t *testing.T) {
	c, err := FromEnv(env(nil), false)
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != ":9876" || c.DBPath != "kubebench.db" || c.DataDir != "." {
		t.Fatalf("unexpected defaults: %+v", c)
	}
	if c.PluginsDir != "plugins" {
		t.Fatalf("PluginsDir = %q", c.PluginsDir)
	}
	if !c.AdminPasswordIsDefault || c.AdminPassword != DefaultAdminPassword {
		t.Fatal("expected the default admin password")
	}
	if c.SecureCookies != CookiesAuto || c.TrustProxy || c.TLS() {
		t.Fatalf("unexpected transport defaults: %+v", c)
	}
	if c.SessionTTL != 168*time.Hour || c.AuditRetentionDays != 90 {
		t.Fatalf("unexpected retention defaults: %+v", c)
	}
	if !c.AllowPathKubeconfig {
		t.Fatal("path kubeconfigs must be allowed outside a cluster")
	}
}

func TestInClusterDisablesPathKubeconfig(t *testing.T) {
	c, err := FromEnv(env(nil), true)
	if err != nil {
		t.Fatal(err)
	}
	if c.AllowPathKubeconfig {
		t.Fatal("path kubeconfigs must default to off in-cluster")
	}
	c, err = FromEnv(env(map[string]string{"KUBEBENCH_ALLOW_PATH_KUBECONFIG": "true"}), true)
	if err != nil || !c.AllowPathKubeconfig {
		t.Fatalf("explicit opt-in ignored: %v", err)
	}
}

func TestExplicitValues(t *testing.T) {
	c, err := FromEnv(env(map[string]string{
		"PORT":                           "8080",
		"KUBEBENCH_LISTEN_ADDR":          "127.0.0.1:9000",
		"KUBEBENCH_DB":                   "/var/lib/kubebench/kubebench.db",
		"KUBEBENCH_ADMIN_PASSWORD":       "s3cret-pass",
		"KUBEBENCH_SECURE_COOKIES":       "TRUE",
		"KUBEBENCH_TRUST_PROXY":          "1",
		"KUBEBENCH_TLS_CERT":             "/tls/tls.crt",
		"KUBEBENCH_TLS_KEY":              "/tls/tls.key",
		"KUBEBENCH_SESSION_TTL":          "12h",
		"KUBEBENCH_AUDIT_RETENTION_DAYS": "0",
		"KUBECONFIG":                     "/home/me/.kube/config",
	}), false)
	if err != nil {
		t.Fatal(err)
	}
	if c.ListenAddr != "127.0.0.1:9000" {
		t.Fatalf("listen addr must override PORT, got %q", c.ListenAddr)
	}
	if c.DataDir != "/var/lib/kubebench" || c.PluginsDir != filepath.Join("/var/lib/kubebench", "plugins") {
		t.Fatalf("data dirs not derived from the DB path: %+v", c)
	}
	if c.AdminPasswordIsDefault || c.SecureCookies != CookiesSecure || !c.TrustProxy || !c.TLS() {
		t.Fatalf("unexpected values: %+v", c)
	}
	if c.SessionTTL != 12*time.Hour || c.AuditRetentionDays != 0 || c.Kubeconfig != "/home/me/.kube/config" {
		t.Fatalf("unexpected values: %+v", c)
	}
}

func TestPortDerivesListenAddr(t *testing.T) {
	c, err := FromEnv(env(map[string]string{"PORT": "19876"}), false)
	if err != nil || c.ListenAddr != ":19876" {
		t.Fatalf("ListenAddr = %q, err = %v", c.ListenAddr, err)
	}
}

func TestValidationReportsEveryProblem(t *testing.T) {
	_, err := FromEnv(env(map[string]string{
		"PORT":                           "http",
		"KUBEBENCH_SECURE_COOKIES":       "sometimes",
		"KUBEBENCH_TRUST_PROXY":          "maybe",
		"KUBEBENCH_TLS_CERT":             "/tls/tls.crt",
		"KUBEBENCH_SESSION_TTL":          "forever",
		"KUBEBENCH_AUDIT_RETENTION_DAYS": "-1",
	}), false)
	if err == nil {
		t.Fatal("expected validation errors")
	}
	for _, key := range []string{
		"PORT", "KUBEBENCH_SECURE_COOKIES", "KUBEBENCH_TRUST_PROXY",
		"KUBEBENCH_TLS_CERT/KUBEBENCH_TLS_KEY", "KUBEBENCH_SESSION_TTL", "KUBEBENCH_AUDIT_RETENTION_DAYS",
	} {
		if !strings.Contains(err.Error(), key+":") {
			t.Errorf("error does not mention %s: %v", key, err)
		}
	}
}

func TestSessionTTLMinimum(t *testing.T) {
	if _, err := FromEnv(env(map[string]string{"KUBEBENCH_SESSION_TTL": "30s"}), false); err == nil {
		t.Fatal("a 30s session TTL must be rejected")
	}
}

func TestNamespace(t *testing.T) {
	c, err := FromEnv(env(nil), false)
	if err != nil || c.Namespace != "" {
		t.Fatalf("without KUBEBENCH_NAMESPACE the namespace is unknown: %q, %v", c.Namespace, err)
	}
	c, err = FromEnv(env(map[string]string{"KUBEBENCH_NAMESPACE": " tools "}), true)
	if err != nil || c.Namespace != "tools" {
		t.Fatalf("Namespace = %q, %v", c.Namespace, err)
	}
}
