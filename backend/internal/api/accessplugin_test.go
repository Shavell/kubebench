package api

import (
	"net/http"
	"testing"

	authv1 "k8s.io/api/authorization/v1"
	"k8s.io/apimachinery/pkg/runtime"
	k8stesting "k8s.io/client-go/testing"

	accessplugin "kubebench/internal/plugins/access"
	"kubebench/internal/store"
)

// Every access route is read: a viewer may ask them even on a read-only
// cluster, including POST /review, and none of them is audited.
func TestAccessRoutesAreReadForViewers(t *testing.T) {
	e := newTestEnv(t, nil, accessplugin.New())
	readOnly := true
	if err := e.store.UpdateCluster(e.cluster, store.ClusterUpdate{ReadOnly: &readOnly}); err != nil {
		t.Fatal(err)
	}
	for _, path := range []string{
		"/api/plugins/access/subjects",
		"/api/plugins/access/subjects/User/alice",
		"/api/plugins/access/who-can?verb=get&resource=pods&namespace=default",
		"/api/plugins/access/risky",
		"/api/plugins/access/graph?namespace=default",
	} {
		expect(t, e.do(http.MethodGet, path, "bob", nil), http.StatusOK, "")
	}
	// The fake clientset cannot store reviews; answer them as the API server would.
	e.cs.PrependReactor("create", "subjectaccessreviews", func(a k8stesting.Action) (bool, runtime.Object, error) {
		sar := a.(k8stesting.CreateAction).GetObject().(*authv1.SubjectAccessReview).DeepCopy()
		sar.Status.Allowed = false
		return true, sar, nil
	})
	review := map[string]any{
		"subject": map[string]string{"kind": "ServiceAccount", "namespace": "default", "name": "default"},
		"verb":    "list", "resource": "secrets", "namespace": "default",
	}
	expect(t, e.do(http.MethodPost, "/api/plugins/access/review", "bob", review), http.StatusOK, "")
	if rows := e.audit(); len(rows) != 0 {
		t.Fatalf("access requests were audited: %+v", rows)
	}
	for _, a := range e.cs.Actions() {
		if a.GetVerb() != "list" && !(a.GetVerb() == "create" && a.GetResource().Resource == "subjectaccessreviews") {
			t.Errorf("unexpected request to the cluster: %s %s", a.GetVerb(), a.GetResource().Resource)
		}
	}
}
