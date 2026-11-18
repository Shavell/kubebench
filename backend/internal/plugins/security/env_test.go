package security

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube"
	"kubebench/internal/openshift"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// memKV is an in-memory plugin.KV.
type memKV struct {
	mu sync.Mutex
	m  map[string]string
}

func (kv *memKV) Get(key string) (string, bool, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	v, ok := kv.m[key]
	return v, ok, nil
}

func (kv *memKV) Set(key, value string) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	kv.m[key] = value
	return nil
}

func (kv *memKV) Delete(key string) error {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	delete(kv.m, key)
	return nil
}

func (kv *memKV) List(prefix string) (map[string]string, error) {
	kv.mu.Lock()
	defer kv.mu.Unlock()
	out := map[string]string{}
	for k, v := range kv.m {
		if strings.HasPrefix(k, prefix) {
			out[k] = v
		}
	}
	return out, nil
}

var testNow = time.Date(2026, 11, 14, 12, 0, 0, 0, time.UTC)

// env is a fake cluster with the plugin's routes mounted on a mux, as the
// API server mounts them (its gates are tested in internal/api).
type env struct {
	t      *testing.T
	cs     *fake.Clientset
	rt     *kube.Runtime
	p      *Plugin
	kv     *memKV
	mux    *http.ServeMux
	audits []string // "target detail" given to SetAudit
}

// newEnv is a plain Kubernetes cluster: the OpenShift APIs are not served.
func newEnv(t *testing.T, objects ...runtime.Object) *env {
	t.Helper()
	dyn := sccClient()
	dyn.PrependReactor("list", openshift.SCCGVR.Resource, func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewNotFound(schema.GroupResource{Group: openshift.SCCGVR.Group, Resource: openshift.SCCGVR.Resource}, "")
	})
	return newEnvWith(t, dyn, objects...)
}

// newOpenShiftEnv is an OpenShift cluster with the SCCs given.
func newOpenShiftEnv(t *testing.T, sccs []*unstructured.Unstructured, objects ...runtime.Object) *env {
	t.Helper()
	dyn := sccClient()
	for _, s := range sccs {
		// Created by resource: the fake would guess the plural of the kind wrong.
		if _, err := dyn.Resource(openshift.SCCGVR).Create(t.Context(), s, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	return newEnvWith(t, dyn, objects...)
}

// sccClient is a dynamic client that serves the SCC list.
func sccClient() *dynamicfake.FakeDynamicClient {
	return dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(),
		map[schema.GroupVersionResource]string{openshift.SCCGVR: "SecurityContextConstraintsList"})
}

func newEnvWith(t *testing.T, dyn *dynamicfake.FakeDynamicClient, objects ...runtime.Object) *env {
	cs := fake.NewClientset(objects...)
	e := &env{
		t: t, cs: cs, rt: kube.NewRuntimeForTest(store.Cluster{ID: 7, Name: "lab"}, cs, dyn),
		p: New(), kv: &memKV{m: map[string]string{}}, mux: http.NewServeMux(),
	}
	e.p.now = func() time.Time { return testNow }
	e.p.Routes(e)
	return e
}

// Handle implements plugin.Router.
func (e *env) Handle(method, path string, _ plugin.Access, h plugin.HandlerFunc) {
	e.mux.HandleFunc(method+" "+path, func(w http.ResponseWriter, r *http.Request) {
		h(w, r, &plugin.Context{
			Runtime: e.rt, User: &store.User{Username: "alice"}, KV: e.kv,
			SetAudit: func(target, detail string) { e.audits = append(e.audits, target+" "+detail) },
		})
	})
}

func (e *env) do(method, path string, body any) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			e.t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	e.mux.ServeHTTP(rec, httptest.NewRequest(method, path, &buf))
	return rec
}

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

func (e *env) scan(includeSystem bool) *Report {
	e.t.Helper()
	rep := decode[Report](e.t, expect(e.t, e.do(http.MethodPost, "/scan", map[string]bool{"includeSystem": includeSystem}), http.StatusOK, ""))
	return &rep
}

// --- fixtures ----------------------------------------------------------------

func namespace(name string, labels map[string]string) *corev1.Namespace {
	return &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}}
}

func meta(ns, name string) metav1.ObjectMeta { return metav1.ObjectMeta{Namespace: ns, Name: name} }

