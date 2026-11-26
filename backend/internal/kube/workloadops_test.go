package kube

import (
	"context"
	"slices"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube/kubetest"
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
		if err := rt.ScaleWorkload(ctx, "", kind, "shop", name, 4); err != nil {
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
	expectOpError(t, rt.ScaleWorkload(ctx, "", "DaemonSet", "shop", "agent", 2), OpInvalid)
	expectOpError(t, rt.ScaleWorkload(ctx, "", "StatefulSet", "shop", "db", -1), OpInvalid)
	if len(dyn.Actions()) != before {
		t.Fatal("an invalid scale reached the cluster")
	}
}

// With an apiVersion any kind whose API has a scale subresource scales,
// found through discovery: a custom resource like an Argo Rollout.
func TestScaleWorkloadDiscoversTheScaleSubresource(t *testing.T) {
	rollouts := kubetest.Kind{GVK: schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Rollout"}, Resource: "rollouts", Namespaced: true}
	widgets := kubetest.Kind{GVK: schema.GroupVersionKind{Group: "argoproj.io", Version: "v1alpha1", Kind: "Widget"}, Resource: "widgets", Namespaced: true}
	daemonSets := kubetest.Kind{GVK: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "DaemonSet"}, Resource: "daemonsets", Namespaced: true}
	statefulSets := kubetest.Kind{GVK: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "StatefulSet"}, Resource: "statefulsets", Namespaced: true}
	kinds := []kubetest.Kind{rollouts, widgets, daemonSets, statefulSets}
	resources := kubetest.Resources(kinds)
	for _, list := range resources {
		for _, scaled := range []string{"rollouts", "statefulsets"} {
			if slices.ContainsFunc(list.APIResources, func(r metav1.APIResource) bool { return r.Name == scaled }) {
				list.APIResources = append(list.APIResources, metav1.APIResource{Name: scaled + "/scale", Kind: "Scale", Namespaced: true, Verbs: metav1.Verbs{"get", "patch", "update"}})
			}
		}
	}
	cs := fake.NewClientset()
	cs.Fake.Resources = resources
	object := func(k kubetest.Kind, name string) *unstructured.Unstructured {
		return kubetest.Object(k.GVK.GroupVersion().String(), k.GVK.Kind, "shop", name, map[string]any{"spec": map[string]any{"replicas": int64(1)}})
	}
	dyn := kubetest.NewDynamic(kinds, object(rollouts, "canary"), object(widgets, "w"), object(daemonSets, "agent"), object(statefulSets, "db"))
	rt := NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, dyn)
	ctx := context.Background()

	for _, k := range []kubetest.Kind{rollouts, statefulSets} {
		name := map[string]string{"Rollout": "canary", "StatefulSet": "db"}[k.GVK.Kind]
		if err := rt.ScaleWorkload(ctx, k.GVK.GroupVersion().String(), k.GVK.Kind, "shop", name, 5); err != nil {
			t.Fatalf("%s: %v", k.GVK.Kind, err)
		}
		obj, err := dyn.Resource(k.GVR()).Namespace("shop").Get(ctx, name, metav1.GetOptions{})
		if n, _, _ := unstructured.NestedInt64(obj.Object, "spec", "replicas"); err != nil || n != 5 {
			t.Fatalf("%s replicas = %d, %v", k.GVK.Kind, n, err)
		}
	}
	patches := 0
	for _, a := range dyn.Actions() {
		if patch, ok := a.(k8stesting.PatchAction); ok {
			patches++
			if patch.GetSubresource() != "scale" {
				t.Fatalf("scaled through %q instead of the scale subresource", patch.GetSubresource())
			}
		}
	}
	if patches != 2 {
		t.Fatalf("%d patches, want 2", patches)
	}

	// Kinds without the subresource, and unknown kinds, are refused before anything is sent.
	before := len(dyn.Actions())
	expectOpError(t, rt.ScaleWorkload(ctx, "argoproj.io/v1alpha1", "Widget", "shop", "w", 2), OpInvalid)
	expectOpError(t, rt.ScaleWorkload(ctx, "apps/v1", "DaemonSet", "shop", "agent", 2), OpInvalid)
	if err := rt.ScaleWorkload(ctx, "example.com/v1", "Gadget", "shop", "g", 2); err == nil {
		t.Fatal("an unknown kind was scaled")
	}
	if len(dyn.Actions()) != before {
		t.Fatal("a refused scale reached the cluster")
	}
}
