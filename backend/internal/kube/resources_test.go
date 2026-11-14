package kube

import (
	"context"
	"strings"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/store"
)

// Typed clients return objects with an empty TypeMeta; the YAML view must
// still be a complete manifest, without managedFields.
func TestObjectYAMLIsACompleteManifest(t *testing.T) {
	managed := []metav1.ManagedFieldsEntry{{Manager: "kubectl"}}
	cs := fake.NewClientset(
		&corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: "web-1", Namespace: "shop", ManagedFields: managed}},
		&appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: "web", Namespace: "shop", ManagedFields: managed}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "node-1"}},
	)
	rt := NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()))

	cases := []struct{ kind, namespace, name, header string }{
		{"Pod", "shop", "web-1", "apiVersion: v1\nkind: Pod\n"},
		{"Deployment", "shop", "web", "apiVersion: apps/v1\nkind: Deployment\n"},
		{"Node", "", "node-1", "apiVersion: v1\nkind: Node\n"},
	}
	for _, c := range cases {
		out, err := rt.ObjectYAML(context.Background(), c.kind, c.namespace, c.name)
		if err != nil {
			t.Fatalf("%s: %v", c.kind, err)
		}
		if !strings.HasPrefix(out, c.header) {
			t.Errorf("%s YAML starts with %q, want %q", c.kind, firstLines(out, 2), c.header)
		}
		if strings.Contains(out, "managedFields") {
			t.Errorf("%s YAML still has managedFields", c.kind)
		}
	}
}

func firstLines(s string, n int) string {
	lines := strings.SplitN(s, "\n", n+1)
	return strings.Join(lines[:min(n, len(lines))], "\n")
}
