// Package config parses the server's environment once at startup
// (docs/design-v0.5.md §2) into a validated, immutable Config.
package config

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"k8s.io/client-go/rest"
)

// DefaultAdminPassword is the bootstrap password used when
// KUBEBENCH_ADMIN_PASSWORD is unset; an admin created with it must change
// it at first login.
const DefaultAdminPassword = "admin"

// CookieMode controls the Secure attribute of the session cookie.
type CookieMode string

const (
	// CookiesAuto marks the cookie Secure when the request arrived over TLS
	// (or, with TrustProxy, carried X-Forwarded-Proto: https).
	CookiesAuto   CookieMode = "auto"
	CookiesSecure CookieMode = "true"
	CookiesPlain  CookieMode = "false"
)

// Config is the parsed server configuration. Treat it as read-only.
type Config struct {
	// ListenAddr is the bind address (KUBEBENCH_LISTEN_ADDR, else ":"+PORT).
	ListenAddr string
	DBPath     string
	DataDir    string
	PluginsDir string

	AdminPassword string
	// AdminPasswordIsDefault reports that the bootstrap password is the
	// well-known default (which is not checked against the password policy).
	AdminPasswordIsDefault bool

	SecureCookies CookieMode
	TrustProxy    bool
	TLSCert       string
	TLSKey        string

	SessionTTL         time.Duration
	AuditRetentionDays int

	AllowPathKubeconfig bool
	// AllowKubeconfigExec lets pasted kubeconfigs run exec credential
	// plugins and auth providers and read files on the server.
	AllowKubeconfigExec bool
	// AllowAmbientCloudIdentity lets pasted kubeconfigs sign in to EKS,
	// GKE and AKS with the server's own cloud identity.
	AllowAmbientCloudIdentity bool
	// Kubeconfig is the KUBECONFIG path list used to seed the inventory on first run.
	Kubeconfig string
	InCluster  bool
	// Namespace is the namespace KubeBench runs in (KUBEBENCH_NAMESPACE; in
	// a pod, by default the pod's own). Deleting it is refused unless
	// forced. Empty when unknown.
	Namespace string

	// UpdateMode is the default update check mode (KUBEBENCH_UPDATE_MODE):
	// "off", "stable" or "prerelease". An admin's choice overrides it.
	UpdateMode string
	// UpdateURL is the releases endpoint the update check reads
	// (KUBEBENCH_UPDATE_URL): GitHub's "list releases" JSON.
	UpdateURL string

	// ProxyOrigin is the separate origin (scheme://host[:port]) that
	// serves the service browser's proxy unsandboxed
	// (KUBEBENCH_PROXY_ORIGIN, §7.14); empty serves it sandboxed on
	// KubeBench's own origin only.
	ProxyOrigin string

	// PublicURL is the URL users reach KubeBench at (KUBEBENCH_PUBLIC_URL,
	// scheme://host[:port], no path); empty derives it from each request.
	// The OIDC redirect URL is built from it (§3.6).
	PublicURL string
	// Auth configures external identities (§3.6, v0.7).
	Auth AuthConfig
	// Monitoring configures how OpenShift monitoring is reached (v0.8).
	Monitoring MonitoringConfig
}

// DefaultUpdateURL lists the releases of KubeBench's GitHub repository.
const DefaultUpdateURL = "https://api.github.com/repos/Shavell/kubebench/releases"

// TLS reports whether the server terminates TLS itself.
func (c *Config) TLS() bool { return c.TLSCert != "" }

// podNamespaceFile holds the namespace of the pod a process runs in.
const podNamespaceFile = "/var/run/secrets/kubernetes.io/serviceaccount/namespace"

// Load reads the process environment. In a pod, Namespace defaults to the
// pod's namespace.
func Load() (*Config, error) {
	_, inClusterErr := rest.InClusterConfig()
	c, err := FromEnv(os.LookupEnv, inClusterErr == nil)
	if err == nil && c.InCluster && c.Namespace == "" {
		if ns, readErr := os.ReadFile(podNamespaceFile); readErr == nil {
			c.Namespace = strings.TrimSpace(string(ns))
		}
	}
	return c, err
}

