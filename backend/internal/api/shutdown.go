package api

import (
	"context"
	"errors"
	"net/http"
	"sync"

	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// errShuttingDown is why the server ends a stream or terminal itself when
// it stops; clients see its reason.
var errShuttingDown = reason.New(reasonShuttingDown, nil)

// lifecycle ends the long-lived connections when the server stops.
// http.Server.Shutdown waits for SSE handlers, which never return on their
// own, and ignores hijacked WebSocket connections, whose shells would be
// cut off without a hangup and keep running in their containers.
type lifecycle struct {
	stopping context.Context
	stop     context.CancelFunc

	mu        sync.Mutex
	closed    bool // no terminal may start any more
	terminals sync.WaitGroup
}

func newLifecycle() *lifecycle {
	ctx, cancel := context.WithCancel(context.Background())
	return &lifecycle{stopping: ctx, stop: cancel}
}

// Shutdown ends every log and watch stream (with an end or error event
// naming the reason) and hangs up every terminal, then waits until the
// terminals are closed or ctx ends. Call it before http.Server.Shutdown,
// which then finds the streams' connections idle. New terminals are
// refused from now on; new streams end at once.
func (s *Server) Shutdown(ctx context.Context) error {
	s.life.mu.Lock()
	s.life.closed = true
	s.life.mu.Unlock()
	s.life.stop()

	done := make(chan struct{})
	go func() {
		s.life.terminals.Wait()
		close(done)
	}()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// streamContext is parent, also cancelled with the cause errShuttingDown
// when the server stops.
func (s *Server) streamContext(parent context.Context) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithCancelCause(parent)
	stopAfter := context.AfterFunc(s.life.stopping, func() { cancel(errShuttingDown) })
	return ctx, func() {
		stopAfter()
		cancel(context.Canceled)
	}
}

// shuttingDown reports whether ctx (from streamContext) ended because the
// server stops.
func shuttingDown(ctx context.Context) bool {
	return errors.Is(context.Cause(ctx), errShuttingDown)
}

// startTerminal counts a terminal for Shutdown to wait for; call done
// when it is closed. It fails once the server stops.
func (s *Server) startTerminal() (done func(), err error) {
	s.life.mu.Lock()
	defer s.life.mu.Unlock()
	if s.life.closed {
		return nil, fail(http.StatusServiceUnavailable, plugin.CodeInternal, reasonShuttingDown, nil)
	}
	s.life.terminals.Add(1)
	return s.life.terminals.Done, nil
}
