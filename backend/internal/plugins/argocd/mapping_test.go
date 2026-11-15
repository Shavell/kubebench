package argocd

import (
	"encoding/json"
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
)

// syncedApp is an Application after a sync, as Argo CD 3.6 left it on a
// test cluster (trimmed).
const syncedApp = `{
  "apiVersion": "argoproj.io/v1alpha1", "kind": "Application",
  "metadata": {"name": "guestbook", "namespace": "argocd"},
  "spec": {
    "project": "default",
    "source": {"repoURL": "https://example.com/apps.repo", "path": "guestbook", "targetRevision": "HEAD"},
    "destination": {"server": "https://kubernetes.default.svc", "namespace": "guestbook"},
    "syncPolicy": {"syncOptions": ["CreateNamespace=true"]}
  },
  "status": {
    "health": {"status": "Healthy", "lastTransitionTime": "2026-11-15T08:19:24Z"},
    "reconciledAt": "2026-11-15T08:19:30Z",
    "resources": [
      {"kind": "Service", "name": "guestbook-ui", "namespace": "guestbook", "status": "Synced", "version": "v1"},
      {"group": "apps", "kind": "Deployment", "name": "guestbook-ui", "namespace": "guestbook", "status": "Synced", "version": "v1",
       "health": {"status": "Progressing", "message": "waiting for rollout"}},
      {"group": "example.com", "kind": "Widget", "name": "w", "namespace": "guestbook", "status": "OutOfSync", "version": "v1", "requiresPruning": true}
    ],
    "sync": {"status": "Synced", "revision": "8088f4c0"},
    "history": [
      {"id": 0, "revision": "8088f4c0", "deployedAt": "2026-11-15T08:19:22Z", "deployStartedAt": "2026-11-15T08:19:21Z",
       "initiatedBy": {"username": "kubebench:test"},
       "source": {"repoURL": "https://example.com/apps.repo", "path": "guestbook", "targetRevision": "HEAD"}}
    ],
    "operationState": {
      "phase": "Succeeded", "message": "successfully synced (all tasks run)",
      "startedAt": "2026-11-15T08:19:21Z", "finishedAt": "2026-11-15T08:19:22Z",
      "operation": {"initiatedBy": {"username": "kubebench:test"}, "sync": {"revision": "HEAD", "syncOptions": ["CreateNamespace=true"]}},
      "syncResult": {"revision": "8088f4c0", "resources": [
        {"group": "", "kind": "Service", "name": "guestbook-ui", "namespace": "guestbook", "status": "Synced",
         "message": "service/guestbook-ui created", "hookPhase": "Running", "syncPhase": "Sync"}
      ]}
    },
    "summary": {"images": ["gcr.io/google-samples/gb-frontend:v5"]}
  }
}`

func decodeObject(t *testing.T, doc string) *unstructured.Unstructured {
	t.Helper()
	var obj map[string]any
	if err := json.Unmarshal([]byte(doc), &obj); err != nil {
		t.Fatal(err)
	}
	return &unstructured.Unstructured{Object: obj}
}

