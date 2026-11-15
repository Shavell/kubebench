package argocd

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"slices"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

type auditRow struct{ action, target, detail string }

type testEnv struct {
	t      *testing.T
	dyn    *dynamicfake.FakeDynamicClient
	ctx    *plugin.Context
	audits []auditRow
}

func newEnv(t *testing.T, objects ...runtime.Object) *testEnv {
	t.Helper()
	dyn := dynamicfake.NewSimpleDynamicClientWithCustomListKinds(runtime.NewScheme(), map[schema.GroupVersionResource]string{
		applicationsGVR:    "ApplicationList",
		projectsGVR:        "AppProjectList",
		applicationSetsGVR: "ApplicationSetList",
	}, objects...)
	e := &testEnv{t: t, dyn: dyn}
	rt := kube.NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, fake.NewClientset(), dyn)
	e.ctx = &plugin.Context{
		Runtime: rt,
		User:    &store.User{Username: "alice"},
		Audit: func(action, target, detail string) {
			e.audits = append(e.audits, auditRow{action, target, detail})
		},
	}
	return e
}

// do calls h like the router would; pathValues are name/value pairs.
func (e *testEnv) do(h plugin.HandlerFunc, method, target string, body any, pathValues ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	var buf bytes.Buffer
	if body != nil {
		if err := json.NewEncoder(&buf).Encode(body); err != nil {
			e.t.Fatal(err)
		}
	}
	r := httptest.NewRequest(method, target, &buf)
	for i := 0; i+1 < len(pathValues); i += 2 {
		r.SetPathValue(pathValues[i], pathValues[i+1])
	}
	rec := httptest.NewRecorder()
	h(rec, r, e.ctx)
	return rec
}

func (e *testEnv) app(name string) *unstructured.Unstructured {
	e.t.Helper()
	u, err := e.dyn.Resource(applicationsGVR).Namespace("argocd").Get(e.t.Context(), name, metav1.GetOptions{})
	if err != nil {
		e.t.Fatal(err)
	}
	return u
}

// mutations lists the verbs of the non-read actions sent to the cluster.
func (e *testEnv) mutations() []string {
	var verbs []string
	for _, a := range e.dyn.Actions() {
		if v := a.GetVerb(); v != "get" && v != "list" && v != "watch" {
			verbs = append(verbs, v)
		}
	}
	return verbs
}

func expectStatus(t *testing.T, rec *httptest.ResponseRecorder, status int, code string) {
	t.Helper()
	if rec.Code != status {
		t.Fatalf("status = %d, want %d: %s", rec.Code, status, rec.Body)
	}
	if code == "" {
		return
	}
	var body plugin.ErrorBody
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Code != code {
		t.Fatalf("body = %s, want code %q", rec.Body, code)
	}
}

func decodeJSON[T any](t *testing.T, rec *httptest.ResponseRecorder) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(rec.Body.Bytes(), &v); err != nil {
		t.Fatalf("decode %s: %v", rec.Body, err)
	}
	return v
}

// application returns an Application in namespace argocd; edit changes
// its object before it is used.
func application(name string, edit func(obj map[string]any)) *unstructured.Unstructured {
	obj := guestbookApp()
	obj["apiVersion"] = groupVersion
	obj["kind"] = "Application"
	obj["metadata"] = map[string]any{"name": name, "namespace": "argocd", "uid": "uid-" + name, "resourceVersion": "1"}
	if edit != nil {
		edit(obj)
	}
	return &unstructured.Unstructured{Object: obj}
}

func setPath(obj map[string]any, value any, path ...string) {
	for _, key := range path[:len(path)-1] {
		next, ok := obj[key].(map[string]any)
		if !ok {
			next = map[string]any{}
			obj[key] = next
		}
		obj = next
	}
	obj[path[len(path)-1]] = value
}

