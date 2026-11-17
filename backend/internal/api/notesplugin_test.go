package api

import (
	"fmt"
	"net/http"
	"strings"
	"testing"

	"kubebench/internal/plugin"
	"kubebench/internal/plugins/notes"
	"kubebench/internal/store"
)

const notesAPI = "/api/plugins/notes"

func (e *testEnv) createNote(user, visibility string, body map[string]any) notes.Note {
	e.t.Helper()
	rec := e.do(http.MethodPost, notesAPI+"/notes/"+visibility, user, body)
	expect(e.t, rec, http.StatusOK, "")
	return decode[notes.Note](e.t, rec)
}

func noteTitles(list notes.NoteList) []string {
	out := []string{}
	for _, n := range list.Items {
		out = append(out, n.Title)
	}
	return out
}

// TestNotesPersonalIsolation: a personal note is its owner's alone. Other
// users — admins included — cannot read, edit, pin or delete it by id,
// and get the same 404 as for a note that does not exist.
func TestNotesPersonalIsolation(t *testing.T) {
	e := newTestEnv(t, nil, &notes.Plugin{})
	n := e.createNote("bob", "personal", map[string]any{"title": "bob's", "body": "secret plan"})
	if n.Visibility != "personal" || n.CreatedBy != "bob" || n.ClusterID == nil || *n.ClusterID != e.cluster || n.Version != 1 {
		t.Fatalf("created %+v", n)
	}
	path := fmt.Sprintf("%s/notes/%d", notesAPI, n.ID)
	personal := fmt.Sprintf("%s/notes/personal/%d", notesAPI, n.ID)
	for _, user := range []string{"alice", "admin"} {
		expect(t, e.do(http.MethodGet, path, user, nil), http.StatusNotFound, plugin.CodeNotFound)
		expect(t, e.do(http.MethodPut, personal, user, map[string]any{"title": "mine", "version": 1}), http.StatusNotFound, plugin.CodeNotFound)
		expect(t, e.do(http.MethodPut, personal+"/pin", user, map[string]bool{"pinned": true}), http.StatusNotFound, plugin.CodeNotFound)
		expect(t, e.do(http.MethodDelete, personal, user, nil), http.StatusNotFound, plugin.CodeNotFound)
		// Nor under the shared path, which admins may use.
		expect(t, e.do(http.MethodDelete, fmt.Sprintf("%s/notes/shared/%d", notesAPI, n.ID), user, nil), http.StatusNotFound, plugin.CodeNotFound)
		if got := noteTitles(decode[notes.NoteList](t, e.do(http.MethodGet, notesAPI+"/notes?clusters=all", user, nil))); len(got) != 0 {
			t.Fatalf("%s lists %v", user, got)
		}
	}
	expect(t, e.do(http.MethodGet, fmt.Sprintf("%s/notes/%d", notesAPI, 9999), "bob", nil), http.StatusNotFound, plugin.CodeNotFound)

	got := decode[notes.Note](t, e.do(http.MethodGet, path, "bob", nil))
	if got.Body != "secret plan" {
		t.Fatalf("owner reads %+v", got)
	}
	expect(t, e.do(http.MethodPut, personal, "bob", map[string]any{"title": "bob's v2", "body": "x", "version": 1}), http.StatusOK, "")
	expect(t, e.do(http.MethodDelete, personal, "bob", nil), http.StatusOK, "")
	// Only the attempts under the shared path were audited.
	for _, row := range e.audit() {
		if !strings.Contains(row.Action, "/notes/shared/") {
			t.Fatalf("a personal note request was audited: %+v", row)
		}
	}
}

