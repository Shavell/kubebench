package kube

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubebench/internal/watch"
)

// Watch returns the runtime's live-update hub (§7.10). Close closes it,
// which ends its subscriptions: a rebuilt runtime starts a new one.
func (r *Runtime) Watch() *watch.Hub { return r.watch }

// SetWatchForTest replaces the live-update hub, e.g. with one over a
// dynamic fake that serves list and watch. Call it before the runtime is
// used; Close closes the replacement.
func (r *Runtime) SetWatchForTest(h *watch.Hub) {
	r.watch.Close()
	r.watch = h
}

// resolveTopic maps a live-update topic ("pods", "deployments.apps") to
// the resource at the cluster's preferred version.
func (r *Runtime) resolveTopic(topic string) (schema.GroupVersionResource, error) {
	resource, group := watch.SplitTopic(topic)
	gvr, _, err := r.ResolveResource(group, resource)
	if meta.IsNoMatchError(err) {
		return gvr, &watch.NotServedError{Topic: topic}
	}
	if err != nil {
		return gvr, fmt.Errorf("cannot resolve %s: %v", topic, err)
	}
	return gvr, nil
}
