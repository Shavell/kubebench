package api

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"

	"kubebench/internal/reason"
)

// maxTerminalMessage bounds every WebSocket message of a terminal, in
// both directions; output is sent in parts of at most this size.
const maxTerminalMessage = 64 << 10

// The terminal protocol (design §7.1). The client sends JSON text frames
//
//	{"type":"input","data":"ls\r"}  {"type":"resize","cols":120,"rows":40}
//
// and the server answers with binary frames (terminal output) and JSON
// text frames that end the session:
//
//	{"type":"exit","code":0}  {"type":"error","message":"…"}
//
// An error frame explains KubeBench's own reasons with "reason" and
// "params" (§3.3) next to the English message.
type terminalInput struct {
	Type string `json:"type"`
	Data string `json:"data"`
	Cols int    `json:"cols"`
	Rows int    `json:"rows"`
}

type terminalExit struct {
	Type string `json:"type"` // "exit"
	Code int    `json:"code"`
}

type terminalError struct {
	Type    string        `json:"type"` // "error"
	Message string        `json:"message"`
	Reason  string        `json:"reason,omitempty"`
	Params  reason.Params `json:"params,omitempty"`
}

// errorFrame is the error frame explaining err.
func errorFrame(err error) terminalError {
	m := reason.MessageOf(err)
	return terminalError{Type: "error", Message: m.Text, Reason: m.Code, Params: m.Params}
}

// terminalSession pumps one session between the browser's WebSocket and
// the process in the container. Closing either side closes the other.
type terminalSession struct {
	conn     *websocket.Conn
	settings streamSettings
	// check, when set, runs at every ping: an error ends the session
	// (the caller lost access since it started).
	check func() error
	// stopping, when set, is closed when the server stops: the session
	// is then hung up like an idle one.
	stopping <-chan struct{}

	writeMu sync.Mutex // one writer at a time (gorilla/websocket rule)
	stdinR  *io.PipeReader
	stdinW  *io.PipeWriter
	sizes   *sizeQueue
	input   chan struct{} // signals client input, for the idle timeout

	cancel     context.CancelFunc
	done       chan struct{} // closed when the remote stream has ended
	hangupOnce sync.Once
	clientGone atomic.Bool           // the client left or stopped answering pings
	idle       atomic.Bool           // the idle timeout ended the session
	ended      atomic.Pointer[error] // why check or the shutdown ended the session
}

func newTerminalSession(conn *websocket.Conn, settings streamSettings) *terminalSession {
	stdinR, stdinW := io.Pipe()
	return &terminalSession{
		conn:     conn,
		settings: settings,
		stdinR:   stdinR,
		stdinW:   stdinW,
		input:    make(chan struct{}, 1),
		done:     make(chan struct{}),
	}
}

// run streams the session until the process exits, the client leaves,
// the client stops answering pings or the idle timeout passes.
func (t *terminalSession) run(ctx context.Context, executor remotecommand.Executor) {
	defer t.conn.Close()
	ctx, t.cancel = context.WithCancel(ctx)
	defer t.cancel()
	t.sizes = &sizeQueue{sizes: make(chan remotecommand.TerminalSize, 1), done: ctx.Done()}

	go t.readClient()
	go t.keepAlive(ctx)
	err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             t.stdinR,
		Stdout:            t,
		Tty:               true,
		TerminalSizeQueue: t.sizes,
	})
	close(t.done)
	// Unblocks a client message still waiting for the process to read it.
	_ = t.stdinR.Close()
	t.finish(err)
}

// The keys that end what runs in a terminal, the way a user would: Ctrl+C
// interrupts the foreground program, Ctrl+D ends the shell's input. The
// interrupt also discards input the terminal has not delivered yet, so
// the end of input follows after a pause.
const (
	interruptKey  = "\x03"
	endOfInputKey = "\x04"
	hangupPause   = 300 * time.Millisecond
)

// hangup ends the session from KubeBench's side (the client left, or the
// idle timeout passed). Container runtimes keep a TTY session's process
// running when its stream or input closes (kubectl exec leaves it behind
// too), so the shell is first told to finish through its terminal, as the
// Kubernetes dashboard does. A process still there after the grace period
// is cut off; it may then stay in the container, like jobs the shell left
// running in the background.
func (t *terminalSession) hangup() {
	t.hangupOnce.Do(func() {
		go func() {
			// Each write blocks until the process reads it, or the stream ends.
			_, _ = io.WriteString(t.stdinW, interruptKey)
			select {
			case <-time.After(hangupPause):
			case <-t.done:
			}
			_, _ = io.WriteString(t.stdinW, endOfInputKey)
			_ = t.stdinW.Close()
		}()
		go func() {
			select {
			case <-t.done:
			case <-time.After(t.settings.terminalHangupGrace):
				t.cancel()
			}
		}()
	})
}

