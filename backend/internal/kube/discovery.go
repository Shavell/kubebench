package kube

import (
	"context"
	"log"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/metadata"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"
)

// Discovery returns the cluster's discovery client, cached in memory for
// the runtime's lifetime: reset whenever a CustomResourceDefinition is
// added, changed or deleted (see watchCRDs), and by
// ResolveKind/ResolveResource on a miss, at most once per
// mapperResetInterval.
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

// mapperResetInterval is the least time between two discovery refreshes
// of retryOnNoMatch: lookups of unknown types (any reader can ask for
// them, e.g. through watch topics) cost the API server at most one full
// discovery per interval.
const mapperResetInterval = 10 * time.Second

// retryOnNoMatch runs lookup again with fresh discovery when the cached
// data does not know the type, e.g. a CRD installed after the first lookup.
// Discovery is refreshed at most once per mapperResetInterval; within it
// a miss stands (the data is at most that old).
func (r *Runtime) retryOnNoMatch(lookup func() error) error {
	err := lookup()
	if meta.IsNoMatchError(err) && r.resetMapper() {
		err = lookup()
	}
	return err
}

// resetMapper forgets the cached discovery data unless it was refreshed
// within mapperResetInterval, and reports whether it did.
func (r *Runtime) resetMapper() bool {
	r.resetMu.Lock()
	defer r.resetMu.Unlock()
	if !r.lastReset.IsZero() && time.Since(r.lastReset) < mapperResetInterval {
		return false
	}
	r.mapper.Reset()
	r.lastReset = time.Now()
	return true
}

func isNamespaced(m *meta.RESTMapping) bool {
	return m.Scope.Name() == meta.RESTScopeNameNamespace
}

// crdSettle is how long after a CRD change discovery is forgotten once
// more: the API server updates its discovery documents shortly after the
// CRD itself, so a lookup between the two would cache the old state.
const crdSettle = 3 * time.Second

// watchCRDs forgets the cached discovery data whenever a
// CustomResourceDefinition is added, changed or deleted (e.g. a version
// removed), and again crdSettle later, until stop is closed. It watches
// the CRDs' metadata only. Without access to CRDs it gives up, and only
// retryOnNoMatch refreshes discovery.
func (r *Runtime) watchCRDs(stopped <-chan struct{}, client metadata.Interface) {
	ctx, stop := context.WithCancel(context.Background())
	defer stop()
	go func() {
		select {
		case <-stopped:
			stop()
		case <-ctx.Done():
		}
	}()
	si := metadatainformer.NewFilteredMetadataInformer(client, crdGVR, metav1.NamespaceAll, 0, cache.Indexers{}, nil).Informer()
	_ = si.SetTransform(func(obj any) (any, error) {
		m, err := meta.Accessor(obj)
		if err != nil {
			return obj, nil // a deletion tombstone
		}
		return &metav1.PartialObjectMetadata{ObjectMeta: metav1.ObjectMeta{
			Name: m.GetName(), UID: m.GetUID(), ResourceVersion: m.GetResourceVersion(),
		}}, nil
	})
	_ = si.SetWatchErrorHandlerWithContext(func(_ context.Context, _ *cache.Reflector, err error) {
		if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) || apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			log.Printf("[discovery] cluster=%s cannot watch CRDs, discovery refreshes on misses only: %v", r.Cluster.Name, err)
			stop()
		}
	})

	settle := time.AfterFunc(crdSettle, r.invalidateDiscovery)
	settle.Stop()
	defer settle.Stop()
	changed := func() {
		r.invalidateDiscovery()
		settle.Reset(crdSettle)
	}
	_, _ = si.AddEventHandler(cache.ResourceEventHandlerDetailedFuncs{
		AddFunc: func(_ any, initial bool) {
			if !initial {
				changed()
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			o, err1 := meta.Accessor(oldObj)
			n, err2 := meta.Accessor(newObj)
			if err1 != nil || err2 != nil || o.GetResourceVersion() != n.GetResourceVersion() {
				changed()
			}
		},
		DeleteFunc: func(any) { changed() },
	})
	si.RunWithContext(ctx)
}

// invalidateDiscovery forgets the cached discovery data; the next lookup
// discovers again. It does not count against mapperResetInterval.
func (r *Runtime) invalidateDiscovery() {
	r.mapper.Reset()
}
