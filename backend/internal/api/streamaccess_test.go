package api

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
	"kubebench/internal/store"
)

// A terminal ends within a ping interval once its user loses the access
// it started with, whichever way access is lost.
func TestTerminalEndsWhenAccessIsRevoked(t *testing.T) {
	revocations := map[string]func(e *testEnv) error{
		"logout":       func(e *testEnv) error { return e.store.DeleteSession(e.tokens["alice"]) },
		"user deleted": func(e *testEnv) error { return e.store.DeleteUser(e.users["alice"]) },
		"demoted": func(e *testEnv) error {
			return e.store.SetAssignments(e.users["alice"], []store.Assignment{{ClusterID: e.cluster, Role: store.ClusterRoleViewer}})
		},
		"unassigned": func(e *testEnv) error { return e.store.SetAssignments(e.users["alice"], nil) },
		"read-only": func(e *testEnv) error {
			readOnly := true
			return e.store.UpdateCluster(e.cluster, store.ClusterUpdate{ReadOnly: &readOnly})
		},
		"cluster removed": func(e *testEnv) error { return e.store.DeleteCluster(e.cluster) },
	}
	for name, revoke := range revocations {
		t.Run(name, func(t *testing.T) {
			e, srv, sh := terminalEnv(t, func(s *streamSettings) { s.terminalPing = 50 * time.Millisecond })
			conn, _, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
			if err != nil {
				t.Fatal(err)
			}
			send(t, conn, `{"type":"input","data":"before\r"}`)
			readOutput(t, conn, "echo:before")
			if err := revoke(e); err != nil {
				t.Fatal(err)
			}
			msg := readEnd(t, conn)
			if msg["type"] != "error" || !strings.Contains(fmt.Sprint(msg["message"]), "the session was closed") || msg["reason"] != "streams.closed" {
				t.Fatalf("final message = %v, want the session closed", msg)
			}
			select {
			case <-sh.started()[0].stopped:
			case <-time.After(5 * time.Second):
				t.Fatal("the shell kept running after access was revoked")
			}
		})
	}
}

// Switching the active cluster does not end a terminal on the other one.
func TestTerminalSurvivesAClusterSwitch(t *testing.T) {
	e, srv, sh := terminalEnv(t, func(s *streamSettings) { s.terminalPing = 50 * time.Millisecond })
	conn, _, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
	if err != nil {
		t.Fatal(err)
	}
	other, err := e.store.AddCluster(store.Cluster{Name: "other", Source: store.SourceKubeconfig, Kubeconfig: testKubeconfig})
	if err != nil {
		t.Fatal(err)
	}
	if err := e.store.SetActiveCluster(e.users["alice"], other); err != nil {
		t.Fatal(err)
	}
	for i := range 5 { // a few pings, answered while reading
		send(t, conn, fmt.Sprintf(`{"type":"input","data":"still here %d\r"}`, i))
		readOutput(t, conn, fmt.Sprintf("echo:still here %d", i))
		time.Sleep(40 * time.Millisecond)
	}
	select {
	case <-sh.started()[0].stopped:
		t.Fatal("a cluster switch ended the terminal")
	default:
	}
}

// A log stream checks the caller's access at every heartbeat and ends
// with the reason once it is gone.
func TestStreamLogsEndsWhenCheckFails(t *testing.T) {
	w := &syncRecorder{header: http.Header{}}
	sse, err := startSSE(w, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	logR, logW := io.Pipe()
	defer logW.Close()
	var revoked atomic.Bool
	check := func() error {
		if revoked.Load() {
			return fail(http.StatusForbidden, plugin.CodeForbidden, reasonAccessLost, reason.Params{"cluster": "test"})
		}
		return nil
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		streamLogs(context.Background(), sse, []logFeed{feed("a", func(context.Context) (io.ReadCloser, error) {
			return logR, nil
		})}, 1, 10*time.Millisecond, check, nil)
	}()
	if _, err := io.WriteString(logW, "hello\n"); err != nil {
		t.Fatal(err)
	}
	revoked.Store(true)
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("the stream outlived the caller's access")
	}
	// The reason names the refusal as its cause, for the UI to translate.
	want := `{"error":"the session was closed: you no longer have access to cluster \"test\"","reason":"streams.closed",` +
		`"params":{"cause":{"error":"you no longer have access to cluster \"test\"","reason":"cluster.access_lost","params":{"cluster":"test"}}}}`
	if !strings.HasSuffix(w.String(), "event: error\ndata: "+want+"\n\n") {
		t.Fatalf("stream = %q, want it to end with the reason", w.String())
	}
	if _, err := logW.Write([]byte("more\n")); err == nil {
		t.Fatal("the log was left open")
	}
}

