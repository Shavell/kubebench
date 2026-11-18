package api

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/gorilla/websocket"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/store"
)

// echoShell stands in for the process in the container: it echoes every
// input line ("echo:<line>"), prints every resize ("[resize 80x24]"),
// exits on "exit <code>" and when its input ends, unless it was told
// "ignore-eof" before. With fail set it fails at once instead.
type echoShell struct {
	target  kube.ExecTarget
	fail    error
	stopped chan struct{} // closed when the session returned
	ctxErr  atomic.Value  // the context error the session ended with

	mu    sync.Mutex
	input strings.Builder // everything the shell read
}

// received is the input the shell has read so far.
func (f *echoShell) received() string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.input.String()
}

func (f *echoShell) Stream(o remotecommand.StreamOptions) error {
	return f.StreamWithContext(context.Background(), o)
}

func (f *echoShell) StreamWithContext(ctx context.Context, o remotecommand.StreamOptions) error {
	defer close(f.stopped)
	if f.fail != nil {
		return f.fail
	}
	if !o.Tty || o.Stdin == nil || o.Stdout == nil || o.TerminalSizeQueue == nil {
		return errors.New("a terminal needs a TTY, stdin, stdout and a size queue")
	}
	go func() {
		for size := o.TerminalSizeQueue.Next(); size != nil; size = o.TerminalSizeQueue.Next() {
			fmt.Fprintf(o.Stdout, "[resize %dx%d]", size.Width, size.Height)
		}
	}()
	lines := make(chan string)
	go func() {
		defer close(lines)
		r := bufio.NewReader(o.Stdin)
		for {
			line, err := r.ReadString('\r')
			f.mu.Lock()
			f.input.WriteString(line)
			f.mu.Unlock()
			if err != nil {
				return
			}
			select {
			case lines <- strings.TrimSuffix(line, "\r"):
			case <-ctx.Done():
				return
			}
		}
	}()
	ignoreEOF := false
	for {
		select {
		case <-ctx.Done():
			f.ctxErr.Store(ctx.Err())
			return ctx.Err()
		case line, ok := <-lines:
			if !ok {
				if ignoreEOF {
					lines = nil // wait for the context alone
					continue
				}
				return nil // the input ended
			}
			if code, found := strings.CutPrefix(line, "exit "); found {
				n, _ := strconv.Atoi(code)
				if n == 0 {
					return nil
				}
				return utilexec.CodeExitError{Err: fmt.Errorf("command terminated with exit code %d", n), Code: n}
			}
			ignoreEOF = ignoreEOF || line == "ignore-eof"
			fmt.Fprintf(o.Stdout, "echo:%s\r\n", line)
		}
	}
}

// shells records the sessions the terminal endpoint started.
type shells struct {
	mu       sync.Mutex
	sessions []*echoShell
	fail     error
}

func (s *shells) executor(_ *kube.Runtime, t kube.ExecTarget) (remotecommand.Executor, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	shell := &echoShell{target: t, fail: s.fail, stopped: make(chan struct{})}
	s.sessions = append(s.sessions, shell)
	return shell, nil
}

// failWith makes the sessions started from now on fail at once with err.
func (s *shells) failWith(err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.fail = err
}

func (s *shells) started() []*echoShell {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]*echoShell(nil), s.sessions...)
}

// terminalEnv is a test environment served over TCP, as WebSockets need,
// whose terminals run echoShells. tune adjusts the stream settings before
// the server starts.
func terminalEnv(t *testing.T, tune ...func(*streamSettings)) (*testEnv, *httptest.Server, *shells) {
	t.Helper()
	e := newTestEnv(t, nil)
	sh := &shells{}
	e.srv.streams.newExecutor = sh.executor
	e.srv.streams.terminalWriteWait = time.Second
	for _, f := range tune {
		f(&e.srv.streams)
	}
	srv := httptest.NewServer(e.handler)
	t.Cleanup(srv.Close)
	return e, srv, sh
}

// dial opens /api/exec/ws?query as user from the server's own origin;
// header entries are added or override (an empty value removes one).
func dial(t *testing.T, e *testEnv, srv *httptest.Server, user, query string, header http.Header) (*websocket.Conn, *http.Response, error) {
	t.Helper()
	h := http.Header{"Origin": {srv.URL}}
	if user != "" {
		h.Set("Cookie", sessionCookie+"="+e.tokens[user])
	}
	for k, v := range header {
		if len(v) == 1 && v[0] == "" {
			h.Del(k)
			continue
		}
		h[k] = v
	}
	url := "ws" + strings.TrimPrefix(srv.URL, "http") + "/api/exec/ws?" + query
	conn, resp, err := websocket.DefaultDialer.Dial(url, h)
	if conn != nil {
		t.Cleanup(func() { conn.Close() })
	}
	return conn, resp, err
}

