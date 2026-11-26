package kube

import (
	"errors"
	"slices"
	"strconv"
	"testing"
	"time"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	metafake "k8s.io/client-go/metadata/fake"
	k8stesting "k8s.io/client-go/testing"
)

func fakeCRDMeta(objects ...runtime.Object) *metafake.FakeMetadataClient {
	scheme := metafake.NewTestScheme()
	_ = metav1.AddMetaToScheme(scheme)
	return metafake.NewSimpleMetadataClient(scheme, objects...)
}

func crdMeta(name, resourceVersion string) *metav1.PartialObjectMetadata {
	return &metav1.PartialObjectMetadata{
		TypeMeta:   metav1.TypeMeta{APIVersion: "apiextensions.k8s.io/v1", Kind: "CustomResourceDefinition"},
		ObjectMeta: metav1.ObjectMeta{Name: name, ResourceVersion: resourceVersion},
	}
}

// A version removed from a CRD is no miss (the group still resolves), so
// only the CRD watch makes discovery forget it.
func TestCRDChangeInvalidatesDiscovery(t *testing.T) {
	rt, disc := testRuntime(t)
	defer rt.Close()
	widgets := func(version string) *metav1.APIResourceList {
		return &metav1.APIResourceList{
			GroupVersion: "example.com/" + version,
			APIResources: []metav1.APIResource{{Name: "widgets", Kind: "Widget", Namespaced: true}},
		}
	}
	disc.Resources = append(disc.Resources, widgets("v2"), widgets("v1"))
	if gvr, _, err := rt.ResolveResource("example.com", "widgets"); err != nil || gvr.Version != "v2" {
		t.Fatalf("before: %v, %v", gvr, err)
	}
	// v2 is removed; the cached discovery still has it.
	disc.Resources = slices.DeleteFunc(disc.Resources, func(l *metav1.APIResourceList) bool {
		return l.GroupVersion == "example.com/v2"
	})
	if gvr, _, _ := rt.ResolveResource("example.com", "widgets"); gvr.Version != "v2" {
		t.Fatalf("discovery was refreshed without a CRD change: %v", gvr)
	}

	m := fakeCRDMeta(crdMeta("widgets.example.com", "1"))
	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		rt.watchCRDs(stop, m)
	}()
	defer func() {
		close(stop)
		<-done
	}()
	deadline := time.Now().Add(5 * time.Second)
	for rv := 2; ; rv++ {
		// An update before the informer synced counts as initial state:
		// update until one is seen.
		if _, err := m.Resource(crdGVR).(metafake.MetadataClient).UpdateFake(
			crdMeta("widgets.example.com", strconv.Itoa(rv)), metav1.UpdateOptions{}); err != nil {
			t.Fatal(err)
		}
		time.Sleep(20 * time.Millisecond)
		if gvr, _, err := rt.ResolveResource("example.com", "widgets"); err == nil && gvr.Version == "v1" {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("the CRD change did not refresh discovery")
		}
	}
}

// Without access to CRDs the watch gives up at once.
func TestCRDWatchDeniedStops(t *testing.T) {
	rt, _ := testRuntime(t)
	defer rt.Close()
	m := fakeCRDMeta()
	m.PrependReactor("list", "customresourcedefinitions", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewForbidden(schema.GroupResource{Resource: "customresourcedefinitions"}, "", errors.New("RBAC"))
	})
	done := make(chan struct{})
	go func() {
		defer close(done)
		rt.watchCRDs(make(chan struct{}), m)
	}()
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the denied CRD watch kept running")
	}
}
