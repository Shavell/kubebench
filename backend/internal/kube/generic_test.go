package kube

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"strings"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/store"
)

// A Secret created with `kubectl apply` repeats its values in the
// last-applied-configuration annotation; no read path may return them.
func TestSecretValuesAreRedactedOnEveryReadPath(t *testing.T) {
	const value = "S3cr3t-Viewer-Must-Not-See"
	encoded := base64.StdEncoding.EncodeToString([]byte(value))
	applied := `{"apiVersion":"v1","kind":"Secret","metadata":{"name":"db","namespace":"shop"},"stringData":{"password":"` + value + `"}}`
	secret := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1",
		"kind":       "Secret",
		"metadata": map[string]any{
			"name":      "db",
			"namespace": "shop",
			"annotations": map[string]any{
				lastAppliedAnnotation: applied,
				"team":                "payments",
			},
		},
		"data": map[string]any{"password": encoded},
	}}
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{Catalog["secrets"].GVR: "SecretList"}, secret)
	rt := NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, fake.NewClientset(), dyn)
	ctx := context.Background()

	list, err := rt.ListGeneric(ctx, "secrets", "shop")
	if err != nil || len(list) != 1 {
		t.Fatalf("ListGeneric = %v, %v", list, err)
	}
	one, err := rt.GetGeneric(ctx, "secrets", "shop", "db")
	if err != nil {
		t.Fatal(err)
	}
	ref := ObjectRef{Kind: "Secret", Namespace: "shop", Name: "db"}
	text, err := rt.ObjectYAML(ctx, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	neat, err := rt.ObjectYAML(ctx, ref, true)
	if err != nil {
		t.Fatal(err)
	}
	for name, got := range map[string]string{"ListGeneric": asJSON(t, list), "GetGeneric": asJSON(t, one), "ObjectYAML": text, "ObjectYAML neat": neat} {
		if strings.Contains(got, value) || strings.Contains(got, encoded) {
			t.Errorf("%s leaks the Secret value: %s", name, got)
		}
		if !strings.Contains(got, "payments") || !strings.Contains(got, "password") {
			t.Errorf("%s must keep other annotations and the keys: %s", name, got)
		}
	}
}

func asJSON(t *testing.T, v any) string {
	t.Helper()
	b, err := json.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
