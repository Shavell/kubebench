package access

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/fake"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// env mounts the plugin's routes on a mux around a fake cluster, as the
// API server mounts them (without its gates, which internal/api tests).
type env struct {
	t      *testing.T
	rt     *kube.Runtime
	mux    *http.ServeMux
	now    time.Time
	access map[string]plugin.Access
}

func newEnv(t *testing.T, objects ...runtime.Object) *env {
	t.Helper()
	return newEnvWith(t, fake.NewClientset(objects...))
}

func newEnvWith(t *testing.T, cs kubernetes.Interface) *env {
	t.Helper()
	e := &env{t: t, rt: kube.NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, nil), mux: http.NewServeMux(), now: time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC), access: map[string]plugin.Access{}}
	newPlugin(func() time.Time { return e.now }).Routes(e)
	return e
}

// Handle implements plugin.Router.
func (e *env) Handle(method, path string, a plugin.Access, h plugin.HandlerFunc) {
	e.access[method+" "+path] = a
	e.mux.HandleFunc(method+" "+path, func(w http.ResponseWriter, r *http.Request) {
		h(w, r, &plugin.Context{Runtime: e.rt, User: &store.User{ID: 7, Username: "ops"}})
	})
}

func (e *env) do(method, path string, body any) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	if s, ok := body.(string); ok {
		buf.WriteString(s)
	} else if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
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

// --- fixtures: a small cluster shaped like an Argo CD install ----------------

func sa(ns, name string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}}
}

func clusterRole(name string, rules ...rbacv1.PolicyRule) *rbacv1.ClusterRole {
	return &rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: name}, Rules: rules}
}

func nsRole(ns, name string, rules ...rbacv1.PolicyRule) *rbacv1.Role {
	return &rbacv1.Role{ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name}, Rules: rules}
}

func crb(name, roleName string, subjects ...rbacv1.Subject) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{Name: name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: kindClusterRole, Name: roleName},
		Subjects:   subjects,
	}
}

func rb(ns, name, roleKind, roleName string, subjects ...rbacv1.Subject) *rbacv1.RoleBinding {
	return &rbacv1.RoleBinding{
		ObjectMeta: metav1.ObjectMeta{Namespace: ns, Name: name},
		RoleRef:    rbacv1.RoleRef{APIGroup: rbacv1.GroupName, Kind: roleKind, Name: roleName},
		Subjects:   subjects,
	}
}

func saSubject(ns, name string) rbacv1.Subject {
	return rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Namespace: ns, Name: name}
}

func userSubject(name string) rbacv1.Subject {
	return rbacv1.Subject{Kind: rbacv1.UserKind, APIGroup: rbacv1.GroupName, Name: name}
}

func groupSubject(name string) rbacv1.Subject {
	return rbacv1.Subject{Kind: rbacv1.GroupKind, APIGroup: rbacv1.GroupName, Name: name}
}

func fixture() []runtime.Object {
	view := clusterRole("view", rule(l("get", "list", "watch"), l(""), l("pods", "configmaps")))
	view.AggregationRule = &rbacv1.AggregationRule{ClusterRoleSelectors: []metav1.LabelSelector{
		{MatchLabels: map[string]string{"rbac.authorization.k8s.io/aggregate-to-view": "true"}},
	}}
	return []runtime.Object{
		sa("argocd", "argocd-server"),
		sa("argocd", "argocd-application-controller"),
		sa("default", "default"),
		clusterRole("cluster-admin",
			rule(l("*"), l("*"), l("*")),
			rbacv1.PolicyRule{Verbs: l("*"), NonResourceURLs: l("*")}),
		view,
		clusterRole("argocd-application-controller", rule(l("*"), l("*"), l("*"))),
		clusterRole("system:discovery", rbacv1.PolicyRule{Verbs: l("get"), NonResourceURLs: l("/api", "/api/*", "/healthz")}),
		nsRole("argocd", "argocd-server",
			rule(l("get", "list", "watch"), l(""), l("secrets", "configmaps")),
			rbacv1.PolicyRule{Verbs: l("update"), APIGroups: l(""), Resources: l("configmaps"), ResourceNames: l("argocd-cm")}),
		crb("cluster-admin", "cluster-admin", groupSubject("system:masters")),
		crb("argocd-application-controller", "argocd-application-controller", saSubject("argocd", "argocd-application-controller")),
		crb("system:discovery", "system:discovery", groupSubject("system:authenticated")),
		crb("dangling", "no-such-role", userSubject("bob")),
		// The subject has no namespace: it defaults to the binding's.
		rb("argocd", "argocd-server", kindRole, "argocd-server", rbacv1.Subject{Kind: rbacv1.ServiceAccountKind, Name: "argocd-server"}),
		rb("shop", "view-shop", kindClusterRole, "view", groupSubject("devs"), userSubject("alice"), saSubject("shop", "ghost")),
	}
}