// refused checks that a handshake was answered with status and the §3.3 code.
func refused(t *testing.T, resp *http.Response, err error, status int, code string) {
	t.Helper()
	if !errors.Is(err, websocket.ErrBadHandshake) || resp == nil {
		t.Fatalf("handshake error = %v, want a refused handshake", err)
	}
	var body plugin.ErrorBody
	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != status || json.Unmarshal(raw, &body) != nil || body.Code != code {
		t.Fatalf("refused with %d %s, want %d %q", resp.StatusCode, raw, status, code)
	}
}

func send(t *testing.T, conn *websocket.Conn, msg string) {
	t.Helper()
	if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
		t.Fatalf("send %s: %v", msg, err)
	}
}

// readOutput collects terminal output until it contains want.
func readOutput(t *testing.T, conn *websocket.Conn, want string) {
	t.Helper()
	var out strings.Builder
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for !strings.Contains(out.String(), want) {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("waiting for %q (got %q): %v", want, out.String(), err)
		}
		if kind != websocket.BinaryMessage {
			t.Fatalf("unexpected text message %s while waiting for %q", data, want)
		}
		out.Write(data)
	}
}

// readEnd reads past any output to the session's final JSON message and
// the close frame that follows it.
func readEnd(t *testing.T, conn *websocket.Conn) map[string]any {
	t.Helper()
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		kind, data, err := conn.ReadMessage()
		if err != nil {
			t.Fatalf("no final message: %v", err)
		}
		if kind != websocket.TextMessage {
			continue
		}
		var msg map[string]any
		if err := json.Unmarshal(data, &msg); err != nil {
			t.Fatalf("final message %s: %v", data, err)
		}
		_, _, err = conn.ReadMessage()
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) || closeErr.Code != websocket.CloseNormalClosure {
			t.Fatalf("after %s: %v, want a normal close", data, err)
		}
		return msg
	}
}

func TestTerminalSession(t *testing.T) {
	e, srv, sh := terminalEnv(t)
	conn, resp, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
	if err != nil {
		t.Fatalf("dial: %v (%v)", err, resp)
	}
	send(t, conn, `{"type":"resize","cols":120,"rows":40}`)
	readOutput(t, conn, "[resize 120x40]")
	send(t, conn, `{"type":"input","data":"ls -l\r"}`)
	readOutput(t, conn, "echo:ls -l")
	send(t, conn, `{"type":"resize","cols":80,"rows":24}`)
	readOutput(t, conn, "[resize 80x24]")
	send(t, conn, `{"type":"input","data":"exit 3\r"}`)
	if msg := readEnd(t, conn); msg["type"] != "exit" || msg["code"] != float64(3) {
		t.Fatalf("final message = %v, want exit code 3", msg)
	}

	started := sh.started()
	if len(started) != 1 {
		t.Fatalf("sessions = %d", len(started))
	}
	got := started[0].target
	if got.Namespace != "default" || got.Pod != "web-1" || got.Container != "app" ||
		strings.Join(got.Command, " ") != strings.Join(defaultShell, " ") {
		t.Fatalf("target = %+v", got)
	}
	rows := e.audit()
	if len(rows) != 1 {
		t.Fatalf("audit rows = %+v, want one per session", rows)
	}
	want := store.AuditEntry{Username: "alice", Cluster: "test", Action: "exec.session", Target: "pod/default/web-1/app", Status: http.StatusSwitchingProtocols}
	if row := rows[0]; row.Username != want.Username || row.Cluster != want.Cluster || row.Action != want.Action ||
		row.Target != want.Target || row.Status != want.Status {
		t.Fatalf("audit row = %+v, want %+v", row, want)
	}
}

func TestTerminalExitMessages(t *testing.T) {
	e, srv, sh := terminalEnv(t)
	conn, _, err := dial(t, e, srv, "admin", "ns=default&pod=web-1&container=app&command=bash", nil)
	if err != nil {
		t.Fatal(err)
	}
	send(t, conn, `{"type":"input","data":"exit 0\r"}`)
	if msg := readEnd(t, conn); msg["type"] != "exit" || msg["code"] != float64(0) {
		t.Fatalf("final message = %v, want exit code 0", msg)
	}
	if cmd := sh.started()[0].target.Command; len(cmd) != 1 || cmd[0] != "bash" {
		t.Fatalf("command = %v, want the requested shell", cmd)
	}

	// A session that cannot start reports why.
	sh.failWith(errors.New(`container "app" in pod "web-1" is not running`))
	conn, _, err = dial(t, e, srv, "admin", "ns=default&pod=web-1&container=app", nil)
	if err != nil {
		t.Fatal(err)
	}
	if msg := readEnd(t, conn); msg["type"] != "error" || !strings.Contains(fmt.Sprint(msg["message"]), "is not running") {
		t.Fatalf("final message = %v, want the error", msg)
	}
}

