package watch

import (
	"context"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/dynamic/dynamicinformer"
	"k8s.io/client-go/metadata/metadatainformer"
	"k8s.io/client-go/tools/cache"

	"kubebench/internal/reason"
)

// informer watches one resource type for the hub. Its fields are guarded
// by hub.mu.
type informer struct {
	gvr    schema.GroupVersionResource
	cancel context.CancelFunc
	mode   Mode
	reason *reason.Error // why the mode is polling; nil when live
	// denied is set once access was refused: the mode stays polling until
	// the informer is stopped (a later subscriber after that tries again).
	denied bool
	subs   map[*Subscription][]string // subscriber → its topics of gvr
	idle   *time.Timer                // armed while there are no subscribers
	// idleSince is when the last subscriber left, while subs is empty.
	idleSince time.Time
	// full is set when the resource cannot be listed as metadata only
	// (an aggregated API without PartialObjectMetadata support): the
	// informer then lists whole objects and strips them. Only run uses it.
	full bool
}

// startInformer creates the informer of gvr and starts its goroutine.
// h.mu is held.
func (h *Hub) startInformer(gvr schema.GroupVersionResource) *informer {
	ctx, cancel := context.WithCancel(context.Background())
	inf := &informer{gvr: gvr, cancel: cancel, mode: ModeStarting, subs: map[*Subscription][]string{}}
	h.informers[gvr] = inf
	h.wg.Go(func() { h.run(ctx, inf) })
	h.opts.Logf("[watch] cluster=%s starting informer %s", h.opts.Cluster, gvrName(gvr))
	return inf
}

// attach adds topic of sub. h.mu is held.
func (inf *informer) attach(sub *Subscription, topic string) {
	inf.stopIdle()
	inf.subs[sub] = append(inf.subs[sub], topic)
}

// detach removes sub and reports whether no subscriber is left. h.mu is held.
func (inf *informer) detach(sub *Subscription) bool {
	delete(inf.subs, sub)
	return len(inf.subs) == 0
}

func (inf *informer) stopIdle() {
	if inf.idle != nil {
		inf.idle.Stop()
		inf.idle = nil
	}
}

// setMode records inf's mode and tells its subscribers. A denied informer
// keeps its polling mode.
func (h *Hub) setMode(inf *informer, mode Mode, why *reason.Error, denied bool) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if inf.denied || (inf.mode == mode && sameReason(inf.reason, why)) {
		return
	}
	inf.mode, inf.reason, inf.denied = mode, why, denied
	for sub, topics := range inf.subs {
		for _, t := range topics {
			sub.addStatus(topicStatus(t, mode, why))
		}
	}
}

func sameReason(a, b *reason.Error) bool {
	if a == nil || b == nil {
		return a == b
	}
	return a.Error() == b.Error()
}

// dispatch tells inf's subscribers that an object changed in namespace.
func (h *Hub) dispatch(inf *informer, namespace string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	for sub, topics := range inf.subs {
		for _, t := range topics {
			sub.addChange(Change{Topic: t, Namespace: namespace})
		}
	}
}

// run checks access to inf's resource, then runs its informer until ctx
// ends or access is denied.
func (h *Hub) run(ctx context.Context, inf *informer) {
	if !h.probe(ctx, inf) {
		return
	}
	runCtx, stop := context.WithCancel(ctx)
	defer stop()

	// A metadata informer: change events need no more than names and
	// versions, and e.g. Secret values never leave the API server.
	var si cache.SharedIndexInformer
	if inf.full {
		si = dynamicinformer.NewFilteredDynamicInformer(h.dyn, inf.gvr, metav1.NamespaceAll, 0, cache.Indexers{}, nil).Informer()
	} else {
		si = metadatainformer.NewFilteredMetadataInformer(h.meta, inf.gvr, metav1.NamespaceAll, 0, cache.Indexers{}, nil).Informer()
	}
	// Only names and versions are kept in the cache, not labels,
	// annotations (kubectl's last-applied-configuration of a Secret holds
	// its values) or managed fields.
	_ = si.SetTransform(stripObject)
	_ = si.SetWatchErrorHandlerWithContext(func(_ context.Context, _ *cache.Reflector, err error) {
		if denied(err) {
			h.setMode(inf, ModePolling, deniedReason(inf.gvr, err), true)
			h.opts.Logf("[watch] cluster=%s informer %s denied: %v", h.opts.Cluster, gvrName(inf.gvr), err)
			stop()
			return
		}
		h.opts.Logf("[watch] cluster=%s informer %s: watch failed, retrying: %v", h.opts.Cluster, gvrName(inf.gvr), err)
	})
	_, _ = si.AddEventHandler(cache.ResourceEventHandlerDetailedFuncs{
		AddFunc: func(obj any, initial bool) {
			if !initial {
				h.dispatch(inf, namespaceOf(obj))
			}
		},
		UpdateFunc: func(oldObj, newObj any) {
			// A relist replays unchanged objects as updates.
			if resourceVersion(oldObj) != resourceVersion(newObj) {
				h.dispatch(inf, namespaceOf(newObj))
			}
		},
		DeleteFunc: func(obj any) { h.dispatch(inf, namespaceOf(obj)) },
	})

	ran := make(chan struct{})
	go func() {
		defer close(ran)
		si.RunWithContext(runCtx)
	}()
	if cache.WaitForCacheSync(runCtx.Done(), si.HasSynced) {
		h.setMode(inf, ModeLive, nil, false)
		h.opts.Logf("[watch] cluster=%s informer %s synced", h.opts.Cluster, gvrName(inf.gvr))
	}
	<-ran
}

