package kube

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// ListGeneric lists any catalog resource via the dynamic client and
// returns the raw objects (the frontend renders columns from them).
// Secret data values are redacted; keys are kept.
func (r *Runtime) ListGeneric(ctx context.Context, key, namespace string) ([]map[string]any, error) {
	rt, ok := Catalog[key]
	if !ok {
		return nil, fmt.Errorf("unknown resource %q", key)
	}
	var list *unstructured.UnstructuredList
	var err error
	if rt.Namespaced && namespace != "" {
		list, err = r.Dyn.Resource(rt.GVR).Namespace(namespace).List(ctx, metav1.ListOptions{})
	} else {
		list, err = r.Dyn.Resource(rt.GVR).List(ctx, metav1.ListOptions{})
	}
	if err != nil {
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
		return nil, fmt.Errorf("unknown resource %q", key)
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

// redactSecret replaces data/stringData values with a byte-count marker
// so the values never reach the browser, keeping keys visible.
func redactSecret(obj map[string]any) {
	for _, field := range []string{"data", "stringData"} {
		if data, ok := obj[field].(map[string]any); ok {
			for k, v := range data {
				if s, ok := v.(string); ok {
					data[k] = fmt.Sprintf("<%d bytes, redacted>", len(s))
				} else {
					data[k] = "<redacted>"
				}
			}
		}
	}
}