func TestListAndGetApplications(t *testing.T) {
	e := newEnv(t,
		application("web", func(o map[string]any) {
			setPath(o, "team", "spec", "project")
			setPath(o, "Synced", "status", "sync", "status")
		}),
		application("api", nil),
	)
	rec := e.do(listApplications, http.MethodGet, "/applications?ns=argocd", nil)
	expectStatus(t, rec, http.StatusOK, "")
	list := decodeJSON[[]Summary](t, rec)
	if len(list) != 2 || list[0].Name != "api" || list[1].Name != "web" || list[1].Sync != "Synced" || list[0].Sync != "Unknown" {
		t.Fatalf("list = %+v", list)
	}

	rec = e.do(getApplication, http.MethodGet, "/applications/argocd/web", nil, "ns", "argocd", "name", "web")
	expectStatus(t, rec, http.StatusOK, "")
	if d := decodeJSON[Detail](t, rec); d.Project != "team" || len(d.History) != 2 || d.History[0].ID != 1 {
		t.Fatalf("detail = %+v", d)
	}
	rec = e.do(getApplication, http.MethodGet, "/applications/argocd/nope", nil, "ns", "argocd", "name", "nope")
	expectStatus(t, rec, http.StatusNotFound, plugin.CodeKubernetes)
}

func TestSyncSetsTheOperation(t *testing.T) {
	e := newEnv(t, application("web", nil))
	rec := e.do(syncApplication, http.MethodPost, "/applications/argocd/web/sync",
		map[string]any{"prune": true, "resources": []map[string]string{{"kind": "Service", "name": "web", "namespace": "web"}}},
		"ns", "argocd", "name", "web")
	expectStatus(t, rec, http.StatusOK, "")
	if d := decodeJSON[Detail](t, rec); !d.Operating {
		t.Fatalf("detail after sync is not operating: %+v", d.Summary)
	}

	op := e.app("web").Object["operation"].(map[string]any)
	want := map[string]any{
		"initiatedBy": map[string]any{"username": "kubebench:alice"},
		"sync": map[string]any{
			"revision":    "main",
			"prune":       true,
			"resources":   []any{map[string]any{"group": "", "kind": "Service", "namespace": "web", "name": "web"}},
			"syncOptions": []any{"CreateNamespace=true", "ServerSideApply=true"},
		},
	}
	if !reflect.DeepEqual(op, want) {
		t.Fatalf("operation = %#v\nwant %#v", op, want)
	}
	if len(e.audits) != 1 || e.audits[0].action != "argocd.sync" || e.audits[0].target != "application/argocd/web" ||
		e.audits[0].detail != `{"dryRun":false,"prune":true,"resources":1,"revision":""}` {
		t.Fatalf("audit = %+v", e.audits)
	}

	// The operation is now pending: another sync is refused.
	rec = e.do(syncApplication, http.MethodPost, "/applications/argocd/web/sync", nil, "ns", "argocd", "name", "web")
	expectStatus(t, rec, http.StatusConflict, plugin.CodeConflict)
}

func TestSyncConflictsWithARunningOperation(t *testing.T) {
	e := newEnv(t, application("web", func(o map[string]any) {
		setPath(o, "Running", "status", "operationState", "phase")
	}))
	rec := e.do(syncApplication, http.MethodPost, "/applications/argocd/web/sync", nil, "ns", "argocd", "name", "web")
	expectStatus(t, rec, http.StatusConflict, plugin.CodeConflict)
	if m := e.mutations(); len(m) != 0 {
		t.Fatalf("a refused sync sent %v", m)
	}
	if len(e.audits) != 0 {
		t.Fatalf("a refused sync was audited as started: %+v", e.audits)
	}
}

func TestSyncRejectsBadBodies(t *testing.T) {
	e := newEnv(t, application("web", nil))
	for _, body := range []any{map[string]any{"revisionn": "x"}, "nope", map[string]any{"resources": []map[string]string{{"name": "x"}}}} {
		rec := e.do(syncApplication, http.MethodPost, "/applications/argocd/web/sync", body, "ns", "argocd", "name", "web")
		expectStatus(t, rec, http.StatusBadRequest, plugin.CodeBadRequest)
	}
	if m := e.mutations(); len(m) != 0 {
		t.Fatalf("bad requests sent %v", m)
	}
}

