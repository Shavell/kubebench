// Package kube holds one Runtime per cluster: typed and dynamic clients,
// cached discovery with a RESTMapper, the metrics sampler, Prometheus
// access and probed capabilities.
package kube

import (
	"context"
	"log"
	"net/http"
	"sync"
	"time"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/discovery/cached/memory"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/restmapper"
	"k8s.io/client-go/tools/clientcmd"
	clientcmdapi "k8s.io/client-go/tools/clientcmd/api"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"kubebench/internal/cloud"
	"kubebench/internal/cloudauth"
	"kubebench/internal/store"
	"kubebench/internal/watch"
)

type Capabilities struct {
	CRDs bool `json:"crds"`
	Helm bool `json:"helm"`
	Exec bool `json:"exec"`
}

// Runtime is the connection to one cluster. Its exported fields are set at
// construction and not changed afterwards: the Manager replaces the whole
// Runtime when the stored cluster record changes. The clients are
// interfaces so tests can inject fakes (see NewRuntimeForTest).
type Runtime struct {
	Cluster store.Cluster
	Config  *rest.Config
	CS      kubernetes.Interface
	Dyn     dynamic.Interface
	Metrics metricsv.Interface
	Sampler *Sampler

	watch     *watch.Hub // live updates (§7.10); see Watch
	discovery discovery.CachedDiscoveryInterface
	mapper    *restmapper.DeferredDiscoveryRESTMapper
	probed    chan struct{} // closed once the capability probe finished
	promDone  chan struct{} // closed once Prometheus discovery finished
	closeOnce sync.Once

	rawOnce   sync.Once // builds rawClient for RawGet on first use
	rawClient *http.Client
	rawErr    error

	mu       sync.RWMutex
	prom     *PromSource
	caps     Capabilities
	platform cloudState // see Cloud
}

// newRuntime connects to cluster and starts its background work: the
// metrics sampler, Prometheus discovery and the capability probe.
func newRuntime(cluster store.Cluster) (*Runtime, error) {
	config, err := restConfigFor(&cluster)
	if err != nil {
		return nil, err
	}
	config.QPS = 50
	config.Burst = 100
	// Names KubeBench in the API server's audit log, and as the field
	// manager of changes made without an explicit one.
	config.UserAgent = FieldManager

	cs, dyn, metrics, err := clientsFor(config)
	if err != nil {
		return nil, err
	}
	rt := assemble(cluster, config, cs, dyn, metrics)
	rt.Sampler.start()
	go rt.discoverProm()
	go rt.probeCapabilities()
	go rt.detectCloudOnce()
	return rt, nil
}

// clientsFor builds the typed, dynamic and metrics clients over one HTTP
// client that does not follow redirects (see httpClientFor).
func clientsFor(config *rest.Config) (kubernetes.Interface, dynamic.Interface, metricsv.Interface, error) {
	httpClient, err := httpClientFor(config)
	if err != nil {
		return nil, nil, nil, err
	}
	cs, err := kubernetes.NewForConfigAndClient(config, httpClient)
	if err != nil {
		return nil, nil, nil, err
	}
	dyn, err := dynamic.NewForConfigAndClient(config, httpClient)
	if err != nil {
		return nil, nil, nil, err
	}
	metrics, err := metricsv.NewForConfigAndClient(config, httpClient)
	if err != nil {
		return nil, nil, nil, err
	}
	return cs, dyn, metrics, nil
}

// NewRuntimeForTest builds a Runtime around injected (usually fake)
// clients. It starts no background work: the capability probe counts as
// finished with no capabilities, Prometheus discovery as finished without
// one, cloud detection as finished with nothing recognised (see
// DetectCloudForTest), the sampler stays empty, Metrics is nil, and Config points
// at an unreachable host. Tests that need metrics or RawGet set Metrics or
// Config before using the runtime.
func NewRuntimeForTest(cluster store.Cluster, cs kubernetes.Interface, dyn dynamic.Interface) *Runtime {
	rt := assemble(cluster, &rest.Config{Host: "https://kubebench.invalid"}, cs, dyn, nil)
	close(rt.probed)
	close(rt.promDone)
	rt.platform.detection, rt.platform.at = cloud.Detection{Provider: cloud.ProviderKubernetes}, time.Now()
	close(rt.platform.done)
	return rt
}

// DetectCloudForTest runs cloud detection against the runtime's clients
// now, as a fresh runtime does in the background.
func (r *Runtime) DetectCloudForTest() { r.refreshCloud() }

// ProbeForTest makes the capability probe of a runtime from
// NewRuntimeForTest pending again, as on a freshly built runtime, and
// returns the function that finishes it with caps. Call it before the
// runtime is used.
func (r *Runtime) ProbeForTest() (finish func(Capabilities)) {
	r.mu.Lock()
	r.caps = Capabilities{}
	r.mu.Unlock()
	r.probed = make(chan struct{})
	return func(caps Capabilities) {
		r.mu.Lock()
		r.caps = caps
		r.mu.Unlock()
		close(r.probed)
	}
}

