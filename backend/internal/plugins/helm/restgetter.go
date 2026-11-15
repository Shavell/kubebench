package helm

import (
	"fmt"
	"net"
	"net/http"
	"net/url"

	"helm.sh/helm/v3/pkg/action"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"

	"kubebench/internal/kube"
)

// newActionConfig is the Helm action configuration for one namespace of
// rt's cluster ("" = all namespaces, for listing), with releases stored in
// Secrets like the helm CLI does.
func (e *environment) newActionConfig(rt *kube.Runtime, namespace string) (*action.Configuration, error) {
	cfg := new(action.Configuration)
	getter := &restGetter{config: rt.Config, namespace: namespace}
	if err := cfg.Init(getter, namespace, "secret", func(string, ...any) {}); err != nil {
		return nil, err
	}
	cfg.RegistryClient = e.registry
	return cfg, nil
}

// restGetter adapts a cluster's rest.Config to the RESTClientGetter the
// Helm SDK builds its Kubernetes clients from.
//
// Those clients are Helm's own: unlike the runtime's clients (§4.5) they
// follow redirects, which a rest.Config cannot switch off. ToRESTConfig
// therefore pins them to the API server's host: a redirect elsewhere fails
// before anything is sent, so the stored credential never leaves the API
// server; redirects within it are still followed.
type restGetter struct {
	config    *rest.Config
	namespace string
}

var _ genericclioptions.RESTClientGetter = (*restGetter)(nil)

func (g *restGetter) ToRESTConfig() (*rest.Config, error) {
	server, _, err := rest.DefaultServerUrlFor(g.config)
	if err != nil {
		return nil, err
	}
	cfg := rest.CopyConfig(g.config)
	host := hostPort(server)
	wrap := cfg.WrapTransport
	cfg.WrapTransport = func(rt http.RoundTripper) http.RoundTripper {
		if wrap != nil {
			rt = wrap(rt)
		}
		return &pinnedHost{host: host, next: rt}
	}
	return cfg, nil
}

func (g *restGetter) ToDiscoveryClient() (discovery.CachedDiscoveryInterface, error) {
	cfg, err := g.ToRESTConfig()
	if err != nil {
		return nil, err
	}
	dc, err := discovery.NewDiscoveryClientForConfig(cfg)
	if err != nil {
		return nil, err
	}
	return memory.NewMemCacheClient(dc), nil
}

func (g *restGetter) ToRESTMapper() (meta.RESTMapper, error) {
	dc, err := g.ToDiscoveryClient()
	if err != nil {
		return nil, err
	}
	return restmapper.NewDeferredDiscoveryRESTMapper(dc), nil
}

// ToRawKubeConfigLoader only carries the namespace: Helm reads its default
// namespace from it, never a kubeconfig file.
func (g *restGetter) ToRawKubeConfigLoader() clientcmd.ClientConfig {
	return clientcmd.NewDefaultClientConfig(*clientcmdapi.NewConfig(), &clientcmd.ConfigOverrides{
		Context: clientcmdapi.Context{Namespace: g.namespace},
	})
}

// pinnedHost refuses requests to any host but the API server's.
type pinnedHost struct {
	host string
	next http.RoundTripper
}

func (p *pinnedHost) RoundTrip(req *http.Request) (*http.Response, error) {
	if hostPort(req.URL) != p.host {
		return nil, fmt.Errorf("refusing a request to %s: Helm only talks to the cluster's API server %s", req.URL.Host, p.host)
	}
	return p.next.RoundTrip(req)
}

// hostPort is u's host with the scheme's default port made explicit.
func hostPort(u *url.URL) string {
	if u.Port() != "" {
		return u.Host
	}
	port := "443"
	if u.Scheme == "http" {
		port = "80"
	}
	return net.JoinHostPort(u.Hostname(), port)
}
