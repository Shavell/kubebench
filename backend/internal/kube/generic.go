package kube

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ListGeneric lists any catalog resource via the dynamic client and
// returns the raw objects (the frontend renders columns from them).
// Secret data values are redacted; keys are kept.
func (r *Runtime) ListGeneric(ctx context.Context, key, namespace string) ([]map[string]any, error) {
	rt, ok := Catalog[key]
	if !ok {
		return nil, unknownResource(key)
	}
	var list *unstructured.UnstructuredList
	var err error
	if rt.Namespaced && namespace != "" {
		list, err = r.Dyn.Resource(rt.GVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		list, err = r.Dyn.Resource(rt.GVR).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
		// A type not served by this cluster (older version / not installed)
		// reads as empty rather than an error.
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return []map[string]any{}, nil
		}
		return nil, err
	}
	out := make([]map[string]any, 0, len(list.Items))
	for i := range list.Items {
		obj := list.Items[i].Object
		obj["managedFields"] = nil
		if meta, ok := obj["metadata"].(map[string]any); ok {
			delete(meta, "managedFields")
		}
		if key == "secrets" {
			redactSecret(obj)
		}
		out = append(out, obj)
	}
	return out, nil
}

// GetGeneric fetches one catalog object (used for the detail view).
func (r *Runtime) GetGeneric(ctx context.Context, key, namespace, name string) (map[string]any, error) {
	rt, ok := Catalog[key]
	if !ok {
		return nil, unknownResource(key)
	}
	var obj *unstructured.Unstructured
	var err error
	if rt.Namespaced && namespace != "" {
		obj, err = r.Dyn.Resource(rt.GVR).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
	} else {
		obj, err = r.Dyn.Resource(rt.GVR).Get(ctx, name, metav1.GetOptions{})
	}
	if err != nil {
		return nil, err
	}
	if meta, ok := obj.Object["metadata"].(map[string]any); ok {
		delete(meta, "managedFields")
	}
	if key == "secrets" {
		redactSecret(obj.Object)
	}
	return obj.Object, nil
}

// unknownResource is a 400 for a catalog key the API does not know.
func unknownResource(key string) error {
	return apierrors.NewBadRequest(fmt.Sprintf("unknown resource %q", key))
}

// lastAppliedAnnotation is kubectl's client-side apply state: a copy of
// the manifest last applied, which for a Secret includes its values.
const lastAppliedAnnotation = "kubectl.kubernetes.io/last-applied-configuration"

// redactedValue replaces a hidden value that has no byte count to show.
const redactedValue = "<redacted>"

// redactSecret replaces data/stringData values with a byte-count marker
// so the values never reach the browser, keeping keys visible. kubectl's
// last-applied-configuration annotation repeats the values and is
// redacted too.
func redactSecret(obj map[string]any) {
	for _, field := range []string{"data", "stringData"} {
		if data, ok := obj[field].(map[string]any); ok {
			for k, v := range data {
				if s, ok := v.(string); ok {
					data[k] = fmt.Sprintf("<%d bytes, redacted>", len(s))
				} else {
					data[k] = redactedValue
				}
			}
		}
	}
	if meta, ok := obj["metadata"].(map[string]any); ok {
		if annotations, ok := meta["annotations"].(map[string]any); ok {
			if _, found := annotations[lastAppliedAnnotation]; found {
				annotations[lastAppliedAnnotation] = redactedValue
			}
		}
	}
}

// RedactSecret is redactSecret for content read outside the runtime's
// clients, e.g. the Secrets in a Helm release's rendered manifest: obj is
// a Secret's content (data, stringData, metadata), changed in place.
func RedactSecret(obj map[string]any) { redactSecret(obj) }

// IsRedacted reports whether s is one of redactSecret's markers ("<12
// bytes, redacted>", "<redacted>") rather than a real value. A base64
// data value cannot look like one.
func IsRedacted(s string) bool {
	return strings.HasPrefix(s, "<") && strings.HasSuffix(s, "redacted>")
}
