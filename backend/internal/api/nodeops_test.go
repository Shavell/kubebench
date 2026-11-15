package api

import (
	"context"
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

func (e *testEnv) node(name string) *corev1.Node {
	e.t.Helper()
	n, err := e.cs.CoreV1().Nodes().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		e.t.Fatal(err)
	}
	return n
}

func TestCordonThroughTheAPI(t *testing.T) {
	e := newTestEnv(t, nil)
	e.add(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}})

	expect(t, e.do(http.MethodPost, "/api/nodes/cordon?name=worker", "alice", nil), http.StatusOK, "")
	if !e.node("worker").Spec.Unschedulable {
		t.Fatal("cordon is the default")
	}
	expectAudit(t, e, "POST /api/nodes/cordon", "node/worker", http.StatusOK, "")
	expect(t, e.do(http.MethodPost, "/api/nodes/cordon?name=worker&unschedulable=false", "alice", nil), http.StatusOK, "")
	if e.node("worker").Spec.Unschedulable {
		t.Fatal("unschedulable=false uncordons")
	}
	expectAudit(t, e, "POST /api/nodes/cordon", "node/worker", http.StatusOK, `{"unschedulable":"false"}`)

	nodes := decode[[]kube.NodeInfo](t, e.do(http.MethodGet, "/api/nodes", "bob", nil))
	if len(nodes) != 1 || nodes[0].Unschedulable {
		t.Fatalf("nodes = %+v", nodes)
	}

	expect(t, e.do(http.MethodPost, "/api/nodes/cordon", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/nodes/cordon?name=worker&unschedulable=maybe", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodPost, "/api/nodes/cordon?name=ghost", "alice", nil), http.StatusNotFound, plugin.CodeKubernetes)
}

func TestTaintsThroughTheAPI(t *testing.T) {
	e := newTestEnv(t, nil)
	e.add(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}})

	body := map[string]any{"taints": []map[string]string{
		{"key": "dedicated", "value": "gpu", "effect": "NoSchedule"},
		{"key": "maintenance", "effect": "NoExecute"},
	}}
	got := decode[map[string][]kube.Taint](t, e.do(http.MethodPut, "/api/nodes/taints?name=worker", "alice", body))
	if len(got["taints"]) != 2 || got["taints"][0].Value != "gpu" || len(e.node("worker").Spec.Taints) != 2 {
		t.Fatalf("taints = %+v", got)
	}
	expectAudit(t, e, "PUT /api/nodes/taints", "node/worker", http.StatusOK,
		`{"taints":["dedicated=gpu:NoSchedule","maintenance:NoExecute"]}`)

	before := len(e.mutations())
	for _, bad := range []any{
		map[string]any{},
		map[string]any{"taints": []map[string]string{{"key": "x", "effect": "Never"}}},
		"not json",
	} {
		expect(t, e.do(http.MethodPut, "/api/nodes/taints?name=worker", "alice", bad), http.StatusBadRequest, plugin.CodeBadRequest)
	}
	if len(e.mutations()) != before {
		t.Fatal("invalid taints reached the cluster")
	}
	expect(t, e.do(http.MethodPut, "/api/nodes/taints?name=worker", "alice", map[string]any{"taints": []any{}}), http.StatusOK, "")
	if len(e.node("worker").Spec.Taints) != 0 {
		t.Fatal("[] removes every taint")
	}
}

func TestDrainThroughTheAPI(t *testing.T) {
	e := newTestEnv(t, nil)
	e.add(&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}})
	e.podsOnNodes()

	before := len(e.mutations())
	for _, query := range []string{
		"", "&timeoutSeconds=0", "&timeoutSeconds=601", "&gracePeriod=-2", "&gracePeriod=x", "&ignoreDaemonSets=maybe",
	} {
		target := "/api/nodes/drain?name=worker" + query
		if query == "" {
			target = "/api/nodes/drain"
		}
		expect(t, e.do(http.MethodPost, target, "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	}
	if len(e.mutations()) != before {
		t.Fatal("an invalid drain reached the cluster")
	}

	// The node has no pods, so the drain only cordons it.
	res := decode[kube.DrainResult](t, e.do(http.MethodPost, "/api/nodes/drain?name=worker&timeoutSeconds=5", "alice", nil))
	if !res.Cordoned || !res.Completed || res.Node != "worker" || !e.node("worker").Spec.Unschedulable {
		t.Fatalf("result = %+v", res)
	}
	expectAudit(t, e, "POST /api/nodes/drain", "node/worker", http.StatusOK,
		`{"blockedByPDB":0,"completed":true,"deleteEmptyDirData":false,"evicted":0,"failed":0,"force":false,"gracePeriod":-1,"ignoreDaemonSets":true,"timeoutSeconds":5}`)
	expect(t, e.do(http.MethodPost, "/api/nodes/drain?name=ghost", "alice", nil), http.StatusNotFound, plugin.CodeKubernetes)
}
