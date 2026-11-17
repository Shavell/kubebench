package kube

import (
	"net/http"
	"net/url"

	"k8s.io/client-go/rest"
)

// ProxyTransport is an HTTP transport to the API server that adds the
// runtime's credential to every request, for the service browser's
// reverse proxy (docs/design-v0.5.md §7.14), and the server URL it is
// meant for. It speaks HTTP/1.1 only, so that WebSocket upgrades through
// the API server's proxy subresources work. A transport never follows
// redirects; the caller decides what to do with one. It is built on first
// use and shared by every request of the runtime.
func (r *Runtime) ProxyTransport() (http.RoundTripper, *url.URL, error) {
	r.proxyOnce.Do(func() {
		config := rest.CopyConfig(r.Config)
		config.NextProtos = []string{"http/1.1"}
		r.proxyBase, _, r.proxyErr = rest.DefaultServerUrlFor(config)
		if r.proxyErr == nil {
			r.proxyRT, r.proxyErr = rest.TransportFor(config)
		}
	})
	return r.proxyRT, r.proxyBase, r.proxyErr
}
