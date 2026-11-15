package api

import (
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/store"
)

// Test helpers for the workload and node operations (§7.3).

// opsRoutes are the operation requests that change the cluster.
var opsRoutes = []struct{ method, target string }{
	{http.MethodPost, "/api/nodes/cordon?name=worker"},
	{http.MethodPost, "/api/nodes/drain?name=worker"},
	{http.MethodPut, "/api/nodes/taints?name=worker"},
	{http.MethodPost, "/api/deployments/rollback?ns=default&name=web&revision=1"},
	{http.MethodPost, "/api/deployments/pause?ns=default&name=web"},
	{http.MethodPost, "/api/workloads/restart?kind=StatefulSet&ns=default&name=db"},
	{http.MethodPost, "/api/workloads/scale?kind=StatefulSet&ns=default&name=db&replicas=3"},
	{http.MethodDelete, "/api/pods?ns=default&name=web-1&force=true"},
	{http.MethodPost, "/api/pods/bulk-delete"},
	{http.MethodPost, "/api/pods/debug?ns=default&pod=web-1"},
	{http.MethodPost, "/api/cronjobs/trigger?ns=default&name=report"},
	{http.MethodPost, "/api/cronjobs/suspend?ns=default&name=report"},
	{http.MethodPost, "/api/namespaces"},
	{http.MethodDelete, "/api/namespaces?name=team-a"},
}

// add puts objects into the fake cluster.
func (e *testEnv) add(objects ...runtime.Object) {
	e.t.Helper()
	for _, obj := range objects {
		if err := e.cs.Tracker().Add(obj); err != nil {
			e.t.Fatal(err)
		}
	}
}

// podsOnNodes makes pod lists honour the spec.nodeName field selector, as
// the API server does (the fake ignores field selectors).
func (e *testEnv) podsOnNodes() {
	e.cs.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		list := action.(k8stesting.ListAction)
		node, ok := list.GetListRestrictions().Fields.RequiresExactMatch("spec.nodeName")
		if !ok {
			return false, nil, nil
		}
		obj, err := e.cs.Tracker().List(corev1.SchemeGroupVersion.WithResource("pods"),
			corev1.SchemeGroupVersion.WithKind("Pod"), list.GetNamespace())
		if err != nil {
			return true, nil, err
		}
		filtered := &corev1.PodList{}
		for _, p := range obj.(*corev1.PodList).Items {
			if p.Spec.NodeName == node {
				filtered.Items = append(filtered.Items, p)
			}
		}
		return true, filtered, nil
	})
}

// dynamicFake is the dynamic client of the test cluster's runtime.
func (e *testEnv) dynamicFake() *dynamicfake.FakeDynamicClient {
	e.t.Helper()
	cluster, err := e.store.GetCluster(e.cluster)
	if err != nil {
		e.t.Fatal(err)
	}
	rt, err := e.srv.manager.Runtime(cluster)
	if err != nil {
		e.t.Fatal(err)
	}
	return rt.Dyn.(*dynamicfake.FakeDynamicClient)
}

// lastAudit is the newest audit row.
func (e *testEnv) lastAudit() store.AuditEntry {
	e.t.Helper()
	rows := e.audit()
	if len(rows) == 0 {
		e.t.Fatal("no audit row was written")
	}
	return rows[0]
}

// expectAudit checks the newest audit row; an empty detail is not compared.
func expectAudit(t *testing.T, e *testEnv, action, target string, status int, detail string) {
	t.Helper()
	row := e.lastAudit()
	if row.Action != action || row.Target != target || row.Status != status || (detail != "" && row.Detail != detail) {
		t.Fatalf("audit row = %+v, want %s %s %d %s", row, action, target, status, detail)
	}
}
