package kube

import (
	"context"
	"encoding/json"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
)

// DeleteOptions are the options of Runtime.DeleteObject.
type DeleteOptions struct {
	// Propagation decides what happens to the objects the deleted one
	// owns (e.g. a Deployment's ReplicaSets and their Pods): Background
	// deletes them after it, Foreground before it, Orphan keeps them.
	// Empty leaves the choice to the resource's default.
	Propagation metav1.DeletionPropagation
	// GracePeriod overrides the termination grace period (seconds) when set.
	GracePeriod *int64
}

// DeleteObject deletes one object of any kind.
func (r *Runtime) DeleteObject(ctx context.Context, o Object, opts DeleteOptions) error {
	options := metav1.DeleteOptions{GracePeriodSeconds: opts.GracePeriod}
	if opts.Propagation != "" {
		options.PropagationPolicy = &opts.Propagation
	}
	return r.client(o).Delete(ctx, o.Name, options)
}

// MetadataPatch sets labels and annotations (a nil value removes the key);
// keys it does not name are kept.
type MetadataPatch struct {
	Labels      map[string]*string `json:"labels,omitempty"`
	Annotations map[string]*string `json:"annotations,omitempty"`
}

// PatchMetadata changes an object's labels and annotations with a JSON
// merge patch, which works for every kind, custom resources included.
func (r *Runtime) PatchMetadata(ctx context.Context, o Object, patch MetadataPatch) error {
	body, err := json.Marshal(map[string]MetadataPatch{"metadata": patch})
	if err != nil {
		return err
	}
	_, err = r.client(o).Patch(ctx, o.Name, types.MergePatchType, body, metav1.PatchOptions{FieldManager: FieldManager})
	return err
}

var secretsGVR = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}

// RevealSecret returns a Secret with its values (base64 in data) and
// without managedFields. Every other read path redacts the values; the API
// serves this one on an exec-level, audited route only.
func (r *Runtime) RevealSecret(ctx context.Context, namespace, name string) (map[string]any, error) {
	obj, err := r.Dyn.Resource(secretsGVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	obj.SetManagedFields(nil)
	return obj.Object, nil
}