func TestStillAllowed(t *testing.T) {
	e := newTestEnv(t, nil)
	cluster, err := e.store.GetCluster(e.cluster)
	if err != nil {
		t.Fatal(err)
	}
	bob := &call{token: e.tokens["bob"], cluster: cluster}
	if err := e.srv.stillAllowed(bob, accessRead); err != nil {
		t.Fatalf("a viewer may still read: %v", err)
	}
	if err := e.srv.stillAllowed(bob, accessExec); !hasCode(err, plugin.CodeForbiddenRole) {
		t.Fatalf("a viewer's exec: %v, want forbidden_role", err)
	}
	if err := e.store.SetAssignments(e.users["bob"], nil); err != nil {
		t.Fatal(err)
	}
	if err := e.srv.stillAllowed(bob, accessRead); !hasCode(err, plugin.CodeForbidden) {
		t.Fatalf("after the assignment was removed: %v, want forbidden", err)
	}
	if err := e.store.SetMustChangePassword(e.users["admin"], true); err != nil {
		t.Fatal(err)
	}
	admin := &call{token: e.tokens["admin"], cluster: cluster}
	if err := e.srv.stillAllowed(admin, accessRead); !hasCode(err, plugin.CodePasswordChangeRequired) {
		t.Fatalf("with a password change due: %v", err)
	}
}

func TestStreamLimiter(t *testing.T) {
	l := newStreamLimiter()
	var releases []func()
	for range maxTerminalsPerUser {
		release, err := l.acquire(1, kindTerminal)
		if err != nil {
			t.Fatal(err)
		}
		releases = append(releases, release)
	}
	_, err := l.acquire(1, kindTerminal)
	if !hasCode(err, plugin.CodeRateLimited) {
		t.Fatalf("terminal %d: %v, want rate_limited", maxTerminalsPerUser+1, err)
	}
	if _, err := l.acquire(2, kindTerminal); err != nil {
		t.Fatalf("another user is limited too: %v", err)
	}
	if _, err := l.acquire(1, kindLogStream); err != nil {
		t.Fatalf("log streams share the terminals' limit: %v", err)
	}
	releases[0]()
	releases[0]() // a second release gives nothing back
	if _, err := l.acquire(1, kindTerminal); err != nil {
		t.Fatalf("after a release: %v", err)
	}
	if _, err := l.acquire(1, kindTerminal); !hasCode(err, plugin.CodeRateLimited) {
		t.Fatalf("a double release freed two slots: %v", err)
	}
}

// Over the limit, a terminal is refused before the upgrade.
func TestTerminalLimitPerUser(t *testing.T) {
	e, srv, _ := terminalEnv(t)
	for i := range maxTerminalsPerUser {
		if _, _, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil); err != nil {
			t.Fatalf("terminal %d: %v", i+1, err)
		}
	}
	_, resp, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
	refused(t, resp, err, http.StatusTooManyRequests, plugin.CodeRateLimited)
	if _, _, err := dial(t, e, srv, "admin", "ns=default&pod=web-1&container=app", nil); err != nil {
		t.Fatalf("another user's terminal: %v", err)
	}
}

// A client that stops reading ends its log stream once a write has waited
// logWriteWait, and the stream closes its log, instead of holding both
// open for as long as the client keeps the connection.
func TestLogStreamStalledClient(t *testing.T) {
	logR, logW := io.Pipe()
	defer logW.Close()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		sse, err := startSSE(w, 200*time.Millisecond)
		defer sse.end()
		if err != nil {
			return
		}
		streamLogs(r.Context(), sse, []logFeed{feed("a", func(context.Context) (io.ReadCloser, error) {
			return logR, nil
		})}, 1, time.Hour, nil, nil)
	}))
	defer srv.Close()

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET / HTTP/1.1\r\nHost: kubebench\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	// The client reads nothing more; the log keeps writing until the
	// stream gives up and closes it.
	line := []byte(strings.Repeat("x", 32<<10) + "\n")
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, err := logW.Write(line); err != nil {
				return
			}
		}
	}()
	select {
	case <-done:
	case <-time.After(20 * time.Second):
		t.Fatal("a client that stopped reading kept the log stream open")
	}
}