// The patch carries the resourceVersion that was checked, so an operation
// the controller starts between the check and the patch is not overwritten.
func TestOperationPatchIsOptimisticallyLocked(t *testing.T) {
	e := newEnv(t, application("web", nil))
	e.dyn.PrependReactor("patch", "applications", func(a k8stesting.Action) (bool, runtime.Object, error) {
		var body map[string]any
		if err := json.Unmarshal(a.(k8stesting.PatchAction).GetPatch(), &body); err != nil {
			t.Fatal(err)
		}
		if rv := field(body, "metadata", "resourceVersion"); rv != "1" {
			t.Fatalf("patch resourceVersion = %v", rv)
		}
		return false, nil, nil
	})
	rec := e.do(syncApplication, http.MethodPost, "/applications/argocd/web/sync", nil, "ns", "argocd", "name", "web")
	expectStatus(t, rec, http.StatusOK, "")
}

func TestRefresh(t *testing.T) {
	e := newEnv(t, application("web", nil))
	for _, tt := range []struct{ query, want string }{{"", "normal"}, {"?hard=true", "hard"}} {
		rec := e.do(refreshApplication, http.MethodPost, "/applications/argocd/web/refresh"+tt.query, nil, "ns", "argocd", "name", "web")
		expectStatus(t, rec, http.StatusOK, "")
		if got := e.app("web").GetAnnotations()[refreshAnnotation]; got != tt.want {
			t.Fatalf("refresh%s annotation = %q, want %q", tt.query, got, tt.want)
		}
	}
	rec := e.do(refreshApplication, http.MethodPost, "/applications/argocd/web/refresh?hard=yes", nil, "ns", "argocd", "name", "web")
	expectStatus(t, rec, http.StatusBadRequest, plugin.CodeBadRequest)
}

func TestRollback(t *testing.T) {
	e := newEnv(t,
		application("web", nil),
		application("auto", func(o map[string]any) {
			setPath(o, map[string]any{"selfHeal": true}, "spec", "syncPolicy", "automated")
		}),
	)
	rec := e.do(rollbackApplication, http.MethodPost, "/applications/argocd/web/rollback", map[string]any{"historyId": 0},
		"ns", "argocd", "name", "web")
	expectStatus(t, rec, http.StatusOK, "")
	sync := field(e.app("web").Object, "operation", "sync").(map[string]any)
	if sync["revision"] != "aaa111" || str(sync, "source", "targetRevision") != "v1" {
		t.Fatalf("rollback sync = %#v", sync)
	}
	if len(e.audits) != 1 || e.audits[0].action != "argocd.rollback" || e.audits[0].detail != `{"historyId":0,"prune":false}` {
		t.Fatalf("audit = %+v", e.audits)
	}

	rec = e.do(rollbackApplication, http.MethodPost, "/applications/argocd/auto/rollback", map[string]any{"historyId": 0},
		"ns", "argocd", "name", "auto")
	expectStatus(t, rec, http.StatusConflict, plugin.CodeConflict)
	if _, set := e.app("auto").Object["operation"]; set {
		t.Fatal("a refused rollback set an operation")
	}
}

