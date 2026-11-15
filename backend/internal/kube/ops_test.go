package kube

import (
	"errors"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/store"
)

// opsRuntime is a Runtime around a fake clientset holding objects.
func opsRuntime(t *testing.T, objects ...runtime.Object) (*Runtime, *fake.Clientset) {
	t.Helper()
	cs := fake.NewClientset(objects...)
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	return NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, dyn), cs
}

// writes lists the requests that change state.
func writes(actions []k8stesting.Action) []k8stesting.Action {
	var out []k8stesting.Action
	for _, a := range actions {
		switch a.GetVerb() {
		case "get", "list", "watch":
		default:
			out = append(out, a)
		}
	}
	return out
}

// expectOpError fails unless err is an *OpError of kind.
func expectOpError(t *testing.T, err error, kind OpErrorKind) {
	t.Helper()
	var op *OpError
	if !errors.As(err, &op) || op.Kind != kind {
		t.Fatalf("error = %v, want an OpError of kind %d", err, kind)
	}
}
