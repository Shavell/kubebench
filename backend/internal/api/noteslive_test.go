package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"kubebench/internal/plugins/notes"
	"kubebench/internal/store"
	"kubebench/internal/watch"
)

// noEvent fails when an event named name arrives within wait.
func noEvent(t *testing.T, events <-chan sseEvent, name string, wait time.Duration) {
	t.Helper()
	timeout := time.After(wait)
	for {
		select {
		case ev, ok := <-events:
			if !ok {
				return
			}
			if ev.name == name {
				t.Fatalf("unexpected %q event: %s", name, ev.data)
			}
		case <-timeout:
			return
		}
	}
}

// TestNotesLiveUpdates: note changes reach the live-update streams of the
// users who may see the note, and nobody else's (§7.13): a shared note
// every user of its cluster, a personal note only its owner, on any
// cluster they watch.
func TestNotesLiveUpdates(t *testing.T) {
	e := newTestEnv(t, nil, &notes.Plugin{})
	e.srv.streams.watchTimings = watchTimings{heartbeat: time.Second, debounce: 20 * time.Millisecond}
	other, err := e.store.AddCluster(store.Cluster{Name: "other", Source: store.SourceKubeconfig, Kubeconfig: testKubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	// dave works on the other cluster only.
	e.addUser("dave", store.RoleUser, "")
	if err := e.store.SetAssignments(e.users["dave"], []store.Assignment{{ClusterID: other, Role: store.ClusterRoleOperator}}); err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetActiveCluster(e.users["dave"], other); err != nil {
		t.Fatal(err)
	}
	srv := httptest.NewServer(e.handler)
	t.Cleanup(srv.Close)

	streams := map[string]<-chan sseEvent{}
	for _, user := range []string{"admin", "alice", "bob", "dave"} {
		events := openWatch(t, e, srv, user, notes.Topic)
		var st watch.TopicStatus
		if err := json.Unmarshal([]byte(nextEvent(t, events, "status")), &st); err != nil {
			t.Fatal(err)
		}
		if st.Topic != notes.Topic || st.Mode != watch.ModeLive {
			t.Fatalf("%s: status %+v", user, st)
		}
		streams[user] = events
	}
	change := fmt.Sprintf(`{"topic":%q,"namespace":""}`, notes.Topic)
	expectChange := func(users ...string) {
		t.Helper()
		for _, user := range users {
			if got := nextEvent(t, streams[user], "change"); got != change {
				t.Fatalf("%s: change = %s", user, got)
			}
		}
	}
	quiet := func(users ...string) {
		t.Helper()
		for _, user := range users {
			noEvent(t, streams[user], "change", 200*time.Millisecond)
		}
	}

	shared := e.createNote("alice", "shared", map[string]any{"title": "runbook"})
	expectChange("admin", "alice", "bob")
	quiet("dave")

	personal := e.createNote("bob", "personal", map[string]any{"title": "mine"})
	expectChange("bob")
	quiet("admin", "alice", "dave")

	// A global personal note reaches its owner on another cluster too.
	e.createNote("dave", "personal", map[string]any{"title": "everywhere", "global": true})
	expectChange("dave")
	quiet("admin", "alice", "bob")

	path := fmt.Sprintf("%s/notes/shared/%d", notesAPI, shared.ID)
	expect(t, e.do(http.MethodPut, path+"/pin", "alice", map[string]bool{"pinned": true}), http.StatusOK, "")
	expectChange("admin", "alice", "bob")
	expect(t, e.do(http.MethodDelete, path, "admin", nil), http.StatusOK, "")
	expectChange("admin", "alice", "bob")
	expect(t, e.do(http.MethodPut, fmt.Sprintf("%s/notes/personal/%d", notesAPI, personal.ID), "bob",
		map[string]any{"title": "mine v2", "version": 1}), http.StatusOK, "")
	expectChange("bob")
	quiet("admin", "alice", "dave")

	// Refused changes publish nothing.
	expect(t, e.do(http.MethodPost, notesAPI+"/notes/shared", "bob", map[string]any{"title": "x"}), http.StatusForbidden, "")
	quiet("admin", "alice", "bob")

	if n := e.srv.board.Subscribers(); n != 4 {
		t.Fatalf("board subscribers = %d", n)
	}
}

func TestParseWatchTopicsInternal(t *testing.T) {
	got, err := parseWatchTopics("pods,kubebench:notes")
	if err != nil || len(got) != 2 || got[1] != "kubebench:notes" {
		t.Fatalf("topics = %v, %v", got, err)
	}
	for _, raw := range []string{"kubebench:", "kubebench:Notes", "other:notes", "kubebench:notes/x"} {
		if _, err := parseWatchTopics(raw); !hasCode(err, "bad_request") {
			t.Errorf("parseWatchTopics(%q): %v", raw, err)
		}
	}
}
