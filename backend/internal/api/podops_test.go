package api

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"unicode/utf8"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

func TestForceDeletePod(t *testing.T) {
	e := newTestEnv(t, nil)
	expect(t, e.do(http.MethodDelete, "/api/pods?ns=default&name=web-1&force=true", "alice", nil), http.StatusOK, "")
	actions := e.cs.Actions()
	del := actions[len(actions)-1].(k8stesting.DeleteAction)
	if g := del.GetDeleteOptions().GracePeriodSeconds; g == nil || *g != 0 {
		t.Fatalf("force deletes with grace period 0: %+v", del.GetDeleteOptions())
	}
	expectAudit(t, e, "DELETE /api/pods", "pod/default/web-1", http.StatusOK, `{"force":"true"}`)
	expect(t, e.do(http.MethodDelete, "/api/pods?ns=default&name=web-2&force=yes", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
}

func TestBulkDeletePods(t *testing.T) {
	e := newTestEnv(t, nil)
	body := map[string]any{
		"items": []map[string]string{{"namespace": "default", "name": "web-1"}, {"namespace": "default", "name": "ghost"}},
		"force": false,
	}
	got := decode[map[string][]kube.PodDeleteResult](t, e.do(http.MethodPost, "/api/pods/bulk-delete", "alice", body))
	results := got["results"]
	if len(results) != 2 || results[0].Error != "" || !strings.Contains(results[1].Error, "not found") {
		t.Fatalf("results = %+v", results)
	}
	expectAudit(t, e, "POST /api/pods/bulk-delete", "pod", http.StatusOK, `{"failed":1,"force":false,"pods":["default/web-1","default/ghost"]}`)

	tooMany := make([]map[string]string, kube.MaxBulkDelete+1)
	for i := range tooMany {
		tooMany[i] = map[string]string{"namespace": "default", "name": "web-2"}
	}
	before := len(e.mutations())
	for _, bad := range []any{map[string]any{"items": []any{}}, map[string]any{"items": tooMany}, "[]"} {
		expect(t, e.do(http.MethodPost, "/api/pods/bulk-delete", "alice", bad), http.StatusBadRequest, plugin.CodeBadRequest)
	}
	if len(e.mutations()) != before {
		t.Fatal("an invalid bulk delete reached the cluster")
	}
}

// The audit log lists every pod of a bulk delete, across as many rows as
// the list needs, rather than cutting the list off at the row's size.
func TestBulkDeleteAuditsEveryPod(t *testing.T) {
	e := newTestEnv(t, nil)
	items := make([]map[string]string, kube.MaxBulkDelete)
	for i := range items {
		items[i] = map[string]string{"namespace": "production", "name": fmt.Sprintf("payments-api-7d9f8c6b5-%05d", i)}
	}
	expect(t, e.do(http.MethodPost, "/api/pods/bulk-delete", "alice", map[string]any{"items": items}), http.StatusOK, "")

	recorded := map[string]bool{}
	parts := 0
	for _, row := range e.audit() {
		if row.Action != "POST /api/pods/bulk-delete" {
			continue
		}
		parts++
		if n := utf8.RuneCountInString(row.Detail); n > maxAuditDetail || strings.HasSuffix(row.Detail, "…") {
			t.Fatalf("row of %d characters was cut: %s", n, row.Detail)
		}
		var detail struct {
			Pods []string `json:"pods"`
			Part string   `json:"part"`
		}
		if err := json.Unmarshal([]byte(row.Detail), &detail); err != nil || detail.Part == "" {
			t.Fatalf("detail %q: %v", row.Detail, err)
		}
		for _, p := range detail.Pods {
			recorded[p] = true
		}
	}
	if parts < 2 || len(recorded) != len(items) {
		t.Fatalf("%d rows recorded %d of %d pods", parts, len(recorded), len(items))
	}
}

func TestAuditParts(t *testing.T) {
	fields := map[string]any{"force": true}
	if got := auditParts(fields, "pods", []string{"a/b"}); len(got) != 1 || got[0] != `{"force":true,"pods":["a/b"]}` {
		t.Fatalf("one part = %q", got)
	}
	long := make([]string, 100)
	for i := range long {
		long[i] = fmt.Sprintf("namespace-%03d/%s", i, strings.Repeat("x", 40))
	}
	got := auditParts(fields, "pods", long)
	if len(got) < 2 || !strings.Contains(got[0], `"part":"1/`) || !strings.Contains(got[len(got)-1], fmt.Sprintf(`"part":"%d/%d"`, len(got), len(got))) {
		t.Fatalf("parts = %q", got)
	}
	if _, ok := fields["pods"]; ok {
		t.Fatal("auditParts changed its fields argument")
	}
}

func TestDebugContainerThroughTheAPI(t *testing.T) {
	e := newTestEnv(t, nil)
	e.add(&corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api-1", Namespace: "default"},
		Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "api", Image: "api:1"}}},
	})

	debug := decode[kube.DebugContainer](t, e.do(http.MethodPost, "/api/pods/debug?ns=default&pod=api-1&target=api", "alice", nil))
	if !strings.HasPrefix(debug.Container, "debugger-") || debug.Pod != "api-1" {
		t.Fatalf("debug = %+v", debug)
	}
	expectAudit(t, e, "POST /api/pods/debug", "pod/default/api-1", http.StatusOK,
		`{"container":"`+debug.Container+`","image":"busybox:1.37","target":"api"}`)
	pod, err := e.cs.CoreV1().Pods("default").Get(context.Background(), "api-1", metav1.GetOptions{})
	if err != nil || len(pod.Spec.EphemeralContainers) != 1 || pod.Spec.EphemeralContainers[0].Name != debug.Container {
		t.Fatalf("pod = %+v, %v", pod, err)
	}

	expect(t, e.do(http.MethodPost, "/api/pods/debug?ns=default&pod=api-1&target=nope", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/pods/debug?ns=default", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/pods/debug?ns=default&pod=ghost", "alice", nil), http.StatusNotFound, plugin.CodeKubernetes)
}
