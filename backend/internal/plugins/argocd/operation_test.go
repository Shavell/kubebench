package argocd

import (
	"errors"
	"net/http"
	"reflect"
	"testing"
)

// guestbookApp is a single-source Application as Argo CD stores it.
func guestbookApp() map[string]any {
	return map[string]any{
		"spec": map[string]any{
			"project": "default",
			"source": map[string]any{
				"repoURL":        "https://example.com/apps.repo",
				"path":           "guestbook",
				"targetRevision": "main",
			},
			"syncPolicy": map[string]any{
				"syncOptions": []any{"CreateNamespace=true", "ServerSideApply=true"},
			},
		},
		"status": map[string]any{
			"history": []any{
				map[string]any{
					"id": int64(0), "revision": "aaa111",
					"source": map[string]any{"repoURL": "https://example.com/apps.repo", "path": "guestbook", "targetRevision": "v1"},
				},
				map[string]any{
					"id": int64(1), "revision": "bbb222",
					"source": map[string]any{"repoURL": "https://example.com/apps.repo", "path": "guestbook", "targetRevision": "main"},
				},
			},
		},
	}
}

func requireStatus(t *testing.T, err error, status int) {
	t.Helper()
	var re *requestError
	if !errors.As(err, &re) || re.status != status {
		t.Fatalf("error = %v, want a %d request error", err, status)
	}
}

// The controller does not read spec.syncPolicy.syncOptions for a sync
// requested through .operation (CreateNamespace=true was ignored on a
// real cluster until it was in the operation), so they must be copied in.
func TestSyncOperationCopiesTheApplicationsSyncOptions(t *testing.T) {
	op, err := syncOperation(guestbookApp(), SyncRequest{}, "alice")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"initiatedBy": map[string]any{"username": "kubebench:alice"},
		"sync": map[string]any{
			"revision":    "main",
			"syncOptions": []any{"CreateNamespace=true", "ServerSideApply=true"},
		},
	}
	if !reflect.DeepEqual(op, want) {
		t.Fatalf("operation = %#v\nwant %#v", op, want)
	}
}

func TestSyncOperationHonoursTheRequest(t *testing.T) {
	none := []string{}
	only := []string{"Replace=true"}
	tests := []struct {
		name string
		req  SyncRequest
		want map[string]any
	}{
		{
			name: "revision, prune, dry run and resources",
			req: SyncRequest{
				Revision: "abc123", Prune: true, DryRun: true,
				Resources: []ResourceKey{{Group: "apps", Kind: "Deployment", Namespace: "web", Name: "ui"}},
			},
			want: map[string]any{
				"revision": "abc123", "prune": true, "dryRun": true,
				"resources":   []any{map[string]any{"group": "apps", "kind": "Deployment", "namespace": "web", "name": "ui"}},
				"syncOptions": []any{"CreateNamespace=true", "ServerSideApply=true"},
			},
		},
		{
			name: "own sync options replace the application's",
			req:  SyncRequest{SyncOptions: &only},
			want: map[string]any{"revision": "main", "syncOptions": []any{"Replace=true"}},
		},
		{
			name: "an empty list syncs without options",
			req:  SyncRequest{SyncOptions: &none},
			want: map[string]any{"revision": "main"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			op, err := syncOperation(guestbookApp(), tt.req, "alice")
			if err != nil {
				t.Fatal(err)
			}
			if got := op["sync"]; !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("sync = %#v\nwant %#v", got, tt.want)
			}
		})
	}
}

func TestSyncOperationMultiSource(t *testing.T) {
	app := map[string]any{"spec": map[string]any{"sources": []any{
		map[string]any{"repoURL": "https://charts.example.com", "chart": "redis", "targetRevision": "19.0.0"},
		map[string]any{"repoURL": "https://example.com/values.repo", "targetRevision": "HEAD", "ref": "values"},
	}}}
	op, err := syncOperation(app, SyncRequest{}, "bob")
	if err != nil {
		t.Fatal(err)
	}
	if got := op["sync"]; !reflect.DeepEqual(got, map[string]any{"revisions": []any{"19.0.0", "HEAD"}}) {
		t.Fatalf("sync = %#v", got)
	}
	_, err = syncOperation(app, SyncRequest{Revision: "main"}, "bob")
	requireStatus(t, err, http.StatusBadRequest)
}

