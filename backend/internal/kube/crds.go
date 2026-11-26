package kube

import (
	"bytes"
	"context"
	"fmt"
	"reflect"
	"strconv"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/util/jsonpath"
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
	// PrinterColumns are the additionalPrinterColumns of the version
	// listed (the first served one), in order; the keys of
	// CustomResourceInfo.Extra. Name, Namespace and Age are left out: the
	// list has them already.
	PrinterColumns []PrinterColumn `json:"printerColumns"`
	// Scale is the scale subresource of the listed version; nil when it
	// has none, so the kind cannot be scaled (POST /api/workloads/scale
	// with apiVersion=<group>/<listed version>).
	Scale *CrdScale `json:"scale,omitempty"`
}

// CrdScale describes a CRD version's scale subresource
// (spec.versions[].subresources.scale): the JSON paths, like
// ".spec.replicas", that the API server maps to the Scale object.
type CrdScale struct {
	SpecReplicasPath   string `json:"specReplicasPath"`
	StatusReplicasPath string `json:"statusReplicasPath"`
	LabelSelectorPath  string `json:"labelSelectorPath,omitempty"`
}

// PrinterColumn describes one additionalPrinterColumn. Like kubectl, the
// UI shows columns of priority 0 by default and the others on request
// (kubectl get -o wide).
type PrinterColumn struct {
	Name        string `json:"name"`
	Type        string `json:"type"` // integer, number, string, boolean or date
	Description string `json:"description,omitempty"`
	Priority    int32  `json:"priority"`
}

type CustomResourceInfo struct {
	Name      string `json:"name"`
	Namespace string `json:"namespace,omitempty"`
	CrdName   string `json:"crdName"`
	Status    string `json:"status"`
	Age       string `json:"age"`
	// Extra holds the formatted printer column values by column name;
	// a column without a value is missing.
	Extra map[string]string `json:"extra"`
	// Replicas is the value at the scale subresource's specReplicasPath
	// (the desired replicas); nil when the CRD has no scale subresource
	// or the object has no integer there.
	Replicas *int64 `json:"replicas,omitempty"`
}

type crdSpec struct {
	group, kind, plural, scope string
	version                    string
	printerColumns             []printerColumn
	scale                      *CrdScale
	specReplicas               *jsonpath.JSONPath
}

type printerColumn struct {
	PrinterColumn
	path *jsonpath.JSONPath
}

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
	spec.version, spec.printerColumns, spec.scale = listedVersion(obj.Object)
	if spec.scale != nil {
		// Like the printer columns: a path that does not parse gives no value.
		spec.specReplicas, _ = parseColumnPath("replicas", spec.scale.SpecReplicasPath)
	}
	return spec, nil
}

// listedVersion picks the version of a CRD that lists use, the first
// served one (or the first one when none is served), parses its printer
// columns and reads its scale subresource (nil without one). Columns
// whose JSONPath does not parse are left out, as the API server leaves
// them out of its tables.
func listedVersion(crd map[string]any) (string, []printerColumn, *CrdScale) {
	versions, _, _ := unstructured.NestedSlice(crd, "spec", "versions")
	var version map[string]any
	for _, v := range versions {
		vm, ok := v.(map[string]any)
		if !ok {
			continue
		}
		if version == nil {
			version = vm
		}
		if served, _ := vm["served"].(bool); served {
			version = vm
			break
		}
	}
	if version == nil {
		return "", nil, nil
	}
	name, _ := version["name"].(string)
	scale := scaleOf(version)
	if scale == nil {
		// apiextensions.k8s.io/v1beta1 kept the subresources on the CRD.
		if top, ok := crd["spec"].(map[string]any); ok {
			scale = scaleOf(top)
		}
	}
	cols, _ := version["additionalPrinterColumns"].([]any)
	if len(cols) == 0 {
		// apiextensions.k8s.io/v1beta1 kept the columns on the CRD.
		cols, _, _ = unstructured.NestedSlice(crd, "spec", "additionalPrinterColumns")
	}
	var out []printerColumn
	for _, c := range cols {
		cm, ok := c.(map[string]any)
		if !ok {
			continue
		}
		col := printerColumn{PrinterColumn: PrinterColumn{
			Name: strField(cm, "name"), Type: strField(cm, "type"), Description: strField(cm, "description"),
		}}
		switch p := cm["priority"].(type) {
		case int64:
			col.Priority = int32(p)
		case float64:
			col.Priority = int32(p)
		}
		if col.Name == "" || col.Name == "Age" || col.Name == "Name" || col.Name == "Namespace" {
			continue
		}
		path, err := parseColumnPath(col.Name, strField(cm, "jsonPath"))
		if err != nil {
			continue
		}
		col.path = path
		out = append(out, col)
	}
	return name, out, scale
}