// SetPromForTest makes Prom() return a Prometheus at the given Service,
// queried through the clientset's service proxy, as discovery would.
func (r *Runtime) SetPromForTest(namespace, service, port string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.prom = &PromSource{rt: r, Namespace: namespace, Service: service, Port: port}
}

func assemble(cluster store.Cluster, config *rest.Config, cs kubernetes.Interface,
	dyn dynamic.Interface, metrics metricsv.Interface) *Runtime {
	cached := memory.NewMemCacheClient(cs.Discovery())
	rt := &Runtime{
		Cluster:   cluster,
		Config:    config,
		CS:        cs,
		Dyn:       dyn,
		Metrics:   metrics,
		discovery: cached,
		mapper:    restmapper.NewDeferredDiscoveryRESTMapper(cached),
		probed:    make(chan struct{}),
		promDone:  make(chan struct{}),
		platform:  cloudState{done: make(chan struct{})},
	}
	rt.Sampler = newSampler(rt)
	rt.watch = watch.NewHub(dyn, rt.resolveTopic, watch.Options{Cluster: cluster.Name})
	return rt
}

// Close stops the runtime's background work and ends its live-update
// subscriptions. It is safe to call more than once; the clients keep
// working for requests still holding the runtime.
func (r *Runtime) Close() {
	r.closeOnce.Do(func() {
		r.Sampler.Stop()
		r.watch.Close()
	})
}

// Prom returns the Prometheus discovery found, or nil; see
// WaitPromDiscovered.
func (r *Runtime) Prom() *PromSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prom
}

// Caps returns the probed capabilities; all false until WaitProbed returns.
func (r *Runtime) Caps() Capabilities {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.caps
}

// WaitProbed blocks until the capability probe finished, or returns ctx's
// error when ctx ends first.
func (r *Runtime) WaitProbed(ctx context.Context) error { return wait(ctx, r.probed) }

// WaitPromDiscovered blocks until Prometheus discovery finished, so that
// Prom() is final, or returns ctx's error when ctx ends first.
func (r *Runtime) WaitPromDiscovered(ctx context.Context) error { return wait(ctx, r.promDone) }

func wait(ctx context.Context, done <-chan struct{}) error {
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func restConfigFor(cluster *store.Cluster) (*rest.Config, error) {
	if cluster.Source == store.SourceInCluster {
		return rest.InClusterConfig()
	}
	apiConfig, err := clientcmd.Load([]byte(cluster.Kubeconfig))
	if err != nil {
		return nil, err
	}
	contextName := cluster.Context
	if contextName == "" {
		contextName = apiConfig.CurrentContext
	}
	wrap, err := cloudAuth(apiConfig, contextName, cluster.Credentials)
	if err != nil {
		return nil, err
	}
	config, err := clientcmd.NewNonInteractiveClientConfig(
		*apiConfig, contextName, &clientcmd.ConfigOverrides{}, nil,
	).ClientConfig()
	if err != nil {
		return nil, err
	}
	if wrap != nil {
		config.Wrap(wrap)
	}
	return config, nil
}

// cloudAuth replaces the EKS, GKE and AKS exec plugins of the context's
// user with in-process token providers (§7.12): it removes the recognised
// stanza from apiConfig and returns the transport wrapper that adds the
// tokens, or nil when the user needs none.
func cloudAuth(apiConfig *clientcmdapi.Config, contextName, storedCredentials string) (func(http.RoundTripper) http.RoundTripper, error) {
	creds, err := cloudauth.ParseCredentials(storedCredentials)
	if err != nil {
		return nil, err
	}
	var user *clientcmdapi.AuthInfo
	if ctx, ok := apiConfig.Contexts[contextName]; ok {
		user = apiConfig.AuthInfos[ctx.AuthInfo]
	}
	return cloudauth.Default.Apply(user, creds)
}

func (r *Runtime) probeCapabilities() {
	defer close(r.probed)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	caps := Capabilities{}

	crdGVR := schema.GroupVersionResource{
		Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions",
	}
	if _, err := r.Dyn.Resource(crdGVR).List(ctx, metav1.ListOptions{Limit: 1}); err == nil {
		caps.CRDs = true
	}

	// Helm needs the release secrets; probing a list in all namespaces
	// covers both the SDK availability and RBAC.
	if _, err := r.CS.CoreV1().Secrets("").List(ctx, metav1.ListOptions{
		LabelSelector: "owner=helm", Limit: 1,
	}); err == nil {
		caps.Helm = true
	}

	ssar := &authv1.SelfSubjectAccessReview{
		Spec: authv1.SelfSubjectAccessReviewSpec{
			ResourceAttributes: &authv1.ResourceAttributes{
				Verb: "create", Resource: "pods", Subresource: "exec",
			},
		},
	}
	if res, err := r.CS.AuthorizationV1().SelfSubjectAccessReviews().Create(
		ctx, ssar, metav1.CreateOptions{},
	); err == nil && res.Status.Allowed {
		caps.Exec = true
	}

	r.mu.Lock()
	r.caps = caps
	r.mu.Unlock()
	log.Printf("[capabilities] cluster=%s crds=%v helm=%v exec=%v",
		r.Cluster.Name, caps.CRDs, caps.Helm, caps.Exec)
}