func TestSyncOperationRejectsIncompleteResources(t *testing.T) {
	_, err := syncOperation(guestbookApp(), SyncRequest{Resources: []ResourceKey{{Kind: "Service"}}}, "alice")
	requireStatus(t, err, http.StatusBadRequest)
}

func TestRollbackOperation(t *testing.T) {
	id := int64(0)
	op, err := rollbackOperation(guestbookApp(), RollbackRequest{HistoryID: &id, Prune: true}, "alice")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]any{
		"revision":     "aaa111",
		"source":       map[string]any{"repoURL": "https://example.com/apps.repo", "path": "guestbook", "targetRevision": "v1"},
		"prune":        true,
		"syncStrategy": map[string]any{"apply": map[string]any{}},
		"syncOptions":  []any{"CreateNamespace=true", "ServerSideApply=true"},
	}
	if got := op["sync"]; !reflect.DeepEqual(got, want) {
		t.Fatalf("sync = %#v\nwant %#v", got, want)
	}
	if got := op["initiatedBy"]; !reflect.DeepEqual(got, map[string]any{"username": "kubebench:alice"}) {
		t.Fatalf("initiatedBy = %#v", got)
	}
}

func TestRollbackOperationRefusals(t *testing.T) {
	id, missing := int64(1), int64(7)
	automated := guestbookApp()
	automated["spec"].(map[string]any)["syncPolicy"] = map[string]any{"automated": map[string]any{"prune": true}}
	switchedOff := guestbookApp()
	switchedOff["spec"].(map[string]any)["syncPolicy"] = map[string]any{"automated": map[string]any{"enabled": false}}
	legacy := guestbookApp()
	legacy["status"] = map[string]any{"history": []any{map[string]any{"id": int64(1), "revision": "abc"}}}

	_, err := rollbackOperation(automated, RollbackRequest{HistoryID: &id}, "alice")
	requireStatus(t, err, http.StatusConflict)
	_, err = rollbackOperation(guestbookApp(), RollbackRequest{HistoryID: &missing}, "alice")
	requireStatus(t, err, http.StatusNotFound)
	_, err = rollbackOperation(guestbookApp(), RollbackRequest{}, "alice")
	requireStatus(t, err, http.StatusBadRequest)
	_, err = rollbackOperation(legacy, RollbackRequest{HistoryID: &id}, "alice")
	requireStatus(t, err, http.StatusBadRequest)
	if _, err := rollbackOperation(switchedOff, RollbackRequest{HistoryID: &id}, "alice"); err != nil {
		t.Fatalf("automated sync switched off: %v", err)
	}
}

func TestRollbackOperationMultiSource(t *testing.T) {
	app := map[string]any{"status": map[string]any{"history": []any{map[string]any{
		"id":        int64(3),
		"revisions": []any{"19.0.0", "ccc333"},
		"sources": []any{
			map[string]any{"repoURL": "https://charts.example.com", "chart": "redis", "targetRevision": "19.0.0"},
			map[string]any{"repoURL": "https://example.com/values.repo", "targetRevision": "HEAD", "ref": "values"},
		},
	}}}}
	id := int64(3)
	op, err := rollbackOperation(app, RollbackRequest{HistoryID: &id}, "alice")
	if err != nil {
		t.Fatal(err)
	}
	sync := op["sync"].(map[string]any)
	if !reflect.DeepEqual(sync["revisions"], []any{"19.0.0", "ccc333"}) || len(sync["sources"].([]any)) != 2 {
		t.Fatalf("sync = %#v", sync)
	}
	if _, single := sync["revision"]; single {
		t.Fatalf("multi-source rollback names a single revision: %#v", sync)
	}
}