func TestDeleteSetsTheCascadeFinalizer(t *testing.T) {
	tests := []struct {
		name, query string
		finalizers  []string
		want        []string // finalizers just before the delete; nil: not patched
	}{
		{name: "cascade by default", want: []string{resourcesFinalizer}},
		{name: "cascade keeps other finalizers", query: "?cascade=true", finalizers: []string{"example.com/x"}, want: []string{"example.com/x", resourcesFinalizer}},
		{name: "cascade already set", query: "?cascade=true", finalizers: []string{resourcesFinalizerBackground}},
		{name: "no cascade removes it", query: "?cascade=false", finalizers: []string{resourcesFinalizer, "example.com/x"}, want: []string{"example.com/x"}},
		{name: "no cascade without finalizer", query: "?cascade=false"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			e := newEnv(t, application("web", func(o map[string]any) {
				if tt.finalizers != nil {
					setPath(o, toAnyList(tt.finalizers), "metadata", "finalizers")
				}
			}))
			var patched []string
			// Reactors run under the fake's lock: read the tracker, not the client.
			e.dyn.PrependReactor("delete", "applications", func(k8stesting.Action) (bool, runtime.Object, error) {
				obj, err := e.dyn.Tracker().Get(applicationsGVR, "argocd", "web")
				if err != nil {
					t.Fatal(err)
				}
				patched = obj.(*unstructured.Unstructured).GetFinalizers()
				return false, nil, nil
			})
			rec := e.do(deleteApplication, http.MethodDelete, "/applications/argocd/web"+tt.query, nil, "ns", "argocd", "name", "web")
			expectStatus(t, rec, http.StatusOK, "")
			wantVerbs := []string{"delete"}
			if tt.want != nil {
				wantVerbs = []string{"patch", "delete"}
				if !slices.Equal(patched, tt.want) {
					t.Fatalf("finalizers = %v, want %v", patched, tt.want)
				}
			}
			if m := e.mutations(); !slices.Equal(m, wantVerbs) {
				t.Fatalf("mutations = %v, want %v", m, wantVerbs)
			}
		})
	}
}

func TestDeleteRejectsABadCascade(t *testing.T) {
	e := newEnv(t, application("web", nil))
	rec := e.do(deleteApplication, http.MethodDelete, "/applications/argocd/web?cascade=1", nil, "ns", "argocd", "name", "web")
	expectStatus(t, rec, http.StatusBadRequest, plugin.CodeBadRequest)
	if m := e.mutations(); len(m) != 0 {
		t.Fatalf("mutations = %v", m)
	}
}

func TestProjectsAndApplicationSets(t *testing.T) {
	project := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": groupVersion, "kind": "AppProject",
		"metadata": map[string]any{"name": "default", "namespace": "argocd"},
		"spec": map[string]any{
			"sourceRepos":              []any{"*"},
			"destinations":             []any{map[string]any{"server": "*", "namespace": "*"}},
			"clusterResourceWhitelist": []any{map[string]any{"group": "*", "kind": "*"}},
		},
	}}
	set := &unstructured.Unstructured{Object: map[string]any{
		"apiVersion": groupVersion, "kind": "ApplicationSet",
		"metadata": map[string]any{"name": "guestbooks", "namespace": "argocd"},
		"spec": map[string]any{
			"generators": []any{map[string]any{"list": map[string]any{"elements": []any{}}}},
			"template": map[string]any{
				"metadata": map[string]any{"name": "{{name}}-guestbook"},
				"spec":     map[string]any{"project": "default"},
			},
		},
		"status": map[string]any{"resources": []any{
			map[string]any{"name": "dev-guestbook", "status": "Synced", "health": map[string]any{"status": "Healthy"}},
		}},
	}}
	e := newEnv(t, project, set)

	rec := e.do(listProjects, http.MethodGet, "/projects", nil)
	expectStatus(t, rec, http.StatusOK, "")
	projects := decodeJSON[[]Project](t, rec)
	if len(projects) != 1 || projects[0].Name != "default" || !slices.Equal(projects[0].ClusterResources, []string{"*/*"}) {
		t.Fatalf("projects = %+v", projects)
	}

	rec = e.do(listApplicationSets, http.MethodGet, "/applicationsets", nil)
	expectStatus(t, rec, http.StatusOK, "")
	sets := decodeJSON[[]ApplicationSet](t, rec)
	if len(sets) != 1 || !slices.Equal(sets[0].Generators, []string{"list"}) || sets[0].TemplateName != "{{name}}-guestbook" ||
		len(sets[0].Applications) != 1 || sets[0].Applications[0].Health != "Healthy" {
		t.Fatalf("applicationsets = %+v", sets)
	}
}
