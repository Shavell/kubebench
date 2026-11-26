package kube

import (
	"context"
	"fmt"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
)

// ListGeneric lists any catalog resource via the dynamic client and
// returns the raw objects (the frontend renders columns from them).
// Secret data values and the credentials of the credential kinds (a
// Route's TLS key, webhook secrets) are redacted; keys are kept. At most
// limit objects (0 = all) are returned, see pagedList.
func (r *Runtime) ListGeneric(ctx context.Context, key, namespace string, limit int) ([]map[string]any, *Truncated, error) {
	rt, ok := Catalog[key]
	if !ok {
		return nil, nil, unknownResource(key)
	}
	var res dynamic.ResourceInterface = r.Dyn.Resource(rt.GVR)
	if rt.Namespaced && namespace != "" {
		res = r.Dyn.Resource(rt.GVR).Namespace(namespace)
	}
	items, cut, err := pagedList(ctx, limit, unstructuredPage(res))
	if err != nil {
		// A type not served by this cluster (older version / not installed)
		// reads as empty rather than an error.
		if apierrors.IsNotFound(err) || meta.IsNoMatchError(err) {
			return []map[string]any{}, nil, nil
		}
		return nil, nil, err
	}
	out := make([]map[string]any, 0, len(items))
	for i := range items {
		obj := items[i].Object
		obj["managedFields"] = nil
		if meta, ok := obj["metadata"].(map[string]any); ok {
			delete(meta, "managedFields")
		}
		redactObject(rt.GVR.GroupResource(), obj)
		out = append(out, obj)
	}
	return out, cut, nil
}

// unstructuredPage lists one page of res for pagedList.
func unstructuredPage(res dynamic.ResourceInterface) listPage[unstructured.Unstructured] {
	return func(ctx context.Context, opts metav1.ListOptions) ([]unstructured.Unstructured, metav1.ListMeta, error) {
		l, err := res.List(ctx, opts)
		if err != nil {
			return nil, metav1.ListMeta{}, err
		}
		return l.Items, metav1.ListMeta{Continue: l.GetContinue(), RemainingItemCount: l.GetRemainingItemCount()}, nil
	}
}

// GetGeneric fetches one catalog object (used for the detail view),
// redacted like ListGeneric.
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
	redactObject(rt.GVR.GroupResource(), obj.Object)
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
	redactLastApplied(obj)
}

// IsRedacted reports whether s is one of redactSecret's markers ("<12
// bytes, redacted>", "<redacted>") rather than a real value. A base64
// data value cannot look like one.
func IsRedacted(s string) bool {
	return strings.HasPrefix(s, "<") && strings.HasSuffix(s, "redacted>")
}
