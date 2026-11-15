package api

import (
	"time"

	"k8s.io/client-go/tools/remotecommand"

	"kubebench/internal/kube"
)

// streamSettings holds the timings and the exec plumbing of the
// long-lived endpoints: log streams (SSE) and terminals (WebSocket).
// Tests shorten the timings and replace the executor with a fake.
type streamSettings struct {
	// logHeartbeat is the interval of the comment lines that keep idle
	// log streams open through proxies.
	logHeartbeat time.Duration
	// terminalPing is the WebSocket ping interval; a client that answers
	// none of two pings in a row is gone.
	terminalPing time.Duration
	// terminalIdle closes a terminal without input for that long.
	terminalIdle time.Duration
	// terminalWriteWait bounds one write to a terminal's WebSocket.
	terminalWriteWait time.Duration
	// terminalHangupGrace is how long a process may take to exit after
	// KubeBench hung up its terminal, before the stream is cut.
	terminalHangupGrace time.Duration
	// newExecutor connects a terminal to its container on the caller's cluster.
	newExecutor func(rt *kube.Runtime, t kube.ExecTarget) (remotecommand.Executor, error)
}

func defaultStreamSettings() streamSettings {
	return streamSettings{
		logHeartbeat:        15 * time.Second,
		terminalPing:        30 * time.Second,
		terminalIdle:        30 * time.Minute,
		terminalWriteWait:   10 * time.Second,
		terminalHangupGrace: 5 * time.Second,
		newExecutor:         (*kube.Runtime).TerminalExecutor,
	}
}
