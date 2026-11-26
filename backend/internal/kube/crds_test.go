package kube

import (
	"context"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
	"time"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/store"
)

var widgetsGVR = schema.GroupVersionResource{Group: "compat.example.com", Version: "v1", Resource: "widgets"}

// widgetsCRD has the printer columns of the compatibility check's
// widgets.compat.example.com (filters, indexes), plus columns of every
// type, one of priority 1 and one whose path does not parse.
func widgetsCRD() *unstructured.Unstructured {
	col := func(name, typ, path string, extra ...any) any {
		m := map[string]any{"name": name, "type": typ, "jsonPath": path}
		for i := 0; i+1 < len(extra); i += 2 {
			m[extra[i].(string)] = extra[i+1]
		}
		return m
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "apiextensions.k8s.io/v1",
		"kind":       "CustomResourceDefinition",
		"metadata":   map[string]any{"name": "widgets.compat.example.com"},
		"spec": map[string]any{
			"group": "compat.example.com",
			"scope": "Namespaced",
			"names": map[string]any{"plural": "widgets", "singular": "widget", "kind": "Widget"},
			"versions": []any{
				map[string]any{"name": "v0", "served": false, "storage": false},
				map[string]any{"name": "v1", "served": true, "storage": true, "additionalPrinterColumns": []any{
					col("Size", "integer", ".spec.size"),
					col("Ready", "string", `.status.conditions[?(@.type=="Ready")].status`),
					col("FirstPort", "integer", ".spec.ports[0]"),
					col("Owner", "string", ".spec.owner"),
					col("Ratio", "number", ".spec.ratio", "description", "Share of traffic"),
					col("Paused", "boolean", ".spec.paused"),
					col("Started", "date", ".status.startTime"),
					col("Labels", "string", ".metadata.labels"),
					col("Node", "string", ".status.node", "priority", int64(1)),
					col("Broken", "string", ".spec[", "priority", int64(0)),
					col("Age", "date", ".metadata.creationTimestamp"),
				}},
			},
		},
	}}
}

func widget(name string, spec, status map[string]any) *unstructured.Unstructured {
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "compat.example.com/v1",
		"kind":       "Widget",
		"metadata":   map[string]any{"name": name, "namespace": "team-a", "labels": map[string]any{"tier": "web"}},
		"spec":       spec,
		"status":     status,
	}}
}

func crdRuntime(t *testing.T, objects ...runtime.Object) *Runtime {
	t.Helper()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		crdGVR: "CustomResourceDefinitionList", widgetsGVR: "WidgetList",
	}, objects...)
	rt := NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, fake.NewClientset(), dyn)
	t.Cleanup(rt.Close)
	return rt
}

func TestCrdPrinterColumns(t *testing.T) {
	started := time.Now().Add(-3 * time.Hour).UTC().Format(time.RFC3339)
	rt := crdRuntime(t, widgetsCRD(),
		widget("w1", map[string]any{
			"size": int64(3), "ports": []any{int64(8080), int64(9090)}, "owner": "alice",
			"ratio": 0.25, "paused": false,
		}, map[string]any{
			"conditions": []any{
				map[string]any{"type": "Synced", "status": "False"},
				map[string]any{"type": "Ready", "status": "True"},
			},
			"startTime": started,
			"node":      "node-1",
		}),
		// Wrong types and missing values give no cell, as in kubectl.
		widget("w2", map[string]any{"size": "big", "paused": "yes", "ratio": int64(2)}, map[string]any{"startTime": "yesterday"}),
	)
	ctx := context.Background()

	crds, err := rt.ListCrds(ctx)
	if err != nil || len(crds) != 1 {
		t.Fatalf("ListCrds = %v, %v", crds, err)
	}
	want := []PrinterColumn{
		{Name: "Size", Type: "integer"},
		{Name: "Ready", Type: "string"},
		{Name: "FirstPort", Type: "integer"},
		{Name: "Owner", Type: "string"},
		{Name: "Ratio", Type: "number", Description: "Share of traffic"},
		{Name: "Paused", Type: "boolean"},
		{Name: "Started", Type: "date"},
		{Name: "Labels", Type: "string"},
		{Name: "Node", Type: "string", Priority: 1},
	}
	if !reflect.DeepEqual(crds[0].PrinterColumns, want) {
		t.Fatalf("printer columns = %+v\nwant %+v", crds[0].PrinterColumns, want)
	}
	if !reflect.DeepEqual(crds[0].Versions, []string{"v1"}) {
		t.Fatalf("versions = %v", crds[0].Versions)
	}

	items, _, err := rt.ListCustomResources(ctx, "widgets.compat.example.com", 0)
	if err != nil || len(items) != 2 {
		t.Fatalf("ListCustomResources = %v, %v", items, err)
	}
	byName := map[string]map[string]string{}
	for _, it := range items {
		byName[it.Name] = it.Extra
	}
	wantW1 := map[string]string{
		"Size": "3", "Ready": "True", "FirstPort": "8080", "Owner": "alice",
		"Ratio": "0.25", "Paused": "false", "Started": "3h", "Labels": `{"tier":"web"}`, "Node": "node-1",
	}
	if !reflect.DeepEqual(byName["w1"], wantW1) {
		t.Errorf("w1 extra = %v\nwant %v", byName["w1"], wantW1)
	}
	wantW2 := map[string]string{"Ratio": "2", "Started": "<invalid>", "Labels": `{"tier":"web"}`}
	if !reflect.DeepEqual(byName["w2"], wantW2) {
		t.Errorf("w2 extra = %v\nwant %v", byName["w2"], wantW2)
	}
}

