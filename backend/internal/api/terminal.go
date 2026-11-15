package api

import (
	"context"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/gorilla/websocket"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// defaultShell starts bash when the image has it, else sh.
var defaultShell = []string{"/bin/sh", "-c", "command -v bash >/dev/null 2>&1 && exec bash || exec sh"}

// terminalShells are the commands a terminal may ask for instead of the
// default (the command parameter): interactive shells only, so the
// endpoint never runs an arbitrary command line.
var terminalShells = []string{
	"sh", "bash", "ash", "dash", "zsh",
	"/bin/sh", "/bin/bash", "/bin/ash", "/bin/dash", "/bin/zsh",
	"/usr/bin/bash", "/usr/bin/zsh", "/busybox/sh",
}

// handleTerminal serves GET /api/exec/ws: an interactive TTY session in a
// container over a WebSocket (design §7.1). The router has authorized the
// caller for exec and the origin check has passed; the audit row
// (exec.session) is written when the connection is upgraded. Problems
// found before the upgrade are ordinary JSON errors; once upgraded, the
// session reports them as error messages.
func (s *Server) handleTerminal(w http.ResponseWriter, r *http.Request, c *call) {
	q := r.URL.Query()
	target := kube.ExecTarget{
		Namespace: q.Get("ns"),
		Pod:       q.Get("pod"),
		Container: q.Get("container"),
		Command:   defaultShell,
	}
	if target.Namespace == "" || target.Pod == "" {
		writeErr(w, badRequest("ns and pod are required"))
		return
	}
	if shell := q.Get("command"); shell != "" {
		if !slices.Contains(terminalShells, shell) {
			writeErr(w, badRequest("command must be one of the shells %s", strings.Join(terminalShells, ", ")))
			return
		}
		target.Command = []string{shell}
	}
	if !websocket.IsWebSocketUpgrade(r) {
		writeErr(w, badRequest("this endpoint needs a WebSocket connection"))
		return
	}
	if target.Container == "" {
		// Resolved now so that the audit row names the container.
		name, err := c.rt.PodContainer(r.Context(), target.Namespace, target.Pod, "")
		if err != nil {
			plugin.WriteK8sError(w, err)
			return
		}
		target.Container = name
	}
	c.setAudit(joinTarget("pod", target.Namespace, target.Pod, target.Container), "")

	release, err := s.streamLimits.acquire(c.user.ID, kindTerminal)
	if err != nil {
		writeErr(w, err)
		return
	}
	defer release()
	// Shutdown hangs the session up and waits for it.
	finished, err := s.startTerminal()
	if err != nil {
		writeErr(w, err)
		return
	}
	defer finished()
	executor, err := s.streams.newExecutor(c.rt, target)
	if err != nil {
		writeErr(w, err)
		return
	}
	upgrader := websocket.Upgrader{
		HandshakeTimeout: 10 * time.Second,
		// Server.checkOrigin has applied the same rule; repeated here so
		// that the upgrade never depends on the middleware order.
		CheckOrigin: func(r *http.Request) bool { return s.originError(r) == nil },
		Error: func(w http.ResponseWriter, _ *http.Request, status int, reason error) {
			code := plugin.CodeBadRequest
			switch {
			case status == http.StatusForbidden:
				code = plugin.CodeForbidden
			case status >= http.StatusInternalServerError:
				code = plugin.CodeInternal
			}
			writeErr(w, errorf(status, code, "%v", reason))
		},
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return // Upgrade has answered with the error
	}
	// The session ends itself (see terminalSession.hangup). It must not end
	// with the request's context: net/http cancels that as soon as a read
	// of the hijacked connection fails, before the shell could be hung up.
	session := newTerminalSession(conn, s.streams)
	// A logout, a revoked assignment or a read-only cluster ends it too,
	// and so does the server's shutdown.
	session.check = func() error { return s.stillAllowed(c, accessExec) }
	session.stopping = s.life.stopping.Done()
	session.run(context.WithoutCancel(r.Context()), executor)
}
