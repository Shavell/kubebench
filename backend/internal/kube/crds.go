package kube

import (
	"context"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var crdGVR = schema.GroupVersionResource{
	Group: "apiextensions.k8s.io", Version: "v1", Resource: "customresourcedefinitions",
}

type CrdInfo struct {
	Name       string   `json:"name"`
	Group      string   `json:"group"`
	Kind       string   `json:"kind"`
	Versions   []string `json:"versions"`
	Scope      string   `json:"scope"`
	ShortNames []string `json:"shortNames"`
	Age        string   `json:"age"`
}

type CustomResourceInfo struct {
	Name      string            `json:"name"`
	Namespace string            `json:"namespace,omitempty"`
	CrdName   string            `json:"crdName"`
	Status    string            `json:"status"`
	Age       string            `json:"age"`
	Extra     map[string]string `json:"extra"`
}

type crdSpec struct {
	group, kind, plural, scope string
	version                    string
	printerColumns             []printerColumn
}

type printerColumn struct{ name, jsonPath string }

func (r *Runtime) crdSpec(ctx context.Context, crdName string) (*crdSpec, error) {
	obj, err := r.Dyn.Resource(crdGVR).Get(ctx, crdName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	spec := &crdSpec{
		group:  str(obj.Object, "spec", "group"),
		kind:   str(obj.Object, "spec", "names", "kind"),
		plural: str(obj.Object, "spec", "names", "plural"),
		scope:  str(obj.Object, "spec", "scope"),
	}
	versions, _, _ := unstructured.NestedSlice(obj.Object, "spec", "versions")
	for _, v := range versions {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		served, _ := vm["served"].(bool)
		if !served && spec.version != "" {
			continue
		}
		spec.version, _ = vm["name"].(string)
		spec.printerColumns = nil
		if cols, ok := vm["additionalPrinterColumns"].([]any); ok {
			for _, c := range cols {
				cm, ok := c.(map[string]any)
				if !ok {
					continue
				}
				name, _ := cm["name"].(string)
				jsonPath, _ := cm["jsonPath"].(string)
				if name == "Age" || name == "Name" || name == "Namespace" {
					continue
				}
				if len(spec.printerColumns) < 3 {
					spec.printerColumns = append(spec.printerColumns, printerColumn{name, jsonPath})
				}
			}
		}
		if served {
			break
		}
	}
	return spec, nil
}

func (r *Runtime) ListCrds(ctx context.Context) ([]CrdInfo, error) {
	list, err := r.Dyn.Resource(crdGVR).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := []CrdInfo{}
	for _, item := range list.Items {
		versions := []string{}
		vs, _, _ := unstructured.NestedSlice(item.Object, "spec", "versions")
		for _, v := range vs {
			if vm, ok := v.(map[string]any); ok {
				if served, _ := vm["served"].(bool); served {
					if name, ok := vm["name"].(string); ok {
						versions = append(versions, name)
					}
				}
			}
		}
		shortNames, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "names", "shortNames")
		if shortNames == nil {
			shortNames = []string{}
		}
		out = append(out, CrdInfo{
			Name:       item.GetName(),
			Group:      str(item.Object, "spec", "group"),
			Kind:       str(item.Object, "spec", "names", "kind"),
			Versions:   versions,
			Scope:      str(item.Object, "spec", "scope"),
			ShortNames: shortNames,
			Age:        FormatAge(item.GetCreationTimestamp().Time),
		})
	}
	return out, nil
}

func (r *Runtime) ListCustomResources(ctx context.Context, crdName string) ([]CustomResourceInfo, error) {
	spec, err := r.crdSpec(ctx, crdName)
	if err != nil {
		return nil, err
	}
	gvr := schema.GroupVersionResource{Group: spec.group, Version: spec.version, Resource: spec.plural}
	list, err := r.Dyn.Resource(gvr).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}
	out := []CustomResourceInfo{}
	for _, item := range list.Items {
		extra := map[string]string{}
		for _, col := range spec.printerColumns {
			if value := evalJSONPath(item.Object, col.jsonPath); value != "" {
				extra[col.name] = value
			}
		}
		out = append(out, CustomResourceInfo{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			CrdName:   crdName,
			Status:    crStatus(&item),
			Age:       FormatAge(item.GetCreationTimestamp().Time),
			Extra:     extra,
		})
	}
	return out, nil
}

// GetCustomResourceByKind resolves the CRD by kind and fetches one object.
func (r *Runtime) GetCustomResourceByKind(ctx context.Context, kind, namespace, name string) (*unstructured.Unstructured, error) {
	crds, err := r.ListCrds(ctx)
	if err != nil {
		return nil, err
	}
	for _, crd := range crds {
		if crd.Kind != kind {
			continue
		}
		spec, err := r.crdSpec(ctx, crd.Name)
		if err != nil {
			return nil, err
		}
		gvr := schema.GroupVersionResource{Group: spec.group, Version: spec.version, Resource: spec.plural}
		if spec.scope == "Namespaced" && namespace != "" {
			return r.Dyn.Resource(gvr).Namespace(namespace).Get(ctx, name, metav1.GetOptions{})
		}
		return r.Dyn.Resource(gvr).Get(ctx, name, metav1.GetOptions{})
	}
	return nil, fmt.Errorf("no CRD with kind %q", kind)
}

func crStatus(obj *unstructured.Unstructured) string {
	conditions, _, _ := unstructured.NestedSlice(obj.Object, "status", "conditions")
	for _, c := range conditions {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		t, _ := cm["type"].(string)
		if t == "Ready" || t == "Available" {
			if status, _ := cm["status"].(string); status == "True" {
				return "Ready"
			}
			return "NotReady"
		}
	}
	if phase := str(obj.Object, "status", "phase"); phase != "" {
		return phase
	}
	return "Active"
}

func str(obj map[string]any, fields ...string) string {
	s, _, _ := unstructured.NestedString(obj, fields...)
	return s
}

// evalJSONPath handles the simple ".spec.a.b" paths used by
// additionalPrinterColumns.
func evalJSONPath(obj map[string]any, path string) string {
	parts := strings.Split(strings.TrimPrefix(path, "."), ".")
	var current any = obj
	for _, part := range parts {
		m, ok := current.(map[string]any)
		if !ok {
			return ""
		}
		current, ok = m[part]
		if !ok {
			return ""
		}
	}
	switch v := current.(type) {
	case string:
		return v
	case bool:
		return fmt.Sprintf("%v", v)
	case float64, int64:
		return fmt.Sprintf("%v", v)
	default:
		return ""
	}
}