// finish tells the client how the session ended and closes the WebSocket.
func (t *terminalSession) finish(err error) {
	var exitErr utilexec.ExitError
	switch {
	case t.ended.Load() != nil:
		t.sendJSON(errorFrame(*t.ended.Load()))
	case t.idle.Load():
		t.sendJSON(errorFrame(reason.New(reasonTerminalIdle, reason.Params{"duration": t.settings.terminalIdle.String()})))
	case t.clientGone.Load():
		return // nobody to tell
	case err == nil:
		t.sendJSON(terminalExit{Type: "exit", Code: 0})
	case errors.As(err, &exitErr):
		t.sendJSON(terminalExit{Type: "exit", Code: exitErr.ExitStatus()})
	default:
		t.sendJSON(errorFrame(err))
	}
	_ = t.conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(t.settings.terminalWriteWait))
}

// readClient reads the client's messages until the connection fails or
// closes, then hangs the session up.
func (t *terminalSession) readClient() {
	defer func() {
		t.clientGone.Store(true)
		t.hangup()
	}()
	pongWait := 2 * t.settings.terminalPing
	extend := func() error { return t.conn.SetReadDeadline(time.Now().Add(pongWait)) }
	t.conn.SetReadLimit(maxTerminalMessage)
	_ = extend()
	t.conn.SetPongHandler(func(string) error { return extend() })
	for {
		kind, data, err := t.conn.ReadMessage()
		if err != nil {
			return
		}
		_ = extend()
		if kind != websocket.TextMessage {
			continue // binary frames are not part of the client protocol
		}
		var msg terminalInput
		if err := json.Unmarshal(data, &msg); err != nil {
			_ = t.conn.WriteControl(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseUnsupportedData, "invalid message"),
				time.Now().Add(t.settings.terminalWriteWait))
			return
		}
		switch msg.Type {
		case "input":
			t.touch()
			if _, err := io.WriteString(t.stdinW, msg.Data); err != nil {
				return
			}
		case "resize":
			if msg.Cols > 0 && msg.Rows > 0 && msg.Cols <= 0xFFFF && msg.Rows <= 0xFFFF {
				t.touch()
				t.sizes.push(remotecommand.TerminalSize{Width: uint16(msg.Cols), Height: uint16(msg.Rows)})
			}
		}
		// Other types are ignored, so that newer clients keep working.
	}
}

// touch records client activity for the idle timeout.
func (t *terminalSession) touch() {
	select {
	case t.input <- struct{}{}:
	default:
	}
}

// keepAlive pings the client, checks the caller's access at every ping
// and ends the session once the idle timeout passes without input or the
// server stops. Output
// alone does not count as activity: a forgotten session that keeps
// printing is still forgotten.
func (t *terminalSession) keepAlive(ctx context.Context) {
	ping := time.NewTicker(t.settings.terminalPing)
	defer ping.Stop()
	idle := time.NewTimer(t.settings.terminalIdle)
	defer idle.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ping.C:
			if err := t.conn.WriteControl(websocket.PingMessage, nil,
				time.Now().Add(t.settings.terminalWriteWait)); err != nil {
				t.clientGone.Store(true)
				t.hangup()
				return
			}
			if t.check == nil {
				continue
			}
			if err := t.check(); err != nil {
				why := accessEnded(err)
				t.ended.Store(&why)
				t.hangup()
				return
			}
		case <-t.stopping:
			var why error = errShuttingDown
			t.ended.Store(&why)
			t.hangup()
			return
		case <-t.input:
			idle.Reset(t.settings.terminalIdle)
		case <-idle.C:
			t.idle.Store(true)
			t.hangup()
			return
		}
	}
}

// Write sends terminal output as binary messages; it is the session's
// stdout.
func (t *terminalSession) Write(p []byte) (int, error) {
	for sent := 0; sent < len(p); {
		n := min(len(p)-sent, maxTerminalMessage)
		if err := t.write(websocket.BinaryMessage, p[sent:sent+n]); err != nil {
			return sent, err
		}
		sent += n
	}
	return len(p), nil
}

// sendJSON sends one of the protocol's text messages (plain structs that
// always encode).
func (t *terminalSession) sendJSON(v any) {
	data, _ := json.Marshal(v)
	_ = t.write(websocket.TextMessage, data)
}

func (t *terminalSession) write(kind int, data []byte) error {
	t.writeMu.Lock()
	defer t.writeMu.Unlock()
	if err := t.conn.SetWriteDeadline(time.Now().Add(t.settings.terminalWriteWait)); err != nil {
		return err
	}
	return t.conn.WriteMessage(kind, data)
}

// sizeQueue hands the terminal sizes the client reports to remotecommand.
// A size not picked up yet is replaced by a newer one.
type sizeQueue struct {
	sizes chan remotecommand.TerminalSize
	done  <-chan struct{}
}

// Next blocks until the next size, or returns nil once the session ended.
func (q *sizeQueue) Next() *remotecommand.TerminalSize {
	select {
	case size := <-q.sizes:
		return &size
	case <-q.done:
		return nil
	}
}

func (q *sizeQueue) push(size remotecommand.TerminalSize) {
	for {
		select {
		case q.sizes <- size:
			return
		default:
		}
		select {
		case <-q.sizes: // drop the stale size
		default:
		}
	}
}