func owned(m metav1.ObjectMeta, kind, owner string) metav1.ObjectMeta {
	m.OwnerReferences = []metav1.OwnerReference{{Kind: kind, Name: owner, Controller: ptr(true)}}
	return m
}

func deployment(ns, name string, spec corev1.PodSpec) *appsv1.Deployment {
	return &appsv1.Deployment{ObjectMeta: meta(ns, name), Spec: appsv1.DeploymentSpec{Template: corev1.PodTemplateSpec{Spec: spec}}}
}

func insecureSpec() corev1.PodSpec {
	spec := corev1.PodSpec{
		HostNetwork: true,
		Containers: []corev1.Container{{
			Name: "app", Image: "nginx:latest",
			SecurityContext: &corev1.SecurityContext{Privileged: ptr(true)},
			Env:             []corev1.EnvVar{{Name: "DB_PASSWORD", ValueFrom: &corev1.EnvVarSource{SecretKeyRef: &corev1.SecretKeySelector{Key: "pw"}}}},
		}},
		Volumes: []corev1.Volume{{Name: "root", VolumeSource: corev1.VolumeSource{HostPath: &corev1.HostPathVolumeSource{Path: "/"}}}},
	}
	return spec
}

func clusterRoleBinding(name, role string, labels map[string]string, subjects ...rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels},
		RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: role},
		Subjects:   subjects,
	}
}

func clusterRole(name string, labels map[string]string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name, Labels: labels}, Rules: rules}
}

// lab is a small cluster: an insecure and a compliant Deployment (with the
// ReplicaSet and Pod each controls), a CronJob with its Job, a bare Pod,
// system workloads and RBAC objects of the cluster, an add-on and a user.
func lab() []runtime.Object {
	bootstrap := map[string]string{"kubernetes.io/bootstrapping": "rbac-defaults"}
	all := rbacv1.PolicyRule{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}
	cronSpec := compliantSpec()
	return []runtime.Object{
		namespace("shop", map[string]string{psaEnforceLabel: "baseline"}),
		namespace("kube-system", nil),
		deployment("shop", "insecure", insecureSpec()),
		deployment("shop", "compliant", compliantSpec()),
		&appsv1.ReplicaSet{ObjectMeta: owned(meta("shop", "insecure-5d8"), "Deployment", "insecure"),
			Spec: appsv1.ReplicaSetSpec{Template: corev1.PodTemplateSpec{Spec: insecureSpec()}}},
		&corev1.Pod{ObjectMeta: owned(meta("shop", "insecure-5d8-x1"), "ReplicaSet", "insecure-5d8"), Spec: insecureSpec()},
		&batchv1.CronJob{ObjectMeta: meta("shop", "nightly"), Spec: batchv1.CronJobSpec{JobTemplate: batchv1.JobTemplateSpec{
			Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: cronSpec}}}}},
		&batchv1.Job{ObjectMeta: owned(meta("shop", "nightly-29"), "CronJob", "nightly"), Spec: batchv1.JobSpec{Template: corev1.PodTemplateSpec{Spec: insecureSpec()}}},
		&corev1.Pod{ObjectMeta: meta("shop", "debug"), Spec: compliantSpec()},
		deployment("kube-system", "coredns", insecureSpec()),
		&corev1.ServiceAccount{ObjectMeta: meta("shop", "default"), AutomountServiceAccountToken: ptr(false)},
		clusterRole("cluster-admin", bootstrap, all),
		clusterRole("addon-role", nil, all),
		clusterRole("unused-reader", nil, rbacv1.PolicyRule{APIGroups: []string{""}, Resources: []string{"secrets"}, Verbs: []string{"list"}}),
		clusterRoleBinding("cluster-admin", "cluster-admin", bootstrap, rbacv1.Subject{Kind: "Group", Name: "system:masters"}),
		clusterRoleBinding("helm-kube-system-traefik", "addon-role", nil, rbacv1.Subject{Kind: "ServiceAccount", Namespace: "kube-system", Name: "helm-traefik"}),
		clusterRoleBinding("ci-admin", "cluster-admin", nil, rbacv1.Subject{Kind: "ServiceAccount", Namespace: "shop", Name: "ci"}),
	}
}

// findingsOf indexes a report's findings by key.
func findingsOf(rep *Report) map[string]Finding {
	out := map[string]Finding{}
	for _, f := range rep.Findings {
		out[f.Key] = f
	}
	return out
}
