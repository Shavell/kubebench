package kube

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

func TestCreateNamespace(t *testing.T) {
	rt, cs := opsRuntime(t)
	ctx := context.Background()
	if err := rt.CreateNamespace(ctx, "team-a", map[string]string{"team": "a"}); err != nil {
		t.Fatal(err)
	}
	ns, err := cs.CoreV1().Namespaces().Get(ctx, "team-a", metav1.GetOptions{})
	if err != nil || ns.Labels["team"] != "a" {
		t.Fatalf("namespace = %+v, %v", ns, err)
	}
	if err := rt.CreateNamespace(ctx, "team-a", nil); !apierrors.IsAlreadyExists(err) {
		t.Fatalf("an existing namespace: %v", err)
	}

	before := len(writes(cs.Actions()))
	for _, bad := range []string{"", "Team-A", "team_a", "-team", "a.b", string(make([]byte, 64))} {
		expectOpError(t, rt.CreateNamespace(ctx, bad, nil), OpInvalid)
	}
	expectOpError(t, rt.CreateNamespace(ctx, "team-b", map[string]string{"bad key": "x"}), OpInvalid)
	expectOpError(t, rt.CreateNamespace(ctx, "team-b", map[string]string{"team": "not valid!"}), OpInvalid)
	if len(writes(cs.Actions())) != before {
		t.Fatal("an invalid namespace reached the cluster")
	}
}

func TestDeleteNamespaceProtection(t *testing.T) {
	objects := []runtime.Object{}
	for _, name := range append([]string{"kubebench", "team-a"}, SystemNamespaces...) {
		objects = append(objects, &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name}})
	}
	rt, cs := opsRuntime(t, objects...)
	ctx := context.Background()
	exists := func(name string) bool {
		_, err := cs.CoreV1().Namespaces().Get(ctx, name, metav1.GetOptions{})
		return err == nil
	}

	for _, name := range append([]string{"kubebench"}, SystemNamespaces...) {
		expectOpError(t, rt.DeleteNamespace(ctx, name, "kubebench", false), OpProtected)
	}
	if len(writes(cs.Actions())) != 0 {
		t.Fatal("a protected namespace reached the cluster")
	}
	if err := rt.DeleteNamespace(ctx, "team-a", "kubebench", false); err != nil || exists("team-a") {
		t.Fatalf("delete: %v", err)
	}
	// Forced, and without a known own namespace.
	if err := rt.DeleteNamespace(ctx, "kubebench", "kubebench", true); err != nil || exists("kubebench") {
		t.Fatalf("forced delete: %v", err)
	}
	expectOpError(t, rt.DeleteNamespace(ctx, "", "", false), OpInvalid)
}
