// Package svcproxy is the service browser's reverse proxy
// (docs/design-v0.5.md §7.14): it forwards a browser's requests for an
// in-cluster Service or Pod through the API server's proxy subresource
// with KubeBench's cluster credential, and shapes the answers so that the
// proxied pages stay isolated from KubeBench. Access is a short-lived
// grant whose random token is part of the URL; the session cookie is
// never used to authorize a proxied request (see Grants).
package svcproxy

import (
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

// Kinds of objects a grant reaches: the API server resources whose proxy
// subresource is used.
const (
	KindService = "services"
	KindPod     = "pods"
)

// Target is the object and port a grant reaches.
type Target struct {
	Kind      string `json:"kind"` // KindService or KindPod
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	// Port is a port number or name: a Service port, or a container port
	// of the Pod.
	Port string `json:"port"`
	// Scheme is how the API server talks to the backend: http or https.
	Scheme string `json:"scheme"`
	// Host, when set, is the Host header the backend gets: the host of
	// the Ingress rule the grant was opened for, so that a virtual-hosted
	// application answers as it does behind its Ingress. Empty: the
	// request keeps the API server's host.
	Host string `json:"host,omitempty"`
}

var (
	dnsLabel     = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
	dnsSubdomain = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*$`)
	portName     = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)
)

// Validate checks the target's fields, which end up in an API server path.
func (t Target) Validate() error {
	switch {
	case t.Kind != KindService && t.Kind != KindPod:
		return fmt.Errorf("unknown kind %q", t.Kind)
	case len(t.Namespace) > 63 || !dnsLabel.MatchString(t.Namespace):
		return fmt.Errorf("invalid namespace %q", t.Namespace)
	case len(t.Name) > 253 || !dnsSubdomain.MatchString(t.Name):
		return fmt.Errorf("invalid name %q", t.Name)
	case t.Scheme != "http" && t.Scheme != "https":
		return fmt.Errorf("scheme must be http or https, not %q", t.Scheme)
	case t.Host != "" && (t.Kind != KindService || len(t.Host) > 253 || !dnsSubdomain.MatchString(t.Host)):
		return fmt.Errorf("invalid host %q", t.Host)
	}
	if n, err := strconv.Atoi(t.Port); err == nil {
		if n < 1 || n > 65535 {
			return fmt.Errorf("port %d is out of range", n)
		}
		return nil
	}
	if len(t.Port) > 15 || !portName.MatchString(t.Port) {
		return fmt.Errorf("invalid port %q", t.Port)
	}
	return nil
}

// APIPath is the path of the target's proxy subresource on the API server,
// without a trailing slash, e.g.
// /api/v1/namespaces/shop/services/http:web:80/proxy.
func (t Target) APIPath() string {
	return "/api/v1/namespaces/" + t.Namespace + "/" + t.Kind + "/" + t.Scheme + ":" + t.Name + ":" + t.Port + "/proxy"
}

// AuditTarget names the target in the audit log: service/<ns>/<name>:<port>
// or pod/<ns>/<name>:<port>.
func (t Target) AuditTarget() string {
	return strings.TrimSuffix(t.Kind, "s") + "/" + t.Namespace + "/" + t.Name + ":" + t.Port
}

// CleanPath validates the in-cluster path of a request, escaped as it
// arrived (e.g. "/static/app.js"), and returns it with a leading slash.
// Dot segments and encoded slashes or backslashes are refused: the path
// is appended to the proxy subresource's path on the API server and must
// not climb out of it.
func CleanPath(escaped string) (string, error) {
	if !strings.HasPrefix(escaped, "/") {
		escaped = "/" + escaped
	}
	lower := strings.ToLower(escaped)
	if strings.Contains(lower, "%2f") || strings.Contains(lower, "%5c") || strings.Contains(escaped, `\`) {
		return "", fmt.Errorf("encoded slashes are not allowed in the path")
	}
	for _, seg := range strings.Split(escaped, "/") {
		decoded, err := url.PathUnescape(seg)
		if err != nil {
			return "", fmt.Errorf("invalid path: %v", err)
		}
		if decoded == "." || decoded == ".." {
			return "", fmt.Errorf("dot segments are not allowed in the path")
		}
	}
	return escaped, nil
}