// FromEnv builds a Config from an environment lookup function; inCluster
// selects the in-cluster defaults. Every problem is reported at once.
func FromEnv(lookup func(string) (string, bool), inCluster bool) (*Config, error) {
	p := parser{lookup: lookup}
	c := &Config{InCluster: inCluster}

	port := p.str("PORT", "9876")
	if !validPort(port) {
		p.fail("PORT", "must be a TCP port (1-65535)")
	}
	c.ListenAddr = ":" + port
	if addr := p.str("KUBEBENCH_LISTEN_ADDR", ""); addr != "" {
		c.ListenAddr = addr
		if _, addrPort, err := net.SplitHostPort(addr); err != nil || !validPort(addrPort) {
			p.fail("KUBEBENCH_LISTEN_ADDR", "must be host:port with a TCP port (1-65535), e.g. 127.0.0.1:9876")
		}
	}

	c.DBPath = p.str("KUBEBENCH_DB", "kubebench.db")
	c.DataDir = p.str("KUBEBENCH_DATA_DIR", filepath.Dir(c.DBPath))
	c.PluginsDir = p.str("KUBEBENCH_PLUGINS_DIR", filepath.Join(c.DataDir, "plugins"))

	c.AdminPassword = p.str("KUBEBENCH_ADMIN_PASSWORD", DefaultAdminPassword)
	c.AdminPasswordIsDefault = c.AdminPassword == DefaultAdminPassword

	switch mode := CookieMode(strings.ToLower(p.str("KUBEBENCH_SECURE_COOKIES", string(CookiesAuto)))); mode {
	case CookiesAuto, CookiesSecure, CookiesPlain:
		c.SecureCookies = mode
	default:
		p.fail("KUBEBENCH_SECURE_COOKIES", "must be auto, true or false")
	}
	c.TrustProxy = p.boolean("KUBEBENCH_TRUST_PROXY", false)

	c.TLSCert = p.str("KUBEBENCH_TLS_CERT", "")
	c.TLSKey = p.str("KUBEBENCH_TLS_KEY", "")
	if (c.TLSCert == "") != (c.TLSKey == "") {
		p.fail("KUBEBENCH_TLS_CERT/KUBEBENCH_TLS_KEY", "must be set together")
	}

	c.SessionTTL = p.duration("KUBEBENCH_SESSION_TTL", 168*time.Hour)
	if c.SessionTTL < time.Minute {
		p.fail("KUBEBENCH_SESSION_TTL", "must be at least 1m")
	}
	c.AuditRetentionDays = p.integer("KUBEBENCH_AUDIT_RETENTION_DAYS", 90)
	if c.AuditRetentionDays < 0 {
		p.fail("KUBEBENCH_AUDIT_RETENTION_DAYS", "must be 0 (keep forever) or a positive number of days")
	}

	// A pasted kubeconfig is whatever an admin's browser sent: running its
	// commands or reading the files it names would give every admin the
	// server's account, so that is off unless the operator opts in.
	c.AllowKubeconfigExec = p.boolean("KUBEBENCH_ALLOW_KUBECONFIG_EXEC", false)
	// Signing in with the server's own cloud identity hands its token to
	// whatever API server a pasted kubeconfig names, so that is opt-in too.
	c.AllowAmbientCloudIdentity = p.boolean("KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY", false)
	// Reading kubeconfigs from server-side paths only makes sense when the
	// server runs on the operator's machine, not inside a pod.
	c.AllowPathKubeconfig = p.boolean("KUBEBENCH_ALLOW_PATH_KUBECONFIG", !inCluster)
	c.Kubeconfig = p.str("KUBECONFIG", "")
	c.Namespace = p.str("KUBEBENCH_NAMESPACE", "")

	switch mode := strings.ToLower(p.str("KUBEBENCH_UPDATE_MODE", "stable")); mode {
	case "off", "stable", "prerelease":
		c.UpdateMode = mode
	default:
		p.fail("KUBEBENCH_UPDATE_MODE", "must be off, stable or prerelease")
	}
	c.UpdateURL = p.str("KUBEBENCH_UPDATE_URL", DefaultUpdateURL)
	if u, err := url.Parse(c.UpdateURL); err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		p.fail("KUBEBENCH_UPDATE_URL", "must be an http or https URL")
	}

	if origin := p.str("KUBEBENCH_PROXY_ORIGIN", ""); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" ||
			strings.Trim(u.Path, "/") != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil {
			p.fail("KUBEBENCH_PROXY_ORIGIN", "must be an http or https origin without a path, e.g. https://proxy.kubebench.example")
		} else {
			c.ProxyOrigin = strings.ToLower(u.Scheme + "://" + u.Host)
		}
	}

	c.PublicURL = p.origin("KUBEBENCH_PUBLIC_URL", "https://kubebench.example.com")
	c.Auth = p.auth()
	c.Monitoring = p.monitoring()

	if len(p.errs) > 0 {
		return nil, errors.Join(p.errs...)
	}
	return c, nil
}

// validPort reports whether s is a TCP port number (1-65535). Service
// names such as "http" are refused: the health check builds a URL from it.
func validPort(s string) bool {
	n, err := strconv.ParseUint(s, 10, 16)
	return err == nil && n >= 1
}

// parser collects validation errors so a misconfigured deployment sees
// every problem in one start attempt.
type parser struct {
	lookup func(string) (string, bool)
	errs   []error
}

func (p *parser) fail(key, msg string) {
	p.errs = append(p.errs, fmt.Errorf("%s: %s", key, msg))
}

// str returns the trimmed value, or fallback when unset or empty.
func (p *parser) str(key, fallback string) string {
	if v, ok := p.lookup(key); ok {
		if v = strings.TrimSpace(v); v != "" {
			return v
		}
	}
	return fallback
}

func (p *parser) boolean(key string, fallback bool) bool {
	raw := p.str(key, "")
	if raw == "" {
		return fallback
	}
	v, err := strconv.ParseBool(raw)
	if err != nil {
		p.fail(key, "must be true or false")
		return fallback
	}
	return v
}

func (p *parser) integer(key string, fallback int) int {
	raw := p.str(key, "")
	if raw == "" {
		return fallback
	}
	v, err := strconv.Atoi(raw)
	if err != nil {
		p.fail(key, "must be an integer")
		return fallback
	}
	return v
}

func (p *parser) duration(key string, fallback time.Duration) time.Duration {
	raw := p.str(key, "")
	if raw == "" {
		return fallback
	}
	v, err := time.ParseDuration(raw)
	if err != nil {
		p.fail(key, "must be a Go duration such as 168h or 30m")
		return fallback
	}
	return v
}
