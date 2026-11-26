package watch

import (
	"context"
	"slices"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	k8sruntime "k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"
)

var secretsGVR = schema.GroupVersionResource{Version: "v1", Resource: "secrets"}

// Secrets (like every resource) are watched as metadata only: the whole
// objects, with their values, are never listed or watched.
func TestInformersWatchMetadataOnly(t *testing.T) {
	m := newFakeMeta(object(secretsGVR, "Secret", "shop", "db"))
	dyn := newFakeDynamic()
	dyn.PrependReactor("*", "*", func(a k8stesting.Action) (bool, k8sruntime.Object, error) {
		t.Errorf("whole objects requested: %s %s", a.GetVerb(), a.GetResource().Resource)
		return false, nil, nil
	})
	resolve := func(topic string) (schema.GroupVersionResource, error) {
		if topic == "secrets" {
			return secretsGVR, nil
		}
		return schema.GroupVersionResource{}, &NotServedError{Topic: topic}
	}
	h := NewHub(m, dyn, resolve, Options{Logf: func(string, ...any) {}})
	t.Cleanup(h.Close)
	sub := subscribe(t, h, "secrets")
	waitStatus(t, sub, "secrets", ModeLive)

	create(t, m, secretsGVR, object(secretsGVR, "Secret", "shop", "api"))
	if got := waitChanges(t, sub, 1); !slices.Equal(got, []Change{{Topic: "secrets", Namespace: "shop"}}) {
		t.Fatalf("changes = %v", got)
	}
	var verbs []string
	for _, a := range m.Actions() {
		verbs = append(verbs, a.GetVerb())
	}
	if !slices.Contains(verbs, "list") || !slices.Contains(verbs, "watch") {
		t.Fatalf("metadata requests = %v", verbs)
	}
}

// An API that cannot list metadata only (406) is watched as whole
// objects, which the cache strips.
func TestInformerFallsBackToWholeObjects(t *testing.T) {
	m := newFakeMeta()
	m.PrependReactor("list", "pods", func(k8stesting.Action) (bool, k8sruntime.Object, error) {
		return true, nil, apierrors.NewGenericServerResponse(406, "list", schema.GroupResource{Resource: "pods"}, "", "no metadata", 0, false)
	})
	dyn := newFakeDynamic()
	h := NewHub(m, dyn, testResolver, Options{Logf: func(string, ...any) {}})
	t.Cleanup(h.Close)
	sub := subscribe(t, h, "pods")
	waitStatus(t, sub, "pods", ModeLive)
	pod := &unstructured.Unstructured{}
	pod.SetAPIVersion("v1")
	pod.SetKind("Pod")
	pod.SetNamespace("shop")
	pod.SetName("web")
	if _, err := dyn.Resource(podsGVR).Namespace("shop").Create(context.Background(), pod, metav1.CreateOptions{}); err != nil {
		t.Fatal(err)
	}
	if got := waitChanges(t, sub, 1); !slices.Equal(got, []Change{{Topic: "pods", Namespace: "shop"}}) {
		t.Fatalf("changes = %v", got)
	}
}

// Without a dynamic client there is no fallback: the topic is polled.
func TestNoMetadataWithoutFallbackIsPolled(t *testing.T) {
	m := newFakeMeta()
	m.PrependReactor("list", "pods", func(k8stesting.Action) (bool, k8sruntime.Object, error) {
		return true, nil, apierrors.NewGenericServerResponse(406, "list", schema.GroupResource{Resource: "pods"}, "", "no metadata", 0, false)
	})
	h, _ := newTestHub(t, m, Options{})
	sub := subscribe(t, h, "pods")
	if st := waitStatus(t, sub, "pods", ModePolling); st.Reason == "" {
		t.Fatal("no reason")
	}
}

// The cache keeps names and versions only: no labels, annotations (a
// Secret's last-applied-configuration holds its values) or managed fields.
func TestStripObject(t *testing.T) {
	in := &metav1.PartialObjectMetadata{
		TypeMeta: metav1.TypeMeta{APIVersion: "v1", Kind: "Secret"},
		ObjectMeta: metav1.ObjectMeta{
			Name: "db", Namespace: "shop", UID: "u1", ResourceVersion: "7",
			Labels:        map[string]string{"app": "db"},
			Annotations:   map[string]string{"kubectl.kubernetes.io/last-applied-configuration": `{"data":{"password":"c2VjcmV0"}}`},
			ManagedFields: []metav1.ManagedFieldsEntry{{Manager: "kubectl"}},
		},
	}
	out, err := stripObject(in)
	if err != nil {
		t.Fatal(err)
	}
	got, ok := out.(*metav1.PartialObjectMetadata)
	if !ok || got.Name != "db" || got.Namespace != "shop" || got.Annotations != nil || got.Labels != nil || got.ManagedFields != nil ||
		got.ResourceVersion != "7" || got.UID != "u1" || got.Kind != "Secret" {
		t.Fatalf("stripped = %+v", out)
	}
	u := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "v1", "kind": "Secret",
		"metadata": map[string]any{"name": "db", "namespace": "shop", "resourceVersion": "8"},
		"data":     map[string]any{"password": "c2VjcmV0"},
	}}
	if out, _ := stripObject(u); out.(*metav1.PartialObjectMetadata).ResourceVersion != "8" {
		t.Fatalf("stripped unstructured = %+v", out)
	}
	if out, err := stripObject("not an object"); err != nil || out != "not an object" {
		t.Fatalf("non-object: %v, %v", out, err)
	}
}
