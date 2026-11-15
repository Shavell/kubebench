package kube

import (
	"context"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube/kubetest"
	"kubebench/internal/store"
)

// editingRuntime is a runtime on a fake cluster that serves
// kubetest.Kinds, with server-side apply semantics.
func editingRuntime(t *testing.T, objects ...*unstructured.Unstructured) (*Runtime, *dynamicfake.FakeDynamicClient) {
	t.Helper()
	cs := fake.NewClientset()
	cs.Fake.Resources = kubetest.Resources(kubetest.Kinds)
	dyn := kubetest.NewDynamic(kubetest.Kinds, objects...)
	return NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, dyn), dyn
}

func mustParseManifests(t *testing.T, text string) []*unstructured.Unstructured {
	t.Helper()
	objects, err := ParseManifests([]byte(text))
	if err != nil {
		t.Fatal(err)
	}
	return objects
}

// patches lists the patch requests that reached the fake cluster.
func patchRequests(dyn *dynamicfake.FakeDynamicClient) []k8stesting.PatchActionImpl {
	var out []k8stesting.PatchActionImpl
	for _, a := range dyn.Actions() {
		if p, ok := a.(k8stesting.PatchActionImpl); ok {
			out = append(out, p)
		}
	}
	return out
}

func getFake(t *testing.T, dyn *dynamicfake.FakeDynamicClient, k kubetest.Kind, namespace, name string) *unstructured.Unstructured {
	t.Helper()
	obj, err := dyn.Resource(k.GVR()).Namespace(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatalf("get %s %s/%s: %v", k.GVK.Kind, namespace, name, err)
	}
	return obj
}

func fakeKind(t *testing.T, kind string) kubetest.Kind {
	t.Helper()
	for _, k := range kubetest.Kinds {
		if k.GVK.Kind == kind {
			return k
		}
	}
	t.Fatalf("kubetest serves no %s", kind)
	return kubetest.Kind{}
}
