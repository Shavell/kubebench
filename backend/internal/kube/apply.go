package kube

import (
	"cmp"
	"context"
	"errors"
	"fmt"

	"k8s.io/apimachinery/pkg/api/equality"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
)

// FieldManager is the field manager of every change KubeBench makes with
// server-side apply.
const FieldManager = "kubebench"

// ApplyAction is what applying a document did (or, in a dry run, would do).
type ApplyAction string

const (
	ActionCreated    ApplyAction = "created"
	ActionConfigured ApplyAction = "configured"
	ActionUnchanged  ApplyAction = "unchanged"
)

// ApplyOptions are the options of Runtime.Apply.
type ApplyOptions struct {
	// Namespace is used for namespaced objects that name none; empty
	// means "default".
	Namespace string
	// DryRun lets the API server process every document without
	// persisting anything.
	DryRun bool
	// Force takes over fields other field managers own instead of
	// failing with a conflict.
	Force bool
}

// ApplyResult is the outcome of one object.
type ApplyResult struct {
	APIVersion, Kind, Namespace, Name string
	// Action is empty when Err is set.
	Action   ApplyAction
	Warnings []string
	Err      error
}

// Apply applies objects (from ParseManifests) in order with server-side
// apply as FieldManager. A failed object does not stop the others; its
// error is in its result.
func (r *Runtime) Apply(ctx context.Context, objects []*unstructured.Unstructured, opts ApplyOptions) []ApplyResult {
	results := make([]ApplyResult, 0, len(objects))
	for _, obj := range objects {
		results = append(results, r.applyObject(ctx, obj, opts))
	}
	return results
}

func (r *Runtime) applyObject(ctx context.Context, obj *unstructured.Unstructured, opts ApplyOptions) ApplyResult {
	res := ApplyResult{APIVersion: obj.GetAPIVersion(), Kind: obj.GetKind(), Namespace: obj.GetNamespace(), Name: obj.GetName()}
	gvr, namespaced, err := r.ResolveKind(res.APIVersion, res.Kind)
	if err != nil {
		res.Err = err
		return res
	}
	switch {
	case !namespaced && res.Namespace != "":
		res.Warnings = append(res.Warnings, fmt.Sprintf(
			"%s is cluster-scoped: metadata.namespace %q was ignored", res.Kind, res.Namespace))
		res.Namespace = ""
	case namespaced && res.Namespace == "":
		res.Namespace = cmp.Or(opts.Namespace, metav1.NamespaceDefault)
	}

	manifest := obj.DeepCopy()
	manifest.SetNamespace(res.Namespace)
	manifest.SetManagedFields(nil) // the API server refuses apply requests that set them
	body, err := manifest.MarshalJSON()
	if err != nil {
		res.Err = err
		return res
	}

	target := Object{Resource: gvr, Kind: res.Kind, Namespaced: namespaced, Namespace: res.Namespace, Name: res.Name}
	client := r.client(target)
	before, err := client.Get(ctx, res.Name, metav1.GetOptions{})
	switch {
	case apierrors.IsNotFound(err):
		before = nil
	case err != nil:
		res.Err = err
		return res
	}

	options := metav1.PatchOptions{FieldManager: FieldManager}
	if opts.Force {
		options.Force = &opts.Force
	}
	if opts.DryRun {
		options.DryRun = []string{metav1.DryRunAll}
	}
	after, err := client.Patch(ctx, res.Name, types.ApplyPatchType, body, options)
	if err != nil {
		res.Err = err
		return res
	}
	res.Action = applyAction(before, after)
	return res
}

// applyAction compares the object before (nil: it did not exist) and
// after the apply. Bookkeeping that changes without the content changing,
// managedFields and resourceVersion, is ignored, so applying a manifest
// that is already in effect counts as unchanged even when KubeBench
// becomes a co-owner of its fields.
func applyAction(before, after *unstructured.Unstructured) ApplyAction {
	if before == nil {
		return ActionCreated
	}
	if equality.Semantic.DeepEqual(withoutBookkeeping(before), withoutBookkeeping(after)) {
		return ActionUnchanged
	}
	return ActionConfigured
}

func withoutBookkeeping(obj *unstructured.Unstructured) map[string]any {
	c := obj.DeepCopy()
	c.SetManagedFields(nil)
	c.SetResourceVersion("")
	return c.Object
}

// IsApplyConflict reports whether err is a server-side apply conflict:
// fields the manifest changes are owned by another field manager. Force
// applying takes them over. A plain 409 (e.g. a stale resourceVersion) is
// not one.
func IsApplyConflict(err error) bool {
	var status apierrors.APIStatus
	if !apierrors.IsConflict(err) || !errors.As(err, &status) || status.Status().Details == nil {
		return false
	}
	for _, cause := range status.Status().Details.Causes {
		if cause.Type == metav1.CauseTypeFieldManagerConflict {
			return true
		}
	}
	return false
}
