package kube

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/rest"

	"kubebench/internal/reason"
	"kubebench/internal/store"
)

// OpenShift's cluster monitoring serves PromQL through the Thanos Querier
// in openshift-monitoring. Its port "web" (9091) sits behind
// kube-rbac-proxy, which wants a bearer token allowed by the
// cluster-monitoring-view ClusterRole. The API server's service proxy
// drops the caller's token, so KubeBench queries it directly: in a pod via
// the Service (TLS from the service CA OpenShift mounts into every pod),
// outside the cluster via its Route (TLS from the default ingress CA).
const (
	monitoringNamespace = "openshift-monitoring"
	thanosService       = "thanos-querier"
	thanosPortName      = "web"
	thanosRoute         = "thanos-querier"
	ingressCAConfigMap  = "default-ingress-cert" // in openshift-config-managed
	ingressCANamespace  = "openshift-config-managed"
)

// serviceCAFile is the service CA bundle OpenShift adds to every pod's
// ServiceAccount volume; a variable so tests can point it elsewhere.
var serviceCAFile = "/var/run/secrets/kubernetes.io/serviceaccount/service-ca.crt"

var routeGVR = schema.GroupVersionResource{Group: "route.openshift.io", Version: "v1", Resource: "routes"}

// MonitoringOptions are the operator's explicit choices for reaching
// OpenShift monitoring (KUBEBENCH_OPENSHIFT_MONITORING_*).
type MonitoringOptions struct {
	// Address (host:port) is dialled instead of the Thanos Querier's own
	// host, which still names the TLS server and the Host header — for a
	// router that the Route's host name does not resolve to (e.g. CRC
	// behind a port forward).
	Address string
	// InsecureSkipVerify accepts any TLS certificate of the Thanos Querier.
	InsecureSkipVerify bool
}

// Why OpenShift monitoring cannot serve metrics (GET /api/metrics/sources).
var (
	reasonThanosNoToken   = reason.Define("metrics.thanos_no_token", "OpenShift monitoring needs a bearer token, and the credential of cluster \"{{cluster}}\" has none (client certificates do not pass the thanos-querier Route)")
	reasonThanosNoRoute   = reason.Define("metrics.thanos_no_route", "KubeBench runs outside the cluster and cannot read the thanos-querier Route in openshift-monitoring: {{detail}}")
	reasonThanosNoCA      = reason.Define("metrics.thanos_no_service_ca", "the service CA bundle {{file}} cannot be read: {{detail}}")
	reasonThanosForbidden = reason.Define("metrics.thanos_forbidden", "OpenShift monitoring ({{url}}) refused KubeBench's credential: {{detail}}; bind it to the cluster-monitoring-view ClusterRole")
	reasonThanosFailed    = reason.Define("metrics.thanos_failed", "OpenShift monitoring ({{url}}) did not answer a query: {{detail}}")
)

// discoverThanos returns the Thanos Querier of OpenShift monitoring when
// it answers a query, or why it cannot be used; (nil, nil) when the
// cluster has no OpenShift monitoring.
func (r *Runtime) discoverThanos(ctx context.Context) (*PromSource, error) {
	svc, err := r.CS.CoreV1().Services(monitoringNamespace).Get(ctx, thanosService, metav1.GetOptions{})
	if err != nil {
		return nil, nil
	}
	port := int32(9091)
	for _, p := range svc.Spec.Ports {
		if p.Name == thanosPortName {
			port = p.Port
		}
	}
	base, extraCA, err := r.thanosEndpoint(ctx, port)
	if err != nil {
		return nil, err
	}
	api, err := r.thanosClient(base, extraCA)
	if err != nil {
		return nil, err
	}
	prom := &PromSource{rt: r, location: monitoringNamespace + "/" + thanosService, api: api}
	if _, err := prom.query(ctx, "api/v1/query", map[string]string{"query": "up"}); err != nil {
		var status *httpStatusError
		if errors.As(err, &status) && (status.code == http.StatusUnauthorized || status.code == http.StatusForbidden) {
			return nil, reason.Wrap(err, reasonThanosForbidden, reason.Params{"url": base.String(), "detail": err.Error()})
		}
		return nil, reason.Wrap(err, reasonThanosFailed, reason.Params{"url": base.String(), "detail": err.Error()})
	}
	log.Printf("[prometheus] cluster=%s using OpenShift monitoring at %s", r.Cluster.Name, base)
	return prom, nil
}

