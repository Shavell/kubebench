package api

import (
	"context"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/plugin"
)

func TestNamespacesThroughTheAPI(t *testing.T) {
	e := newTestEnv(t, map[string]string{"KUBEBENCH_NAMESPACE": "kubebench"})
	e.add(
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kube-system"}},
		&corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: "kubebench"}},
	)
	exists := func(name string) bool {
		_, err := e.cs.CoreV1().Namespaces().Get(context.Background(), name, metav1.GetOptions{})
		return err == nil
	}

	expect(t, e.do(http.MethodPost, "/api/namespaces", "alice", map[string]any{"name": "team-a", "labels": map[string]string{"team": "a"}}), http.StatusOK, "")
	if !exists("team-a") {
		t.Fatal("namespace not created")
	}
	expectAudit(t, e, "POST /api/namespaces", "namespace/team-a", http.StatusOK, `{"labels":["team=a"]}`)
	expect(t, e.do(http.MethodPost, "/api/namespaces", "alice", map[string]any{"name": "team-a"}), http.StatusConflict, plugin.CodeKubernetes)
	for _, bad := range []any{map[string]any{"name": "Team A"}, map[string]any{}, map[string]any{"name": "ok", "labels": map[string]string{"bad key": "x"}}} {
		expect(t, e.do(http.MethodPost, "/api/namespaces", "alice", bad), http.StatusBadRequest, plugin.CodeBadRequest)
	}

	// System namespaces and KubeBench's own are refused unless forced.
	for _, name := range []string{"kube-system", "kubebench"} {
		expect(t, e.do(http.MethodDelete, "/api/namespaces?name="+name, "alice", nil), http.StatusForbidden, plugin.CodeForbidden)
		if !exists(name) {
			t.Fatalf("%s was deleted", name)
		}
	}
	expect(t, e.do(http.MethodDelete, "/api/namespaces?name=kubebench&force=true", "alice", nil), http.StatusOK, "")
	expectAudit(t, e, "DELETE /api/namespaces", "namespace/kubebench", http.StatusOK, `{"force":"true"}`)
	expect(t, e.do(http.MethodDelete, "/api/namespaces?name=team-a", "alice", nil), http.StatusOK, "")
	if exists("team-a") || exists("kubebench") {
		t.Fatal("namespaces not deleted")
	}
	expect(t, e.do(http.MethodDelete, "/api/namespaces", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodDelete, "/api/namespaces?name=team-a&force=sure", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
}
