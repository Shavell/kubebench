package main

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// startServer serves handler with newHTTPServer's settings, its read
// timeout shortened to timeout.
func startServer(t *testing.T, handler http.Handler, timeout time.Duration) *httptest.Server {
	t.Helper()
	srv := httptest.NewUnstartedServer(handler)
	srv.Config = newHTTPServer("", handler)
	if srv.Config.ReadTimeout != readTimeout || srv.Config.ReadHeaderTimeout <= 0 || srv.Config.WriteTimeout != 0 {
		t.Fatalf("timeouts: read %v, header %v, write %v", srv.Config.ReadTimeout, srv.Config.ReadHeaderTimeout, srv.Config.WriteTimeout)
	}
	srv.Config.ReadTimeout = timeout
	srv.Start()
	t.Cleanup(srv.Close)
	return srv
}

// send writes raw HTTP/1.1 bytes and reads the response; the connection is
// closed before the server (cleanups run last in, first out).
func send(t *testing.T, srv *httptest.Server, raw string) (*http.Response, error) {
	t.Helper()
	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { conn.Close() })
	if _, err := io.WriteString(conn, raw); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	return http.ReadResponse(bufio.NewReader(conn), nil)
}

// A client that announces a body and never sends it is cut off.
func TestServerCutsStalledBodies(t *testing.T) {
	readErr := make(chan error, 1)
	srv := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := io.ReadAll(r.Body)
		readErr <- err
		w.WriteHeader(http.StatusBadRequest)
	}), 200*time.Millisecond)

	if res, err := send(t, srv, "POST /api/auth/login HTTP/1.1\r\nHost: kubebench\r\nContent-Length: 1000000\r\n\r\n{"); err == nil {
		res.Body.Close()
	}
	select {
	case err := <-readErr:
		if err == nil {
			t.Fatal("reading a stalled body succeeded")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the stalled body was never cut off")
	}
}

// The read timeout ends once the request has been read: streams and long
// operations keep their connection and their request context.
func TestServerReadTimeoutSparesLongHandlers(t *testing.T) {
	const timeout = 200 * time.Millisecond
	ctxErr := make(chan error, 1)
	srv := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, err := io.ReadAll(r.Body); err != nil {
			t.Errorf("read body: %v", err)
		}
		w.WriteHeader(http.StatusOK)
		_ = http.NewResponseController(w).Flush()
		time.Sleep(3 * timeout)
		ctxErr <- r.Context().Err()
		_, _ = io.WriteString(w, "done")
	}), timeout)

	for _, raw := range []string{
		"GET /api/logs/stream HTTP/1.1\r\nHost: kubebench\r\n\r\n",
		"POST /api/nodes/drain HTTP/1.1\r\nHost: kubebench\r\nContent-Length: 2\r\n\r\n{}",
	} {
		res, err := send(t, srv, raw)
		if err != nil {
			t.Fatal(err)
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if err := <-ctxErr; err != nil || !strings.Contains(string(body), "done") {
			t.Fatalf("%q: context %v, body %q", strings.SplitN(raw, "\r", 2)[0], err, body)
		}
	}
}

// A hijacked connection (a WebSocket terminal) keeps no read deadline: the
// client may stay silent for longer than the read timeout.
func TestServerReadTimeoutSparesHijackedConnections(t *testing.T) {
	const timeout = 200 * time.Millisecond
	got := make(chan string, 1)
	srv := startServer(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		conn, buf, err := http.NewResponseController(w).Hijack()
		if err != nil {
			t.Errorf("hijack: %v", err)
			return
		}
		defer conn.Close()
		_, _ = buf.WriteString("HTTP/1.1 101 Switching Protocols\r\n\r\n")
		_ = buf.Flush()
		line, err := buf.ReadString('\n')
		if err != nil {
			line = "error: " + err.Error()
		}
		got <- line
	}), timeout)

	conn, err := net.Dial("tcp", srv.Listener.Addr().String())
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	if _, err := io.WriteString(conn, "GET /api/exec/ws HTTP/1.1\r\nHost: kubebench\r\n\r\n"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(3 * timeout)
	if _, err := io.WriteString(conn, "ls\n"); err != nil {
		t.Fatal(err)
	}
	select {
	case line := <-got:
		if line != "ls\n" {
			t.Fatalf("read after the timeout: %q", line)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("the hijacked connection never read the message")
	}
}
