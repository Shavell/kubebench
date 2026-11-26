package api

import (
	"net/http"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/plugin"
)

// lastLogOptions returns the options of the latest pods/log request.
func (e *testEnv) lastLogOptions() *corev1.PodLogOptions {
	e.t.Helper()
	actions := e.cs.Actions()
	for i := len(actions) - 1; i >= 0; i-- {
		if a, ok := actions[i].(k8stesting.GenericActionImpl); ok && a.GetSubresource() == "log" {
			return a.Value.(*corev1.PodLogOptions)
		}
	}
	e.t.Fatal("no log request reached the cluster")
	return nil
}

func TestLogsParameters(t *testing.T) {
	e := newTestEnv(t, nil)
	lines := decode[[]string](t, e.do(http.MethodGet, "/api/logs?ns=default&pod=web-1&container=app", "bob", nil))
	if len(lines) != 1 || lines[0] != "fake logs" {
		t.Fatalf("lines = %v", lines)
	}
	if o := e.lastLogOptions(); *o.TailLines != defaultLogTail || o.Previous || o.SinceSeconds != nil || o.Container != "app" || !o.Timestamps {
		t.Fatalf("default options = %+v", o)
	}

	expect(t, e.do(http.MethodGet, "/api/logs?ns=default&pod=web-1&tail=20&previous=true&sinceSeconds=60", "bob", nil), http.StatusOK, "")
	if o := e.lastLogOptions(); *o.TailLines != 20 || !o.Previous || *o.SinceSeconds != 60 {
		t.Fatalf("options = %+v", o)
	}
	expect(t, e.do(http.MethodGet, "/api/logs?ns=default&pod=web-1&tail=99999", "bob", nil), http.StatusOK, "")
	if o := e.lastLogOptions(); *o.TailLines != maxLogTail {
		t.Fatalf("tail is capped at %d, got %d", maxLogTail, *o.TailLines)
	}
	for _, query := range []string{"tail=0", "tail=x", "previous=maybe", "sinceSeconds=-5"} {
		expect(t, e.do(http.MethodGet, "/api/logs?ns=default&pod=web-1&"+query, "bob", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	}
}

func TestEventsFilterOnKind(t *testing.T) {
	e := newTestEnv(t, nil)
	expect(t, e.do(http.MethodGet, "/api/events?ns=default&kind=Deployment&name=web", "bob", nil), http.StatusOK, "")
	actions := e.cs.Actions()
	list, ok := actions[len(actions)-1].(k8stesting.ListActionImpl)
	if !ok || list.GetResource().Resource != "events" {
		t.Fatalf("last action = %#v", actions[len(actions)-1])
	}
	// The fake Deployment has no UID: matched by kind, name and namespace.
	if got := list.GetListRestrictions().Fields.String(); got != "involvedObject.kind=Deployment,involvedObject.name=web,involvedObject.namespace=default" {
		t.Fatalf("field selector = %q", got)
	}
}

// Events of a deleted object of the same name are left out: the current
// object's UID (or the one the caller names) joins the selector.
func TestEventsFilterOnUID(t *testing.T) {
	e := newTestEnv(t, nil)
	pod, err := e.cs.CoreV1().Pods("default").Get(t.Context(), "web-1", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	pod.UID = "uid-now"
	if _, err := e.cs.CoreV1().Pods("default").Update(t.Context(), pod, metav1.UpdateOptions{}); err != nil {
		t.Fatal(err)
	}
	selector := func(target string) string {
		t.Helper()
		expect(t, e.do(http.MethodGet, target, "bob", nil), http.StatusOK, "")
		actions := e.cs.Actions()
		return actions[len(actions)-1].(k8stesting.ListActionImpl).GetListRestrictions().Fields.String()
	}
	base := "involvedObject.kind=Pod,involvedObject.name=web-1,involvedObject.namespace=default,"
	if got := selector("/api/events?ns=default&kind=Pod&name=web-1"); got != base+"involvedObject.uid=uid-now" {
		t.Fatalf("looked-up uid: %q", got)
	}
	if got := selector("/api/events?ns=default&kind=Pod&name=web-1&uid=uid-old"); got != base+"involvedObject.uid=uid-old" {
		t.Fatalf("explicit uid: %q", got)
	}
	// A gone object is matched by name.
	if got := selector("/api/events?ns=default&kind=Pod&name=gone"); got != "involvedObject.kind=Pod,involvedObject.name=gone,involvedObject.namespace=default" {
		t.Fatalf("gone object: %q", got)
	}
}

func TestMutationParametersAreValidated(t *testing.T) {
	e := newTestEnv(t, nil)
	for _, target := range []string{
		"/api/deployments/scale?ns=default&name=web",
		"/api/deployments/scale?ns=default&name=web&replicas=-1",
		"/api/deployments/scale?ns=default&name=web&replicas=lots",
		"/api/deployments/restart?ns=default",
		"/api/exec?ns=default&pod=web-1",
	} {
		expect(t, e.do(http.MethodPost, target, "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	}
	expect(t, e.do(http.MethodDelete, "/api/pods?name=web-1", "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodGet, "/api/resource?key=nope", "bob", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	if m := e.mutations(); len(m) != 0 || e.replicas() != 2 {
		t.Fatalf("invalid requests reached the cluster: %v", m)
	}
}
