package kube

import (
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
)

// Discovery returns the cluster's discovery client, cached in memory for
// the runtime's lifetime (reset by ResolveKind/ResolveResource on a miss).
func (r *Runtime) Discovery() discovery.CachedDiscoveryInterface { return r.discovery }

// Mapper returns the RESTMapper over Discovery(). It discovers lazily on
// first use; Reset() forgets what it learned.
func (r *Runtime) Mapper() meta.ResettableRESTMapper { return r.mapper }

// ResolveKind maps an apiVersion ("apps/v1", or "" for the core group's
// preferred version) and kind to its resource and scope.
func (r *Runtime) ResolveKind(apiVersion, kind string) (schema.GroupVersionResource, bool, error) {
	gv, err := schema.ParseGroupVersion(apiVersion)
	if err != nil {
		return schema.GroupVersionResource{}, false, apierrors.NewBadRequest(err.Error())
	}
	var versions []string
	if gv.Version != "" {
		versions = []string{gv.Version}
	}
	var mapping *meta.RESTMapping
	err = r.retryOnNoMatch(func() (err error) {
		mapping, err = r.mapper.RESTMapping(gv.WithKind(kind).GroupKind(), versions...)
		return err
	})
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}
	return mapping.Resource, isNamespaced(mapping), nil
}

// ResolveResource maps a group and resource ("apps", "deployments") to the
// preferred version and the resource's scope.
func (r *Runtime) ResolveResource(group, resource string) (schema.GroupVersionResource, bool, error) {
	var gvr schema.GroupVersionResource
	var mapping *meta.RESTMapping
	err := r.retryOnNoMatch(func() error {
		var err error
		gvr, err = r.mapper.ResourceFor(schema.GroupVersionResource{Group: group, Resource: resource})
		if err != nil {
			return err
		}
		gvk, err := r.mapper.KindFor(gvr)
		if err != nil {
			return err
		}
		mapping, err = r.mapper.RESTMapping(gvk.GroupKind(), gvk.Version)
		return err
	})
	if err != nil {
		return schema.GroupVersionResource{}, false, err
	}
	return gvr, isNamespaced(mapping), nil
}

// retryOnNoMatch runs lookup again with fresh discovery when the cached
// data does not know the type, e.g. a CRD installed after the first lookup.
func (r *Runtime) retryOnNoMatch(lookup func() error) error {
	err := lookup()
	if meta.IsNoMatchError(err) {
		r.mapper.Reset()
		err = lookup()
	}
	return err
}

func isNamespaced(m *meta.RESTMapping) bool {
	return m.Scope.Name() == meta.RESTScopeNameNamespace
}
