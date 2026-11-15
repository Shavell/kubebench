package kube

import (
	"context"
	"reflect"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube/kubetest"
)

func TestDeleteObjectPropagation(t *testing.T) {
	grace := int64(0)
	for _, tc := range []struct {
		name string
		opts DeleteOptions
		want metav1.DeleteOptions
	}{
		{"server default", DeleteOptions{}, metav1.DeleteOptions{}},
		{"background", DeleteOptions{Propagation: metav1.DeletePropagationBackground}, metav1.DeleteOptions{PropagationPolicy: new(metav1.DeletePropagationBackground)}},
		{"foreground", DeleteOptions{Propagation: metav1.DeletePropagationForeground}, metav1.DeleteOptions{PropagationPolicy: new(metav1.DeletePropagationForeground)}},
		{"orphan, now", DeleteOptions{Propagation: metav1.DeletePropagationOrphan, GracePeriod: &grace},
			metav1.DeleteOptions{PropagationPolicy: new(metav1.DeletePropagationOrphan), GracePeriodSeconds: &grace}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			rt, dyn := editingRuntime(t, kubetest.Object("apps/v1", "Deployment", "shop", "web", nil))
			obj, err := rt.ResolveObject("apps/v1", "Deployment", "shop", "web")
			if err != nil {
				t.Fatal(err)
			}
			if err := rt.DeleteObject(context.Background(), obj, tc.opts); err != nil {
				t.Fatal(err)
			}
			var deletes []k8stesting.DeleteActionImpl
			for _, a := range dyn.Actions() {
				if d, ok := a.(k8stesting.DeleteActionImpl); ok {
					deletes = append(deletes, d)
				}
			}
			if len(deletes) != 1 || deletes[0].GetNamespace() != "shop" || deletes[0].GetName() != "web" {
				t.Fatalf("delete requests = %+v", deletes)
			}
			if !reflect.DeepEqual(deletes[0].DeleteOptions, tc.want) {
				t.Fatalf("options = %+v, want %+v", deletes[0].DeleteOptions, tc.want)
			}
			if _, err := dyn.Resource(fakeKind(t, "Deployment").GVR()).Namespace("shop").Get(context.Background(), "web", metav1.GetOptions{}); !apierrors.IsNotFound(err) {
				t.Fatalf("the Deployment is still there: %v", err)
			}
		})
	}
}

func TestPatchMetadata(t *testing.T) {
	existing := kubetest.Object("v1", "ConfigMap", "shop", "app", nil)
	existing.SetLabels(map[string]string{"app": "shop", "tier": "web"})
	existing.SetAnnotations(map[string]string{"note": "old", "owner": "team-a"})
	rt, dyn := editingRuntime(t, existing)
	obj, err := rt.ResolveObject("v1", "ConfigMap", "shop", "app")
	if err != nil {
		t.Fatal(err)
	}
	patch := MetadataPatch{
		Labels:      map[string]*string{"tier": nil, "release": new("canary")},
		Annotations: map[string]*string{"note": new("new")},
	}
	if err := rt.PatchMetadata(context.Background(), obj, patch); err != nil {
		t.Fatal(err)
	}
	sent := patchRequests(dyn)
	if len(sent) != 1 || sent[0].GetPatchType() != types.MergePatchType {
		t.Fatalf("patch requests = %+v", sent)
	}
	if body := string(sent[0].GetPatch()); body != `{"metadata":{"labels":{"release":"canary","tier":null},"annotations":{"note":"new"}}}` {
		t.Fatalf("patch body = %s", body)
	}
	got := getFake(t, dyn, fakeKind(t, "ConfigMap"), "shop", "app")
	if !reflect.DeepEqual(got.GetLabels(), map[string]string{"app": "shop", "release": "canary"}) {
		t.Fatalf("labels = %v", got.GetLabels())
	}
	if !reflect.DeepEqual(got.GetAnnotations(), map[string]string{"note": "new", "owner": "team-a"}) {
		t.Fatalf("annotations = %v", got.GetAnnotations())
	}
}