// probe lists the metadata of one object of inf's resource: a denial
// makes the topic polled for good, other failures until a retry succeeds.
// A resource that cannot be listed as metadata is probed again, and then
// watched, as whole objects. It reports whether the informer may start.
func (h *Hub) probe(ctx context.Context, inf *informer) bool {
	for {
		pctx, cancel := context.WithTimeout(ctx, h.opts.ProbeTimeout)
		var err error
		if !inf.full {
			_, err = h.meta.Resource(inf.gvr).List(pctx, metav1.ListOptions{Limit: 1})
			if noMetadata(err) && h.dyn != nil {
				h.opts.Logf("[watch] cluster=%s informer %s: no metadata-only lists, watching whole objects: %v",
					h.opts.Cluster, gvrName(inf.gvr), err)
				inf.full = true
			}
		}
		if inf.full {
			_, err = h.dyn.Resource(inf.gvr).List(pctx, metav1.ListOptions{Limit: 1})
		}
		cancel()
		switch {
		case ctx.Err() != nil:
			return false
		case err == nil:
			return true
		case denied(err):
			h.setMode(inf, ModePolling, deniedReason(inf.gvr, err), true)
			h.opts.Logf("[watch] cluster=%s informer %s denied: %v", h.opts.Cluster, gvrName(inf.gvr), err)
			return false
		}
		h.setMode(inf, ModePolling, reason.Wrap(err, reasonCannotList,
			reason.Params{"resource": inf.gvr.Resource, "detail": err.Error()}), false)
		select {
		case <-ctx.Done():
			return false
		case <-time.After(h.opts.Retry):
		}
	}
}

// noMetadata reports an API server that cannot serve
// PartialObjectMetadata lists (some aggregated APIs).
func noMetadata(err error) bool {
	return apierrors.IsNotAcceptable(err) || apierrors.IsUnsupportedMediaType(err)
}

// denied reports a failure that retrying with the same credential does
// not fix: no permission, or a resource that cannot be listed or watched.
func denied(err error) bool {
	return apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) ||
		apierrors.IsNotFound(err) || apierrors.IsMethodNotSupported(err) || meta.IsNoMatchError(err)
}

func deniedReason(gvr schema.GroupVersionResource, err error) *reason.Error {
	if apierrors.IsForbidden(err) || apierrors.IsUnauthorized(err) {
		return reason.Wrap(err, reasonDenied, reason.Params{"resource": gvr.Resource})
	}
	return reason.Wrap(err, reasonCannotWatch, reason.Params{"resource": gvr.Resource, "detail": err.Error()})
}

// stripObject keeps what change events need of an object: its name,
// namespace, uid and resource version.
func stripObject(obj any) (any, error) {
	if _, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		return obj, nil
	}
	m, err := meta.Accessor(obj)
	if err != nil {
		return obj, nil
	}
	var typeMeta metav1.TypeMeta
	if t, err := meta.TypeAccessor(obj); err == nil {
		typeMeta = metav1.TypeMeta{APIVersion: t.GetAPIVersion(), Kind: t.GetKind()}
	}
	return &metav1.PartialObjectMetadata{
		TypeMeta: typeMeta,
		ObjectMeta: metav1.ObjectMeta{
			Name: m.GetName(), Namespace: m.GetNamespace(), UID: m.GetUID(), ResourceVersion: m.GetResourceVersion(),
		},
	}, nil
}

func namespaceOf(obj any) string {
	if tomb, ok := obj.(cache.DeletedFinalStateUnknown); ok {
		ns, _, _ := cache.SplitMetaNamespaceKey(tomb.Key)
		return ns
	}
	if m, err := meta.Accessor(obj); err == nil {
		return m.GetNamespace()
	}
	return ""
}

func resourceVersion(obj any) string {
	if m, err := meta.Accessor(obj); err == nil {
		return m.GetResourceVersion()
	}
	return ""
}
