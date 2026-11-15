package kube

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/store"
)

func TestRestartWorkload(t *testing.T) {
	meta := func(name string) metav1.ObjectMeta { return metav1.ObjectMeta{Name: name, Namespace: "shop"} }
	rt, cs := opsRuntime(t,
		&appsv1.Deployment{ObjectMeta: meta("web")},
		&appsv1.StatefulSet{ObjectMeta: meta("db")},
		&appsv1.DaemonSet{ObjectMeta: meta("agent")},
	)
	ctx := context.Background()
	before := time.Now().Add(-time.Second)
	for kind, name := range map[string]string{"Deployment": "web", "StatefulSet": "db", "DaemonSet": "agent"} {
		if err := rt.RestartWorkload(ctx, kind, "shop", name); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
	}
	stamps := map[string]string{}
	d, _ := cs.AppsV1().Deployments("shop").Get(ctx, "web", metav1.GetOptions{})
	stamps["Deployment"] = d.Spec.Template.Annotations[restartedAtAnnotation]
	s, _ := cs.AppsV1().StatefulSets("shop").Get(ctx, "db", metav1.GetOptions{})
	stamps["StatefulSet"] = s.Spec.Template.Annotations[restartedAtAnnotation]
	ds, _ := cs.AppsV1().DaemonSets("shop").Get(ctx, "agent", metav1.GetOptions{})
	stamps["DaemonSet"] = ds.Spec.Template.Annotations[restartedAtAnnotation]
	for kind, stamp := range stamps {
		at, err := time.Parse(time.RFC3339, stamp)
		if err != nil || at.Before(before) {
			t.Errorf("%s restartedAt = %q", kind, stamp)
		}
	}

	expectOpError(t, rt.RestartWorkload(ctx, "ReplicaSet", "shop", "web"), OpInvalid)
	expectOpError(t, rt.RestartWorkload(ctx, "", "shop", "web"), OpInvalid)
}

func TestScaleWorkloadUsesTheScaleSubresource(t *testing.T) {
	scalable := func(kind, name string) *unstructured.Unstructured {
		return &unstructured.Unstructured{Object: map[string]any{
			"apiVersion": "apps/v1", "kind": kind,
			"metadata": map[string]any{"name": name, "namespace": "shop"},
			"spec":     map[string]any{"replicas": int64(1)},
		}}
	}
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(),
		scalable("StatefulSet", "db"), scalable("ReplicaSet", "web-abc"), scalable("Deployment", "web"))
	rt := NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, fake.NewClientset(), dyn)
	ctx := context.Background()

	for kind, name := range map[string]string{"StatefulSet": "db", "ReplicaSet": "web-abc", "Deployment": "web"} {
		if err := rt.ScaleWorkload(ctx, kind, "shop", name, 4); err != nil {
			t.Fatalf("%s: %v", kind, err)
		}
		obj, err := dyn.Resource(scalableKinds[kind]).Namespace("shop").Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if n, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas"); n != 4 {
			t.Fatalf("%s replicas = %d", kind, n)
		}
	}
	for _, a := range dyn.Actions() {
		if patch, ok := a.(k8stesting.PatchAction); ok && patch.GetSubresource() != "scale" {
			t.Fatalf("scaled through %q instead of the scale subresource", patch.GetSubresource())
		}
	}

	before := len(dyn.Actions())
	expectOpError(t, rt.ScaleWorkload(ctx, "DaemonSet", "shop", "agent", 2), OpInvalid)
	expectOpError(t, rt.ScaleWorkload(ctx, "StatefulSet", "shop", "db", -1), OpInvalid)
	if len(dyn.Actions()) != before {
		t.Fatal("an invalid scale reached the cluster")
	}
}
