// Package kubetest fakes a cluster for tests of the generic object code
// (apply, delete, metadata patches, YAML): a dynamic client whose object
// tracker handles server-side apply like the API server's field manager
// (ownership, conflicts, force) and the discovery documents that map its
// kinds. Only tests import it.
package kubetest

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/apimachinery/pkg/util/managedfields"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"
)

// Kind is a kind the fake cluster serves.
type Kind struct {
	GVK        schema.GroupVersionKind
	Resource   string
	Namespaced bool
}

// GVR is the kind's resource.
func (k Kind) GVR() schema.GroupVersionResource { return k.GVK.GroupVersion().WithResource(k.Resource) }

// Kinds is a representative set: core, grouped and cluster-scoped kinds.
var Kinds = []Kind{
	{GVK: schema.GroupVersionKind{Version: "v1", Kind: "ConfigMap"}, Resource: "configmaps", Namespaced: true},
	{GVK: schema.GroupVersionKind{Version: "v1", Kind: "Secret"}, Resource: "secrets", Namespaced: true},
	{GVK: schema.GroupVersionKind{Version: "v1", Kind: "Namespace"}, Resource: "namespaces"},
	{GVK: schema.GroupVersionKind{Group: "apps", Version: "v1", Kind: "Deployment"}, Resource: "deployments", Namespaced: true},
	{GVK: schema.GroupVersionKind{Group: "rbac.authorization.k8s.io", Version: "v1", Kind: "ClusterRole"}, Resource: "clusterroles"},
}

// Resources is the discovery content serving kinds, for
// fakediscovery.FakeDiscovery.Resources (a fake clientset's
// Fake.Resources).
func Resources(kinds []Kind) []*metav1.APIResourceList {
	var lists []*metav1.APIResourceList
	byGroupVersion := map[string]*metav1.APIResourceList{}
	for _, k := range kinds {
		gv := k.GVK.GroupVersion().String()
		list, ok := byGroupVersion[gv]
		if !ok {
			list = &metav1.APIResourceList{GroupVersion: gv}
			byGroupVersion[gv] = list
			lists = append(lists, list)
		}
		list.APIResources = append(list.APIResources, metav1.APIResource{
			Name: k.Resource, Kind: k.GVK.Kind, Namespaced: k.Namespaced,
			Verbs: metav1.Verbs{"get", "list", "create", "update", "patch", "delete"},
		})
	}
	return lists
}

// NewDynamic is a fake dynamic client serving kinds and holding objects.
// Its server-side apply records field ownership per manager and reports
// conflicts as the API server does. Like every client-go fake it ignores
// dry runs: a dry-run request changes the fake's state too.
func NewDynamic(kinds []Kind, objects ...*unstructured.Unstructured) *dynamicfake.FakeDynamicClient {
	scheme := runtime.NewScheme()
	listKinds := map[schema.GroupVersionResource]string{}
	for _, k := range kinds {
		scheme.AddKnownTypeWithName(k.GVK, &unstructured.Unstructured{})
		scheme.AddKnownTypeWithName(k.GVK.GroupVersion().WithKind(k.GVK.Kind+"List"), &unstructured.UnstructuredList{})
		listKinds[k.GVR()] = k.GVK.Kind + "List"
	}
	tracker := k8stesting.NewFieldManagedObjectTracker(scheme,
		serializer.NewCodecFactory(scheme).UniversalDecoder(), managedfields.NewDeducedTypeConverter())
	for _, obj := range objects {
		if err := tracker.Add(obj); err != nil {
			panic(err)
		}
	}
	client := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(scheme, listKinds)
	client.PrependReactor("*", "*", k8stesting.ObjectReaction(tracker))
	return client
}

// Object builds an unstructured object; fields are merged in at the top
// level (e.g. {"data": …}).
func Object(apiVersion, kind, namespace, name string, fields map[string]any) *unstructured.Unstructured {
	obj := &unstructured.Unstructured{Object: map[string]any{}}
	for key, value := range fields {
		obj.Object[key] = value
	}
	obj.SetAPIVersion(apiVersion)
	obj.SetKind(kind)
	obj.SetNamespace(namespace)
	obj.SetName(name)
	return obj
}
