// Package kube holds one Runtime per cluster: typed clients, the metrics
// sampler, Prometheus access, Helm actions and probed capabilities.
package kube

import (
	"context"
	"log"
	"sync"
	"time"

	authv1 "k8s.io/api/authorization/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"

	"kubebench/internal/store"
)

type Capabilities struct {
	CRDs bool `json:"crds"`
	Helm bool `json:"helm"`
	Exec bool `json:"exec"`
}

type Runtime struct {
	Cluster store.Cluster
	Config  *rest.Config
	CS      *kubernetes.Clientset
	Dyn     dynamic.Interface
	Metrics *metricsv.Clientset
	Sampler *Sampler
	Helm    *HelmService

	mu   sync.RWMutex
	prom *PromSource
	caps Capabilities
}

func (r *Runtime) Prom() *PromSource {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.prom
}

func (r *Runtime) Caps() Capabilities {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.caps
}

// Manager caches one Runtime per cluster id, building them lazily.
type Manager struct {
	mu       sync.Mutex
	runtimes map[int64]*Runtime
}

func NewManager() *Manager {
	return &Manager{runtimes: map[int64]*Runtime{}}
}

func (m *Manager) Runtime(cluster *store.Cluster) (*Runtime, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rt, ok := m.runtimes[cluster.ID]; ok {
		return rt, nil
	}

	config, err := restConfigFor(cluster)
	if err != nil {
		return nil, err
	}
	config.QPS = 50
	config.Burst = 100

	cs, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	dyn, err := dynamic.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	metrics, err := metricsv.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	rt := &Runtime{Cluster: *cluster, Config: config, CS: cs, Dyn: dyn, Metrics: metrics}
	rt.Sampler = startSampler(rt)
	rt.Helm = newHelmService(config)
	go rt.discoverProm()
	go rt.probeCapabilities()
	m.runtimes[cluster.ID] = rt
	return rt, nil
}

// Drop removes a cached runtime (e.g. when the cluster is deleted).
func (m *Manager) Drop(clusterID int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if rt, ok := m.runtimes[clusterID]; ok {
		rt.Sampler.Stop()
		delete(m.runtimes, clusterID)
	}
}

func restConfigFor(cluster *store.Cluster) (*rest.Config, error) {
	if cluster.Source == "incluster" {
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
	return clientcmd.NewNonInteractiveClientConfig(
		*apiConfig, contextName, &clientcmd.ConfigOverrides{}, nil,
	).ClientConfig()
}

func (r *Runtime) probeCapabilities() {
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