func TestToDetailOfASyncedApplication(t *testing.T) {
	d := toDetail(decodeObject(t, syncedApp))
	wantSummary := Summary{
		Name: "guestbook", Namespace: "argocd", Project: "default",
		Repo: "https://example.com/apps.repo", Path: "guestbook", TargetRevision: "HEAD", SourceCount: 1,
		Destination: Destination{Server: "https://kubernetes.default.svc", Namespace: "guestbook"},
		Sync:        "Synced", Health: "Healthy", OperationPhase: "Succeeded",
		LastSyncedAt: "2026-11-15T08:19:22Z", Revision: "8088f4c0",
	}
	if !reflect.DeepEqual(d.Summary, wantSummary) {
		t.Fatalf("summary = %+v\nwant %+v", d.Summary, wantSummary)
	}
	if len(d.Resources) != 3 {
		t.Fatalf("resources = %+v", d.Resources)
	}
	if r := d.Resources[1]; r.CatalogKey != "" || r.Health != "Progressing" || r.HealthMessage != "waiting for rollout" {
		t.Fatalf("deployment = %+v", r)
	}
	if r := d.Resources[2]; !r.RequiresPruning || r.CatalogKey != "" {
		t.Fatalf("custom resource = %+v", r)
	}
	if len(d.History) != 1 || d.History[0].InitiatedBy != "kubebench:test" || d.History[0].Sources[0].Path != "guestbook" {
		t.Fatalf("history = %+v", d.History)
	}
	op := d.Operation
	if op == nil || op.Revision != "8088f4c0" || op.InitiatedBy != "kubebench:test" || len(op.Resources) != 1 ||
		op.Resources[0].Message != "service/guestbook-ui created" {
		t.Fatalf("operation = %+v", op)
	}
	if !reflect.DeepEqual(d.SyncPolicy, SyncPolicy{SyncOptions: []string{"CreateNamespace=true"}}) {
		t.Fatalf("sync policy = %+v", d.SyncPolicy)
	}
	if !reflect.DeepEqual(d.Images, []string{"gcr.io/google-samples/gb-frontend:v5"}) {
		t.Fatalf("images = %v", d.Images)
	}
}

func TestCatalogKeysNameKubeBenchLists(t *testing.T) {
	resources := toResources([]map[string]any{
		{"kind": "ConfigMap", "name": "a"},
		{"group": "apps", "kind": "StatefulSet", "name": "b"},
		{"group": "networking.k8s.io", "kind": "Ingress", "name": "c"},
	})
	for i, want := range []string{"configmaps", "statefulsets", "ingresses"} {
		if resources[i].CatalogKey != want {
			t.Fatalf("catalog key of %s = %q, want %q", resources[i].Kind, resources[i].CatalogKey, want)
		}
	}
}

// A new Application has no status, and hand-written objects may hold
// fields of the wrong type: neither may break the mapping.
func TestMappingIsDefensive(t *testing.T) {
	empty := decodeObject(t, `{"metadata": {"name": "new", "namespace": "argocd"}}`)
	d := toDetail(empty)
	if d.Sync != "Unknown" || d.Health != "Unknown" || d.Project != "default" || d.Operation != nil ||
		d.Resources == nil || d.History == nil || d.Sources == nil {
		t.Fatalf("empty detail = %+v", d)
	}
	odd := decodeObject(t, `{"metadata": {"name": "odd", "namespace": "argocd"},
	  "spec": {"source": "not-an-object", "syncPolicy": {"automated": true, "syncOptions": [1, "Prune=false"]}},
	  "status": {"sync": [], "resources": ["x", {"kind": "Pod", "name": "p"}], "history": [{"revision": "no-id"}, {"id": 2.0}]}}`)
	d = toDetail(odd)
	if d.AutoSync || len(d.Resources) != 1 || len(d.History) != 1 || d.History[0].ID != 2 ||
		!reflect.DeepEqual(d.SyncPolicy.SyncOptions, []string{"Prune=false"}) {
		t.Fatalf("odd detail = %+v", d)
	}
}

func TestAutoSyncAndOperating(t *testing.T) {
	cases := []struct {
		doc       string
		auto, ops bool
	}{
		{`{"spec": {"syncPolicy": {"automated": {}}}}`, true, false},
		{`{"spec": {"syncPolicy": {"automated": {"enabled": false}}}}`, false, false},
		{`{"spec": {"syncPolicy": {"automated": {"enabled": true}}}}`, true, false},
		{`{"operation": {"sync": {}}}`, false, true},
		{`{"status": {"operationState": {"phase": "Running"}}}`, false, true},
		{`{"status": {"operationState": {"phase": "Failed"}}}`, false, false},
	}
	for _, c := range cases {
		s := toSummary(decodeObject(t, c.doc))
		if s.AutoSync != c.auto || s.Operating != c.ops {
			t.Errorf("%s: autoSync=%v operating=%v, want %v %v", c.doc, s.AutoSync, s.Operating, c.auto, c.ops)
		}
	}
}