func TestTerminalRefusedBeforeUpgrade(t *testing.T) {
	e, srv, sh := terminalEnv(t)
	query := "ns=default&pod=web-1&container=app"

	resp, err := http.Get(srv.URL + "/api/exec/ws?" + query) // no session
	if err != nil || resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("anonymous: %v %v", resp, err)
	}
	_, resp, err = dial(t, e, srv, "bob", query, nil) // viewer
	refused(t, resp, err, http.StatusForbidden, plugin.CodeForbiddenRole)
	_, resp, err = dial(t, e, srv, "alice", query, http.Header{"Origin": {"http://evil.example"}})
	refused(t, resp, err, http.StatusForbidden, plugin.CodeForbidden)
	// Browsers always send Origin; one that does not is not trusted.
	_, resp, err = dial(t, e, srv, "alice", query, http.Header{"Origin": {""}, "Sec-Fetch-Mode": {"websocket"}})
	refused(t, resp, err, http.StatusForbidden, plugin.CodeForbidden)
	_, resp, err = dial(t, e, srv, "alice", query+"&command=python3", nil)
	refused(t, resp, err, http.StatusBadRequest, plugin.CodeBadRequest)
	_, resp, err = dial(t, e, srv, "alice", "ns=default", nil)
	refused(t, resp, err, http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodGet, "/api/exec/ws?"+query, "alice", nil), http.StatusBadRequest, plugin.CodeBadRequest)

	read := true
	if err := e.store.UpdateCluster(e.cluster, store.ClusterUpdate{ReadOnly: &read}); err != nil {
		t.Fatal(err)
	}
	_, resp, err = dial(t, e, srv, "admin", query, nil)
	refused(t, resp, err, http.StatusForbidden, plugin.CodeClusterReadOnly)

	if n := len(sh.started()); n != 0 {
		t.Fatalf("refused requests started %d sessions", n)
	}
	if actions := e.cs.Actions(); len(actions) != 0 {
		t.Fatalf("refused requests reached the cluster: %v", actions)
	}
	// Refused attempts are audited like any exec request.
	for _, row := range e.audit() {
		if row.Action != "exec.session" || row.Status < 400 {
			t.Fatalf("audit row = %+v", row)
		}
	}
}

func TestTerminalWithoutOriginFromScripts(t *testing.T) {
	e, srv, sh := terminalEnv(t)
	conn, resp, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", http.Header{"Origin": {""}})
	if err != nil {
		t.Fatalf("a non-browser client without Origin: %v (%v)", err, resp)
	}
	send(t, conn, `{"type":"input","data":"exit 0\r"}`)
	readEnd(t, conn)
	if len(sh.started()) != 1 {
		t.Fatal("no session")
	}
}

func TestTerminalDefaultContainer(t *testing.T) {
	e, srv, sh := terminalEnv(t)
	_, err := e.cs.CoreV1().Pods("default").Create(context.Background(), &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default",
			Annotations: map[string]string{"kubectl.kubernetes.io/default-container": "server"}},
		Spec: corev1.PodSpec{Containers: []corev1.Container{{Name: "proxy"}, {Name: "server"}}},
	}, metav1.CreateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	conn, _, err := dial(t, e, srv, "alice", "ns=default&pod=api", nil)
	if err != nil {
		t.Fatal(err)
	}
	send(t, conn, `{"type":"input","data":"exit 0\r"}`)
	readEnd(t, conn)
	if got := sh.started()[0].target.Container; got != "server" {
		t.Fatalf("container = %q, want the annotated default", got)
	}
	if row := e.audit()[0]; row.Target != "pod/default/api/server" {
		t.Fatalf("audit target = %q", row.Target)
	}

	_, resp, err := dial(t, e, srv, "alice", "ns=default&pod=gone", nil)
	refused(t, resp, err, http.StatusNotFound, plugin.CodeKubernetes)
}

func TestTerminalClientLeaving(t *testing.T) {
	e, srv, sh := terminalEnv(t)
	conn, _, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
	if err != nil {
		t.Fatal(err)
	}
	send(t, conn, `{"type":"input","data":"hi\r"}`)
	readOutput(t, conn, "echo:hi")
	conn.Close()
	shell := sh.started()[0]
	select {
	case <-shell.stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("the session outlived its client")
	}
	// The shell was told to finish through its terminal, and did.
	if got := shell.received(); got != "hi\r"+interruptKey+endOfInputKey {
		t.Fatalf("shell input = %q, want the line, then Ctrl+C and Ctrl+D (ctx: %v)", got, shell.ctxErr.Load())
	}
	if err, _ := shell.ctxErr.Load().(error); err != nil {
		t.Fatalf("the shell ended by itself, yet its context ended with %v", err)
	}
}

