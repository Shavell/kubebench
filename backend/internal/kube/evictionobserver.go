package kube

import (
	"context"

	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/client-go/kubernetes"
	policyclient "k8s.io/client-go/kubernetes/typed/policy/v1"
)

// evictionObserver is a clientset whose policy/v1 evictions report their
// answers to observe. The drain helper only tells when an evicted pod is
// gone; this is how a drain learns that a PodDisruptionBudget refused an
// eviction (429) or why another failed. Clusters that only serve the
// policy/v1beta1 eviction API (before Kubernetes 1.22) are drained all the
// same, without those details.
type evictionObserver struct {
	kubernetes.Interface
	observe func(namespace, name string, err error)
}

func (o evictionObserver) PolicyV1() policyclient.PolicyV1Interface {
	return observedPolicy{PolicyV1Interface: o.Interface.PolicyV1(), observe: o.observe}
}

type observedPolicy struct {
	policyclient.PolicyV1Interface
	observe func(namespace, name string, err error)
}

func (p observedPolicy) Evictions(namespace string) policyclient.EvictionInterface {
	return observedEvictions{EvictionInterface: p.PolicyV1Interface.Evictions(namespace), observe: p.observe}
}

type observedEvictions struct {
	policyclient.EvictionInterface
	observe func(namespace, name string, err error)
}

func (e observedEvictions) Evict(ctx context.Context, eviction *policyv1.Eviction) error {
	err := e.EvictionInterface.Evict(ctx, eviction)
	e.observe(eviction.Namespace, eviction.Name, err)
	return err
}
