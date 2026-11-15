package storage

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/kube"
	"kubebench/internal/kube/kubetest"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// env is a fake cluster with the plugin's routes mounted on a mux, as the
// API server mounts them (without its gates, which internal/api tests).
type env struct {
	t      *testing.T
	cs     *fake.Clientset
	rt     *kube.Runtime
	mux    *http.ServeMux
	audits []string // "target detail" given to SetAudit
	access map[string]plugin.Access
}

var snapshotKind = kubetest.Kind{
	GVK:      schema.GroupVersionKind{Group: snapshotGroup, Version: "v1", Kind: "VolumeSnapshot"},
	Resource: "volumesnapshots", Namespaced: true,
}

type envOptions struct {
	snapshots []*unstructured.Unstructured
	// withSnapshotCRD serves snapshot.storage.k8s.io.
	withSnapshotCRD bool
}

func newEnv(t *testing.T, opts envOptions, objects ...runtime.Object) *env {
	t.Helper()
	cs := fake.NewClientset(objects...)
	var kinds []kubetest.Kind
	if opts.withSnapshotCRD {
		kinds = append(kinds, snapshotKind)
	}
	cs.Fake.Resources = kubetest.Resources(append([]kubetest.Kind{
		{GVK: schema.GroupVersionKind{Version: "v1", Kind: "PersistentVolumeClaim"}, Resource: "persistentvolumeclaims", Namespaced: true},
	}, kinds...))
	dyn := kubetest.NewDynamic(slices.Concat(kubetest.Kinds, []kubetest.Kind{snapshotKind}), opts.snapshots...)
	e := &env{
		t: t, cs: cs, rt: kube.NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, dyn),
		mux: http.NewServeMux(), access: map[string]plugin.Access{},
	}
	(&Plugin{}).Routes(e)
	return e
}

// Handle implements plugin.Router.
func (e *env) Handle(method, path string, a plugin.Access, h plugin.HandlerFunc) {
	e.access[method+" "+path] = a
	e.mux.HandleFunc(method+" "+path, func(w http.ResponseWriter, r *http.Request) {
		h(w, r, &plugin.Context{Runtime: e.rt, SetAudit: func(target, detail string) {
			e.audits = append(e.audits, target+" "+detail)
		}})
	})
}

func (e *env) do(method, path string, body any) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if s, ok := body.(string); ok {
			buf.WriteString(s)
		} else if err := json.NewEncoder(&buf).Encode(body); err != nil {
			e.t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, httptest.NewRequest(method, path, &buf))
	return rec
}

// expect checks the status and, for errors, the code; it returns the body.
func expect(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) []byte {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d; body %s", rec.Code, status, rec.Body)
	}
	if code != "" {
		var e plugin.ErrorBody
		if err := json.Unmarshal(rec.Body.Bytes(), &e); err != nil || e.Code != code {
			t.Fatalf("code = %q, want %q; body %s", e.Code, code, rec.Body)
		}
	}
	return rec.Body.Bytes()
}

func decode[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decode %s: %v", body, err)
	}
	return v
}

// --- fixtures ----------------------------------------------------------------

func ptr[T any](v T) *T { return &v }

func class(name, provisioner string, expandable, isDefault bool) *storagev1.StorageClass {
	sc := &storagev1.StorageClass{
		ObjectMeta:  metav1.ObjectMeta{Name: name},
		Provisioner: provisioner, AllowVolumeExpansion: ptr(expandable),
	}
	if isDefault {
		sc.Annotations = map[string]string{defaultClassAnnotation: "true"}
	}
	return sc
}

func claim(ns, name, class, size string, phase corev1.PersistentVolumeClaimPhase) *corev1.PersistentVolumeClaim {
	pvc := &corev1.PersistentVolumeClaim{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name, ResourceVersion: "7"},
		Spec: corev1.PersistentVolumeClaimSpec{
			AccessModes: []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce},
			Resources: corev1.VolumeResourceRequirements{
				Requests: corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
			},
		},
		Status: corev1.PersistentVolumeClaimStatus{Phase: phase},
	}
	if class != "-" {
		pvc.Spec.StorageClassName = ptr(class)
	}
	if phase == corev1.ClaimBound {
		pvc.Status.Capacity = corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)}
	}
	return pvc
}

func volume(name, class, size string, phase corev1.PersistentVolumePhase) *corev1.PersistentVolume {
	return &corev1.PersistentVolume{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		Spec: corev1.PersistentVolumeSpec{
			Capacity:                      corev1.ResourceList{corev1.ResourceStorage: resource.MustParse(size)},
			AccessModes:                   []corev1.PersistentVolumeAccessMode{corev1.ReadWriteOnce, corev1.ReadOnlyMany},
			PersistentVolumeReclaimPolicy: corev1.PersistentVolumeReclaimRetain,
			StorageClassName:              class,
			PersistentVolumeSource: corev1.PersistentVolumeSource{
				HostPath: &corev1.HostPathVolumeSource{Path: "/data/" + name},
			},
		},
		Status: corev1.PersistentVolumeStatus{Phase: phase},
	}
}

func node(name string) *corev1.Node {
	return &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: name}}
}

func snapshot(ns, name, pvc, restoreSize string, ready bool) *unstructured.Unstructured {
	return kubetest.Object(snapshotGroup+"/v1", "VolumeSnapshot", ns, name, map[string]any{
		"spec":   map[string]any{"source": map[string]any{"persistentVolumeClaimName": pvc}, "volumeSnapshotClassName": "csi-hostpath-snapclass"},
		"status": map[string]any{"readyToUse": ready, "restoreSize": restoreSize},
	})
}
