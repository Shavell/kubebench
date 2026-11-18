package kube

import (
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/cloud"
	"kubebench/internal/store"
)

func clusterVersion(history ...map[string]any) *unstructured.Unstructured {
	items := make([]any, len(history))
	for i, h := range history {
		items[i] = h
	}
	return &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": "config.openshift.io/v1", "kind": "ClusterVersion",
		"metadata": map[string]any{"name": "version"},
		"status": map[string]any{
			"desired": map[string]any{"version": "5.0.1"},
			"history": items,
		},
	}}
}

// OpenShift is recognised by its ClusterVersion, also when the credential
// may not read nodes, and its version is the last completed update.
func TestDetectCloudOpenShift(t *testing.T) {
	cs := fake.NewClientset()
	cs.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "v1.36.5"}
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme(), clusterVersion(
		map[string]any{"state": "Partial", "version": "5.0.1"},
		map[string]any{"state": "Completed", "version": "5.0.0"},
	))
	rt := NewRuntimeForTest(store.Cluster{ID: 1, Name: "crc"}, cs, dyn)
	rt.DetectCloudForTest()
	d, _ := rt.Cloud()
	want := cloud.Detection{Provider: cloud.ProviderOpenShift, Version: "v1.36.5", ProductVersion: "5.0.0"}
	if d != want {
		t.Fatalf("detection = %+v, want %+v", d, want)
	}
}

// A plain cluster answers 404 for the ClusterVersion: no product version.
func TestDetectCloudWithoutClusterVersion(t *testing.T) {
	cs := fake.NewClientset()
	cs.Discovery().(*fakediscovery.FakeDiscovery).FakedServerVersion = &version.Info{GitVersion: "v1.35.5+k3s1"}
	rt := NewRuntimeForTest(store.Cluster{ID: 1}, cs, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme()))
	rt.DetectCloudForTest()
	if d, _ := rt.Cloud(); d.Provider != cloud.ProviderK3s || d.ProductVersion != "" {
		t.Fatalf("detection = %+v", d)
	}
}

func TestServedOptional(t *testing.T) {
	cs := fake.NewClientset()
	disco := cs.Discovery().(*fakediscovery.FakeDiscovery)
	if got := servedOptional(disco); len(got) != 0 {
		t.Fatalf("plain cluster serves %v", got)
	}
	disco.Resources = []*metav1.APIResourceList{
		{GroupVersion: "route.openshift.io/v1"},
		{GroupVersion: "image.openshift.io/v1"},
		{GroupVersion: "apps/v1"},
	}
	got := servedOptional(disco)
	if want := []string{"routes", "imagestreams", "imagestreamtags"}; !slices.Equal(got, want) {
		t.Fatalf("served = %v, want %v", got, want)
	}
	for _, key := range OptionalResources {
		if _, ok := Catalog[key]; !ok {
			t.Errorf("optional resource %q is not in the catalog", key)
		}
	}
}