// TestNotesSharedPermissions: everyone with a role reads shared notes;
// viewers cannot write them; a read-only cluster does not block them;
// operators edit any shared note but delete only their own; admins delete
// any. Every shared change is audited without the note's text.
func TestNotesSharedPermissions(t *testing.T) {
	e := newTestEnv(t, nil, &notes.Plugin{})
	body := map[string]any{
		"title": "Deploy freeze", "body": "no deploys on Friday", "tags": []string{"Runbook"},
		"resource": map[string]string{"apiVersion": "apps/v1", "kind": "Deployment", "namespace": "default", "name": "web"},
	}
	expect(t, e.do(http.MethodPost, notesAPI+"/notes/shared", "bob", body), http.StatusForbidden, plugin.CodeForbiddenRole)
	if row := e.lastAudit(); row.Username != "bob" || row.Status != http.StatusForbidden || row.Target != "note" {
		t.Fatalf("refused create: audit row %+v", row)
	}

	// Read-only protects the cluster, not KubeBench's notes.
	readOnly := true
	if err := e.store.UpdateCluster(e.cluster, store.ClusterUpdate{ReadOnly: &readOnly}); err != nil {
		t.Fatal(err)
	}
	n := e.createNote("alice", "shared", body)
	if n.Tags[0] != "runbook" || n.Resource == nil || n.Resource.Name != "web" {
		t.Fatalf("created %+v", n)
	}
	target := fmt.Sprintf("note/%d", n.ID)
	row := e.lastAudit()
	if row.Username != "alice" || row.Status != http.StatusOK || row.Target != target ||
		row.Action != "POST /api/plugins/notes/notes/shared" || row.Detail != `{"resource":"Deployment/default/web"}` {
		t.Fatalf("create: audit row %+v", row)
	}

	for _, user := range []string{"bob", "admin", "alice"} {
		list := decode[notes.NoteList](t, e.do(http.MethodGet, notesAPI+"/notes?scope=shared&kind=Deployment&namespace=default&name=web", user, nil))
		if len(list.Items) != 1 || list.Items[0].Body != "" {
			t.Fatalf("%s lists %+v", user, list)
		}
	}
	shared := fmt.Sprintf("%s/notes/shared/%d", notesAPI, n.ID)
	edit := map[string]any{"title": "Deploy freeze (Fri)", "body": "no deploys on Friday afternoon", "version": 1}
	expect(t, e.do(http.MethodPut, shared, "bob", edit), http.StatusForbidden, plugin.CodeForbiddenRole)
	expect(t, e.do(http.MethodPut, shared+"/pin", "bob", map[string]bool{"pinned": true}), http.StatusForbidden, plugin.CodeForbiddenRole)
	expect(t, e.do(http.MethodDelete, shared, "bob", nil), http.StatusForbidden, plugin.CodeForbiddenRole)

	updated := decode[notes.Note](t, e.do(http.MethodPut, shared, "admin", edit))
	if updated.Version != 2 || updated.UpdatedBy != "admin" || updated.CreatedBy != "alice" {
		t.Fatalf("admin edit: %+v", updated)
	}
	// alice still holds version 1: her save is a conflict, not a lost update.
	expect(t, e.do(http.MethodPut, shared, "alice", edit), http.StatusConflict, plugin.CodeConflict)
	expect(t, e.do(http.MethodPut, shared+"/pin", "alice", map[string]bool{"pinned": true}), http.StatusOK, "")
	if row := e.lastAudit(); row.Target != target || row.Detail != `{"pinned":true}` {
		t.Fatalf("pin: audit row %+v", row)
	}
	// A shared note is not reachable under the personal path.
	expect(t, e.do(http.MethodDelete, fmt.Sprintf("%s/notes/personal/%d", notesAPI, n.ID), "alice", nil), http.StatusNotFound, plugin.CodeNotFound)

	other := e.createNote("admin", "shared", map[string]any{"title": "admin's"})
	expect(t, e.do(http.MethodDelete, fmt.Sprintf("%s/notes/shared/%d", notesAPI, other.ID), "alice", nil), http.StatusForbidden, plugin.CodeForbidden)
	expect(t, e.do(http.MethodDelete, shared, "alice", nil), http.StatusOK, "")
	e.createNote("alice", "shared", map[string]any{"title": "alice's"})
	expect(t, e.do(http.MethodDelete, fmt.Sprintf("%s/notes/shared/%d", notesAPI, other.ID+1), "admin", nil), http.StatusOK, "")

	for _, row := range e.audit() {
		if strings.Contains(row.Detail, "Friday") || strings.Contains(row.Detail, "freeze") {
			t.Fatalf("an audit row carries note text: %+v", row)
		}
	}
	if m := e.mutations(); len(m) != 0 {
		t.Fatalf("notes changed the cluster: %v", m)
	}
}

