package api

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"k8s.io/client-go/tools/remotecommand"
	utilexec "k8s.io/client-go/util/exec"
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
	Type    string `json:"type"` // "error"
	Message string `json:"message"`
}

// terminalSession pumps one session between the browser's WebSocket and
// the process in the container. Closing either side closes the other.
type terminalSession struct {
	conn     *websocket.Conn
	settings streamSettings

	writeMu sync.Mutex // one writer at a time (gorilla/websocket rule)
	stdinR  *io.PipeReader
	stdinW  *io.PipeWriter
	sizes   *sizeQueue
	input   chan struct{} // signals client input, for the idle timeout
	idle    atomic.Bool   // the session ended because of the idle timeout
}

func newTerminalSession(conn *websocket.Conn, settings streamSettings) *terminalSession {
	stdinR, stdinW := io.Pipe()
	return &terminalSession{
		conn:     conn,
		settings: settings,
		stdinR:   stdinR,
		stdinW:   stdinW,
		input:    make(chan struct{}, 1),
	}
}

// run streams the session until the process exits, the client leaves,
// the client stops answering pings or the idle timeout passes.
func (t *terminalSession) run(ctx context.Context, executor remotecommand.Executor) {
	defer t.conn.Close()
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	t.sizes = &sizeQueue{sizes: make(chan remotecommand.TerminalSize, 1), done: ctx.Done()}

	go t.readClient(cancel)
	go t.keepAlive(ctx, cancel)
	err := executor.StreamWithContext(ctx, remotecommand.StreamOptions{
		Stdin:             t.stdinR,
		Stdout:            t,
		Tty:               true,
		TerminalSizeQueue: t.sizes,
	})
	// Unblocks a client message still waiting for the process to read it.
	_ = t.stdinR.Close()
	t.finish(ctx, err)
}

// finish tells the client how the session ended and closes the WebSocket.
func (t *terminalSession) finish(ctx context.Context, err error) {
	var exitErr utilexec.ExitError
	switch {
	case t.idle.Load():
		t.sendJSON(terminalError{Type: "error", Message: fmt.Sprintf(
			"the session was closed after %s without input", t.settings.terminalIdle)})
	case ctx.Err() != nil:
		return // the client left or stopped answering: nobody to tell
	case err == nil:
		t.sendJSON(terminalExit{Type: "exit", Code: 0})
	case errors.As(err, &exitErr):
		t.sendJSON(terminalExit{Type: "exit", Code: exitErr.ExitStatus()})
	default:
		t.sendJSON(terminalError{Type: "error", Message: err.Error()})
	}
	_ = t.conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(t.settings.terminalWriteWait))
}

// readClient reads the client's messages until the connection fails or
// closes, then cancels the session.
func (t *terminalSession) readClient(cancel context.CancelFunc) {
	defer cancel()
	defer t.stdinW.Close()
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

// keepAlive pings the client and ends the session once the idle timeout
// passes without input. Output alone does not count as activity: a
// forgotten session that keeps printing is still forgotten.
func (t *terminalSession) keepAlive(ctx context.Context, cancel context.CancelFunc) {
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
				cancel()
				return
			}
		case <-t.input:
			idle.Reset(t.settings.terminalIdle)
		case <-idle.C:
			t.idle.Store(true)
			cancel()
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