// scaleOf reads subresources.scale of a CRD version (or a v1beta1 spec).
func scaleOf(m map[string]any) *CrdScale {
	scale, found, _ := unstructured.NestedMap(m, "subresources", "scale")
	if !found || scale == nil {
		return nil
	}
	return &CrdScale{
		SpecReplicasPath:   strField(scale, "specReplicasPath"),
		StatusReplicasPath: strField(scale, "statusReplicasPath"),
		LabelSelectorPath:  strField(scale, "labelSelectorPath"),
	}
}

// replicasAt is the integer at path in obj (the scale subresource's
// specReplicasPath), or nil.
func replicasAt(path *jsonpath.JSONPath, obj map[string]any) *int64 {
	if path == nil {
		return nil
	}
	results, err := path.FindResults(obj)
	if err != nil || len(results) == 0 || len(results[0]) == 0 {
		return nil
	}
	switch v := results[0][0].Interface().(type) {
	case int64:
		return &v
	case float64:
		n := int64(v)
		return &n
	}
	return nil
}

func strField(m map[string]any, key string) string {
	s, _ := m[key].(string)
	return s
}

// parseColumnPath parses a printer column's JSONPath as the API server's
// table convertor does: missing keys are no error, just no value.
func parseColumnPath(name, path string) (*jsonpath.JSONPath, error) {
	p := jsonpath.New(name)
	if err := p.Parse(fmt.Sprintf("{%s}", path)); err != nil {
		return nil, err
	}
	p.AllowMissingKeys(true)
	return p, nil
}

func columnInfos(cols []printerColumn) []PrinterColumn {
	out := make([]PrinterColumn, len(cols))
	for i, c := range cols {
		out[i] = c.PrinterColumn
	}
	return out
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
		_, cols, scale := listedVersion(item.Object)
		shortNames, _, _ := unstructured.NestedStringSlice(item.Object, "spec", "names", "shortNames")
		if shortNames == nil {
			shortNames = []string{}
		}
		out = append(out, CrdInfo{
			Name:           item.GetName(),
			Group:          str(item.Object, "spec", "group"),
			Kind:           str(item.Object, "spec", "names", "kind"),
			Versions:       versions,
			Scope:          str(item.Object, "spec", "scope"),
			ShortNames:     shortNames,
			Age:            FormatAge(item.GetCreationTimestamp().Time),
			PrinterColumns: columnInfos(cols),
			Scale:          scale,
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
			if value, ok := col.value(item.Object); ok {
				extra[col.Name] = value
			}
		}
		out = append(out, CustomResourceInfo{
			Name:      item.GetName(),
			Namespace: item.GetNamespace(),
			CrdName:   crdName,
			Status:    crStatus(&item),
			Age:       FormatAge(item.GetCreationTimestamp().Time),
			Extra:     extra,
			Replicas:  replicasAt(spec.specReplicas, item.Object),
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
	return nil, apierrors.NewNotFound(schema.GroupResource{Resource: "kind"}, kind)
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

// value evaluates the column on obj and formats the result by the
// column's type, as the API server's table convertor does (kubectl get):
// the first result counts, a string column prints any value, the others
// only values of their type; a date shows as an age. ok is false when
// there is no value.
func (c printerColumn) value(obj map[string]any) (string, bool) {
	results, err := c.path.FindResults(obj)
	if err != nil || len(results) == 0 || len(results[0]) == 0 {
		return "", false
	}
	value := results[0][0].Interface()
	if value == nil {
		return "", false
	}
	switch c.Type {
	case "string":
		var buf bytes.Buffer
		if err := c.path.PrintResults(&buf, []reflect.Value{reflect.ValueOf(value)}); err != nil {
			return "", false
		}
		return buf.String(), true
	case "integer":
		switch v := value.(type) {
		case int64:
			return strconv.FormatInt(v, 10), true
		case float64:
			return strconv.FormatInt(int64(v), 10), true
		}
	case "number":
		switch v := value.(type) {
		case int64:
			return strconv.FormatInt(v, 10), true
		case float64:
			return strconv.FormatFloat(v, 'f', -1, 64), true
		}
	case "boolean":
		if b, ok := value.(bool); ok {
			return strconv.FormatBool(b), true
		}
	case "date":
		if s, ok := value.(string); ok {
			t, err := time.Parse(time.RFC3339, s)
			if err != nil {
				return "<invalid>", true
			}
			return FormatAge(t), true
		}
	}
	return "", false
}