// A CRD without printer columns lists an empty column list, not null.
func TestCrdWithoutPrinterColumns(t *testing.T) {
	crd := widgetsCRD()
	versions, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
	delete(versions[1].(map[string]any), "additionalPrinterColumns")
	_ = unstructured.SetNestedSlice(crd.Object, versions, "spec", "versions")
	rt := crdRuntime(t, crd)
	crds, err := rt.ListCrds(context.Background())
	if err != nil || len(crds) != 1 || crds[0].PrinterColumns == nil || len(crds[0].PrinterColumns) != 0 {
		t.Fatalf("ListCrds = %+v, %v", crds, err)
	}
}

// A version with a scale subresource lists its paths, and each object its
// desired replicas read at specReplicasPath.
func TestCrdScaleSubresource(t *testing.T) {
	crd := widgetsCRD()
	versions, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
	versions[1].(map[string]any)["subresources"] = map[string]any{
		"status": map[string]any{},
		"scale": map[string]any{
			"specReplicasPath":   ".spec.size",
			"statusReplicasPath": ".status.size",
			"labelSelectorPath":  ".status.selector",
		},
	}
	_ = unstructured.SetNestedSlice(crd.Object, versions, "spec", "versions")
	rt := crdRuntime(t, crd,
		widget("w1", map[string]any{"size": int64(3)}, map[string]any{"size": int64(2)}),
		widget("w2", map[string]any{"size": "big"}, map[string]any{}),
	)
	ctx := context.Background()

	crds, err := rt.ListCrds(ctx)
	if err != nil || len(crds) != 1 {
		t.Fatalf("ListCrds = %v, %v", crds, err)
	}
	want := &CrdScale{SpecReplicasPath: ".spec.size", StatusReplicasPath: ".status.size", LabelSelectorPath: ".status.selector"}
	if !reflect.DeepEqual(crds[0].Scale, want) {
		t.Fatalf("scale = %+v, want %+v", crds[0].Scale, want)
	}
	// The JSON of GET /api/crds (CrdDefinition.scale in src/data/types.ts).
	body, _ := json.Marshal(crds[0])
	if !strings.Contains(string(body), `"scale":{"specReplicasPath":".spec.size","statusReplicasPath":".status.size","labelSelectorPath":".status.selector"}`) {
		t.Fatalf("JSON = %s", body)
	}

	items, err := rt.ListCustomResources(ctx, "widgets.compat.example.com")
	if err != nil || len(items) != 2 {
		t.Fatalf("ListCustomResources = %v, %v", items, err)
	}
	for _, it := range items {
		switch it.Name {
		case "w1":
			if it.Replicas == nil || *it.Replicas != 3 {
				t.Errorf("w1 replicas = %v, want 3", it.Replicas)
			}
		case "w2":
			// No integer at the path: no value rather than a made-up 0.
			if it.Replicas != nil {
				t.Errorf("w2 replicas = %v, want none", *it.Replicas)
			}
		}
	}
}

// Without a scale subresource (only on another, unlisted version) the CRD
// lists no scale and its objects no replicas, so the UI offers no Scale.
func TestCrdWithoutScaleSubresource(t *testing.T) {
	crd := widgetsCRD()
	versions, _, _ := unstructured.NestedSlice(crd.Object, "spec", "versions")
	versions[0].(map[string]any)["subresources"] = map[string]any{
		"scale": map[string]any{"specReplicasPath": ".spec.size", "statusReplicasPath": ".status.size"},
	}
	_ = unstructured.SetNestedSlice(crd.Object, versions, "spec", "versions")
	rt := crdRuntime(t, crd, widget("w1", map[string]any{"size": int64(3)}, map[string]any{}))
	ctx := context.Background()
	crds, err := rt.ListCrds(ctx)
	if err != nil || len(crds) != 1 || crds[0].Scale != nil {
		t.Fatalf("ListCrds = %+v, %v", crds, err)
	}
	if body, _ := json.Marshal(crds[0]); strings.Contains(string(body), `"scale"`) {
		t.Fatalf("JSON = %s", body)
	}
	items, err := rt.ListCustomResources(ctx, "widgets.compat.example.com")
	if err != nil || len(items) != 1 || items[0].Replicas != nil {
		t.Fatalf("ListCustomResources = %+v, %v", items, err)
	}
}

// apiextensions.k8s.io/v1beta1 kept the subresources on the CRD spec.
func TestCrdScaleV1beta1(t *testing.T) {
	crd := map[string]any{"spec": map[string]any{
		"versions":     []any{map[string]any{"name": "v1", "served": true}},
		"subresources": map[string]any{"scale": map[string]any{"specReplicasPath": ".spec.replicas", "statusReplicasPath": ".status.replicas"}},
	}}
	_, _, scale := listedVersion(crd)
	if scale == nil || scale.SpecReplicasPath != ".spec.replicas" || scale.StatusReplicasPath != ".status.replicas" {
		t.Fatalf("scale = %+v", scale)
	}
}