// thanosEndpoint is the Thanos Querier's URL with the CA bundle that signs
// its certificate: the Service and the service CA in a pod, the Route and
// the default ingress CA (when readable) elsewhere.
func (r *Runtime) thanosEndpoint(ctx context.Context, port int32) (*url.URL, []byte, error) {
	if r.Cluster.Source == store.SourceInCluster {
		base := &url.URL{Scheme: "https", Host: net.JoinHostPort(thanosService+"."+monitoringNamespace+".svc", strconv.Itoa(int(port)))}
		if r.opts.Monitoring.InsecureSkipVerify {
			return base, nil, nil
		}
		ca, err := os.ReadFile(serviceCAFile)
		if err != nil {
			return nil, nil, reason.Wrap(err, reasonThanosNoCA, reason.Params{"file": serviceCAFile, "detail": err.Error()})
		}
		return base, ca, nil
	}
	route, err := r.Dyn.Resource(routeGVR).Namespace(monitoringNamespace).Get(ctx, thanosRoute, metav1.GetOptions{})
	if err != nil {
		return nil, nil, reason.Wrap(err, reasonThanosNoRoute, reason.Params{"detail": err.Error()})
	}
	host := routeHost(route)
	if host == "" {
		err := errors.New("the Route has no host")
		return nil, nil, reason.Wrap(err, reasonThanosNoRoute, reason.Params{"detail": err.Error()})
	}
	var ca []byte
	if cm, err := r.CS.CoreV1().ConfigMaps(ingressCANamespace).Get(ctx, ingressCAConfigMap, metav1.GetOptions{}); err == nil {
		ca = []byte(cm.Data["ca-bundle.crt"])
	} else if !apierrors.IsNotFound(err) && !apierrors.IsForbidden(err) {
		log.Printf("[prometheus] cluster=%s default ingress CA: %v", r.Cluster.Name, err)
	}
	return &url.URL{Scheme: "https", Host: host}, ca, nil
}

// routeHost is the host the router admitted the Route under, else its
// spec.host.
func routeHost(route *unstructured.Unstructured) string {
	ingress, _, _ := unstructured.NestedSlice(route.Object, "status", "ingress")
	for _, in := range ingress {
		if m, ok := in.(map[string]any); ok {
			if host, _ := m["host"].(string); host != "" {
				return host
			}
		}
	}
	host, _, _ := unstructured.NestedString(route.Object, "spec", "host")
	return host
}

// hasBearer reports whether config authenticates with a bearer token (or
// a plugin or wrapper that adds one), which is all kube-rbac-proxy accepts
// through a Route.
func hasBearer(config *rest.Config) bool {
	return config.BearerToken != "" || config.BearerTokenFile != "" ||
		config.ExecProvider != nil || config.AuthProvider != nil || config.WrapTransport != nil
}

// thanosClient is an HTTPS client of base that signs requests with the
// cluster credential's token. It trusts the system roots plus extraCA,
// dials MonitoringOptions.Address when set and never follows redirects, so
// the token goes nowhere but base.
func (r *Runtime) thanosClient(base *url.URL, extraCA []byte) (*httpAPI, error) {
	if !hasBearer(r.Config) {
		return nil, reason.New(reasonThanosNoToken, reason.Params{"cluster": r.Cluster.Name})
	}
	roots, err := x509.SystemCertPool()
	if err != nil {
		roots = x509.NewCertPool()
	}
	if len(extraCA) > 0 {
		roots.AppendCertsFromPEM(extraCA)
	}
	dialer := &net.Dialer{Timeout: 10 * time.Second, KeepAlive: 30 * time.Second}
	transport := &http.Transport{
		Proxy:               http.ProxyFromEnvironment,
		TLSClientConfig:     &tls.Config{RootCAs: roots, MinVersion: tls.VersionTLS12, InsecureSkipVerify: r.opts.Monitoring.InsecureSkipVerify}, //nolint:gosec // only when the operator asks for it
		TLSHandshakeTimeout: 10 * time.Second,
		IdleConnTimeout:     90 * time.Second,
		ForceAttemptHTTP2:   true,
		DialContext:         dialer.DialContext,
	}
	if addr := r.opts.Monitoring.Address; addr != "" {
		transport.DialContext = func(ctx context.Context, network, _ string) (net.Conn, error) {
			return dialer.DialContext(ctx, network, addr)
		}
	}
	// Only the authentication of the cluster credential is reused: its TLS
	// settings are those of the API server.
	auth := rest.AnonymousClientConfig(r.Config)
	auth.BearerToken, auth.BearerTokenFile = r.Config.BearerToken, r.Config.BearerTokenFile
	auth.ExecProvider, auth.AuthProvider, auth.AuthConfigPersister = r.Config.ExecProvider, r.Config.AuthProvider, r.Config.AuthConfigPersister
	auth.WrapTransport = r.Config.WrapTransport
	signed, err := rest.HTTPWrappersForConfig(auth, transport)
	if err != nil {
		return nil, err
	}
	return &httpAPI{base: base, client: &http.Client{
		Transport:     signed,
		Timeout:       20 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}, nil
}

// httpAPI is a query API reached over HTTP(S) at base.
type httpAPI struct {
	base   *url.URL
	client *http.Client
}

// httpStatusError is a non-2xx answer of the query API.
type httpStatusError struct {
	code int
	body string
}

func (e *httpStatusError) Error() string {
	if e.body == "" {
		return fmt.Sprintf("HTTP %d", e.code)
	}
	return fmt.Sprintf("HTTP %d: %s", e.code, e.body)
}

func (h *httpAPI) get(ctx context.Context, path string, params map[string]string) ([]byte, error) {
	target := h.base.JoinPath(path)
	q := url.Values{}
	for k, v := range params {
		q.Set(k, v)
	}
	target.RawQuery = q.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target.String(), nil)
	if err != nil {
		return nil, err
	}
	res, err := h.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 32<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode < 200 || res.StatusCode > 299 {
		return nil, &httpStatusError{code: res.StatusCode, body: summarize(body)}
	}
	return body, nil
}

// summarize shortens an error body to one line for a reason's detail.
func summarize(body []byte) string {
	text := strings.Join(strings.Fields(string(body)), " ")
	if len(text) > 200 {
		text = text[:200] + "…"
	}
	return text
}
