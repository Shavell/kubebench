package kube

import (
	"context"
	"errors"
	"strings"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/kube/kubetest"
)

func TestResolveObject(t *testing.T) {
	rt, _ := editingRuntime(t)
	obj, err := rt.ResolveObject("apps/v1", "Deployment", "shop", "web")
	if err != nil || obj.Resource.Resource != "deployments" || !obj.Namespaced || obj.Target() != "deployment/shop/web" {
		t.Fatalf("Deployment: %+v %v", obj, err)
	}
	obj, err = rt.ResolveObject("rbac.authorization.k8s.io/v1", "ClusterRole", "shop", "view")
	if err != nil || obj.Namespace != "" || obj.Target() != "clusterrole/view" {
		t.Fatalf("a cluster-scoped kind ignores the namespace: %+v %v", obj, err)
	}
	var re *RequestError
	if _, err := rt.ResolveObject("v1", "ConfigMap", "", "app"); !errors.As(err, &re) {
		t.Fatalf("a namespaced kind without a namespace: %v", err)
	}
}

func secretFields() map[string]any {
	return map[string]any{
		"type": "Opaque",
		"data": map[string]any{"password": "aHVudGVyMg=="},
	}
}

func TestRevealSecretAndRedactedReads(t *testing.T) {
	secret := kubetest.Object("v1", "Secret", "shop", "db", secretFields())
	secret.SetAnnotations(map[string]string{lastAppliedAnnotation: `{"data":{"password":"aHVudGVyMg=="}}`})
	secret.SetManagedFields([]metav1.ManagedFieldsEntry{{Manager: "kubectl"}})
	rt, _ := editingRuntime(t, secret)
	ctx := context.Background()

	revealed, err := rt.RevealSecret(ctx, "shop", "db")
	if err != nil {
		t.Fatal(err)
	}
	if revealed["data"].(map[string]any)["password"] != "aHVudGVyMg==" {
		t.Fatalf("revealed data = %v", revealed["data"])
	}
	if _, found := revealed["metadata"].(map[string]any)["managedFields"]; found {
		t.Fatal("managedFields must not be revealed")
	}

	// Every other read redacts the values, the last-applied copy included.
	for _, neat := range []bool{false, true} {
		text, err := rt.ObjectYAML(ctx, ObjectRef{APIVersion: "v1", Kind: "Secret", Namespace: "shop", Name: "db"}, neat)
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(text, "aHVudGVyMg") || !strings.Contains(text, "password: <12 bytes, redacted>") {
			t.Fatalf("neat=%v YAML shows the value:\n%s", neat, text)
		}
	}
	generic, err := rt.GetGeneric(ctx, "secrets", "shop", "db")
	if err != nil {
		t.Fatal(err)
	}
	annotations := generic["metadata"].(map[string]any)["annotations"].(map[string]any)
	if annotations[lastAppliedAnnotation] != redactedValue {
		t.Fatalf("last-applied annotation = %v", annotations[lastAppliedAnnotation])
	}
}

func TestObjectYAMLByAPIVersion(t *testing.T) {
	deployment := kubetest.Object("apps/v1", "Deployment", "shop", "web", map[string]any{
		"spec":   map[string]any{"replicas": int64(2)},
		"status": map[string]any{"readyReplicas": int64(2)},
	})
	deployment.SetResourceVersion("77")
	deployment.SetAnnotations(map[string]string{"deployment.kubernetes.io/revision": "3"})
	rt, _ := editingRuntime(t, deployment)
	ctx := context.Background()
	ref := ObjectRef{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "shop", Name: "web"}

	full, err := rt.ObjectYAML(ctx, ref, false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(full, "apiVersion: apps/v1\nkind: Deployment\n") || !strings.Contains(full, "readyReplicas: 2") || !strings.Contains(full, `resourceVersion: "77"`) {
		t.Fatalf("full YAML:\n%s", full)
	}
	neat, err := rt.ObjectYAML(ctx, ref, true)
	if err != nil {
		t.Fatal(err)
	}
	for _, gone := range []string{"status", "resourceVersion", "revision"} {
		if strings.Contains(neat, gone) {
			t.Errorf("neat YAML still has %s:\n%s", gone, neat)
		}
	}
	if !strings.Contains(neat, "replicas: 2") {
		t.Fatalf("neat YAML lost the spec:\n%s", neat)
	}

	if _, err := rt.ObjectYAML(ctx, ObjectRef{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "shop", Name: "gone"}, false); !apierrors.IsNotFound(err) {
		t.Fatalf("a missing object: %v", err)
	}
}
