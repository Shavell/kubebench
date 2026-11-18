package api

import (
	"context"
	"io"
	"net/http"
	"strings"
	"testing"
	"time"

	"kubebench/internal/plugin"
)

// shutdown runs Server.Shutdown as main does on SIGTERM.
func shutdown(t *testing.T, e *testEnv) <-chan error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		done <- e.srv.Shutdown(ctx)
	}()
	return done
}

// On shutdown a terminal is hung up like an idle one, so the shell is
// told to finish instead of being left running in its container, and the
// client learns why.
func TestShutdownHangsUpTerminals(t *testing.T) {
	e, srv, sh := terminalEnv(t)
	conn, _, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
	if err != nil {
		t.Fatal(err)
	}
	send(t, conn, `{"type":"input","data":"hi\r"}`)
	readOutput(t, conn, "echo:hi")

	done := shutdown(t, e)
	if msg := readEnd(t, conn); msg["type"] != "error" || msg["message"] != errShuttingDown.Error() || msg["reason"] != "api.shutting_down" {
		t.Fatalf("final message = %v, want the shutdown", msg)
	}
	if err := <-done; err != nil {
		t.Fatalf("Shutdown = %v, want it to wait for the terminal", err)
	}
	shell := sh.started()[0]
	if got := shell.received(); got != "hi\r"+interruptKey+endOfInputKey {
		t.Fatalf("shell input = %q, want the line, then Ctrl+C and Ctrl+D", got)
	}
	if err, _ := shell.ctxErr.Load().(error); err != nil {
		t.Fatalf("the shell was cut off (%v) instead of finishing", err)
	}

	// No terminal starts once the server stops.
	_, resp, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
	refused(t, resp, err, http.StatusServiceUnavailable, plugin.CodeInternal)
}

// Watch streams end at once with the reason; http.Server.Shutdown would
// otherwise wait for them until its deadline.
func TestShutdownEndsWatchStreams(t *testing.T) {
	e, _, srv := watchEnv(t)
	events := openWatch(t, e, srv, "bob", "pods")
	nextEvent(t, events, "status")
	done := shutdown(t, e)
	if got := nextEvent(t, events, "end"); got != `{"error":"the server is shutting down","reason":"api.shutting_down"}` {
		t.Fatalf("end = %q, want the shutdown", got)
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestShutdownEndsLogStreams(t *testing.T) {
	e := newTestEnv(t, nil)
	w := &syncRecorder{header: http.Header{}}
	sse, err := startSSE(w, time.Second)
	if err != nil {
		t.Fatal(err)
	}
	logR, logW := io.Pipe()
	ctx, cancel := e.srv.streamContext(context.Background())
	defer cancel()
	stopped := make(chan struct{})
	go func() {
		defer close(stopped)
		streamLogs(ctx, sse, []logFeed{feed("a", func(context.Context) (io.ReadCloser, error) {
			return logR, nil
		})}, 1, time.Hour, nil)
	}()
	if _, err := io.WriteString(logW, "hello\n"); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(5 * time.Second); !strings.Contains(w.String(), "hello"); time.Sleep(5 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("no line in %q", w.String())
		}
	}
	if err := <-shutdown(t, e); err != nil {
		t.Fatal(err)
	}
	select {
	case <-stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the log stream outlived the shutdown")
	}
	if got := w.String(); !strings.HasSuffix(got, "event: error\ndata: {\"error\":\"the server is shutting down\",\"reason\":\"api.shutting_down\"}\n\n") || strings.Contains(got, "event: end") {
		t.Fatalf("stream = %q, want it to end with the shutdown error", got)
	}
}