func TestNotesValidationAndQueries(t *testing.T) {
	e := newTestEnv(t, nil, &notes.Plugin{})
	for name, body := range map[string]map[string]any{
		"empty title":         {"title": "  "},
		"long title":          {"title": strings.Repeat("x", 201)},
		"multi-line title":    {"title": "a\nb"},
		"large body":          {"title": "x", "body": strings.Repeat("x", 64<<10+1)},
		"bad tag":             {"title": "x", "tags": []string{"no spaces"}},
		"too many tags":       {"title": "x", "tags": strings.Fields("a b c d e f g h i j k l m n o p q")},
		"bad kind":            {"title": "x", "resource": map[string]string{"kind": "no-kind", "name": "x"}},
		"object without name": {"title": "x", "resource": map[string]string{"kind": "Pod"}},
		"global object":       {"title": "x", "global": true, "resource": map[string]string{"kind": "Pod", "name": "p"}},
		"unknown field":       {"title": "x", "visibility": "shared"},
	} {
		if rec := e.do(http.MethodPost, notesAPI+"/notes/personal", "bob", body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: %d %s", name, rec.Code, rec.Body.String())
		}
	}
	expect(t, e.do(http.MethodPost, notesAPI+"/notes/shared", "alice", map[string]any{"title": "x", "global": true}), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodGet, notesAPI+"/notes?scope=public", "bob", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodGet, notesAPI+"/notes?limit=501", "bob", nil), http.StatusBadRequest, plugin.CodeBadRequest)

	global := e.createNote("bob", "personal", map[string]any{"title": "Kubectl tricks", "body": "use -o wide", "global": true, "tags": []string{"cli"}})
	if global.ClusterID != nil || global.ClusterName != "" {
		t.Fatalf("global note: %+v", global)
	}
	e.createNote("bob", "personal", map[string]any{
		"title": "web pod", "tags": []string{"cli", "web"},
		"resource": map[string]string{"apiVersion": "v1", "kind": "Pod", "namespace": "default", "name": "web-1"},
	})
	e.createNote("alice", "shared", map[string]any{
		"title": "web deployment", "body": "owned by team shop",
		"resource": map[string]string{"apiVersion": "apps/v1", "kind": "Deployment", "namespace": "default", "name": "web"},
	})

	list := decode[notes.NoteList](t, e.do(http.MethodGet, notesAPI+"/notes?q=WIDE&body=true", "bob", nil))
	if len(list.Items) != 1 || list.Items[0].Body != "use -o wide" {
		t.Fatalf("search: %+v", list)
	}
	if got := noteTitles(decode[notes.NoteList](t, e.do(http.MethodGet, notesAPI+"/notes?tag=web", "bob", nil))); len(got) != 1 || got[0] != "web pod" {
		t.Fatalf("tag filter: %v", got)
	}
	tags := decode[struct {
		Tags []store.TagCount `json:"tags"`
	}](t, e.do(http.MethodGet, notesAPI+"/tags", "bob", nil))
	if len(tags.Tags) != 2 || tags.Tags[0] != (store.TagCount{Tag: "cli", Count: 2}) {
		t.Fatalf("tags: %+v", tags)
	}
	counts := decode[struct {
		Items []notes.ObjectCount `json:"items"`
	}](t, e.do(http.MethodGet, notesAPI+"/counts?namespace=default", "bob", nil))
	if len(counts.Items) != 2 {
		t.Fatalf("counts: %+v", counts)
	}
	for _, c := range counts.Items {
		if (c.Kind == "Pod" && c.Personal != 1) || (c.Kind == "Deployment" && c.Shared != 1) {
			t.Fatalf("counts: %+v", counts)
		}
	}
	// alice does not count bob's personal note.
	counts = decode[struct {
		Items []notes.ObjectCount `json:"items"`
	}](t, e.do(http.MethodGet, notesAPI+"/counts?kind=Pod&namespace=default&name=web-1", "alice", nil))
	if len(counts.Items) != 0 {
		t.Fatalf("alice counts bob's note: %+v", counts)
	}
}