func TestTerminalHangupCutsOffAStubbornProcess(t *testing.T) {
	e, srv, sh := terminalEnv(t, func(s *streamSettings) { s.terminalHangupGrace = 200 * time.Millisecond })
	conn, _, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
	if err != nil {
		t.Fatal(err)
	}
	send(t, conn, `{"type":"input","data":"ignore-eof\r"}`)
	readOutput(t, conn, "echo:ignore-eof")
	conn.Close()
	shell := sh.started()[0]
	select {
	case <-shell.stopped:
	case <-time.After(5 * time.Second):
		t.Fatal("a process ignoring the hangup kept the stream open")
	}
	if err, _ := shell.ctxErr.Load().(error); !errors.Is(err, context.Canceled) {
		t.Fatalf("session ended with %v, want the stream cut off", err)
	}
}

func TestTerminalIdleTimeoutAndPing(t *testing.T) {
	e, srv, _ := terminalEnv(t, func(s *streamSettings) {
		s.terminalPing = 100 * time.Millisecond
		s.terminalIdle = 500 * time.Millisecond
	})
	conn, _, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
	if err != nil {
		t.Fatal(err)
	}
	var pings atomic.Int32
	conn.SetPingHandler(func(data string) error {
		pings.Add(1)
		return conn.WriteControl(websocket.PongMessage, []byte(data), time.Now().Add(time.Second))
	})
	started := time.Now()
	msg := readEnd(t, conn)
	if msg["type"] != "error" || !strings.Contains(fmt.Sprint(msg["message"]), "without input") || msg["reason"] != "terminal.idle" {
		t.Fatalf("final message = %v, want the idle timeout", msg)
	}
	if elapsed := time.Since(started); elapsed < 450*time.Millisecond {
		t.Fatalf("closed after %s, before the idle timeout", elapsed)
	}
	if pings.Load() < 3 {
		t.Fatalf("pings = %d, want one every interval", pings.Load())
	}
}

func TestTerminalInputKeepsSessionAlive(t *testing.T) {
	e, srv, sh := terminalEnv(t, func(s *streamSettings) { s.terminalIdle = 300 * time.Millisecond })
	conn, _, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 6 {
		send(t, conn, fmt.Sprintf(`{"type":"input","data":"%d\r"}`, i))
		readOutput(t, conn, fmt.Sprintf("echo:%d", i))
		time.Sleep(100 * time.Millisecond)
	}
	select {
	case <-sh.started()[0].stopped:
		t.Fatal("a session with input timed out")
	default:
	}
}

func TestTerminalMessageLimit(t *testing.T) {
	e, srv, _ := terminalEnv(t)
	conn, _, err := dial(t, e, srv, "alice", "ns=default&pod=web-1&container=app", nil)
	if err != nil {
		t.Fatal(err)
	}
	big := `{"type":"input","data":"` + strings.Repeat("x", maxTerminalMessage) + `"}`
	send(t, conn, big)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	for {
		_, _, err := conn.ReadMessage()
		if err == nil {
			continue
		}
		var closeErr *websocket.CloseError
		if !errors.As(err, &closeErr) || closeErr.Code != websocket.CloseMessageTooBig {
			t.Fatalf("after an oversized message: %v, want close 1009", err)
		}
		return
	}
}

func TestTerminalOutputIsSplit(t *testing.T) {
	var frames [][]byte
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := (&websocket.Upgrader{}).Upgrade(w, r, nil)
		if err != nil {
			return
		}
		session := newTerminalSession(conn, defaultStreamSettings())
		_, _ = session.Write(make([]byte, 2*maxTerminalMessage+10))
		conn.Close()
	}))
	defer server.Close()
	conn, _, err := websocket.DefaultDialer.Dial("ws"+strings.TrimPrefix(server.URL, "http"), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Close()
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			break
		}
		frames = append(frames, data)
	}
	if len(frames) != 3 || len(frames[0]) != maxTerminalMessage || len(frames[2]) != 10 {
		sizes := make([]int, len(frames))
		for i, f := range frames {
			sizes[i] = len(f)
		}
		t.Fatalf("frame sizes = %v", sizes)
	}
}

func TestSizeQueueKeepsLatest(t *testing.T) {
	done := make(chan struct{})
	q := &sizeQueue{sizes: make(chan remotecommand.TerminalSize, 1), done: done}
	q.push(remotecommand.TerminalSize{Width: 80, Height: 24})
	q.push(remotecommand.TerminalSize{Width: 100, Height: 30})
	if got := q.Next(); got == nil || got.Width != 100 || got.Height != 30 {
		t.Fatalf("Next = %+v, want the latest size", got)
	}
	close(done)
	if got := q.Next(); got != nil {
		t.Fatalf("Next after the session ended = %+v", got)
	}
}
