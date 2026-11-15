package kube

import (
	"net/http"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubectl/pkg/scheme"
	"k8s.io/streaming/pkg/httpstream"
)

// ExecTarget is the container and command of an interactive exec session.
type ExecTarget struct {
	Namespace string
	Pod       string
	Container string
	Command   []string
}

// TerminalExecutor returns the executor of an interactive session in t:
// stdin, stdout and a TTY (which carries stderr too). Like kubectl exec it
// speaks the WebSocket protocol (Kubernetes 1.30+) and falls back to SPDY
// when the API server, or a proxy in front of it, refuses that upgrade.
func (r *Runtime) TerminalExecutor(t ExecTarget) (remotecommand.Executor, error) {
	u := r.CS.CoreV1().RESTClient().Post().
		Resource("pods").Namespace(t.Namespace).Name(t.Pod).SubResource("exec").
		VersionedParams(&corev1.PodExecOptions{
			Container: t.Container,
			Command:   t.Command,
			Stdin:     true,
			Stdout:    true,
			TTY:       true,
		}, scheme.ParameterCodec).URL()

	spdy, err := remotecommand.NewSPDYExecutor(r.Config, http.MethodPost, u)
	if err != nil {
		return nil, err
	}
	// RFC 6455 handshakes are GET requests.
	ws, err := remotecommand.NewWebSocketExecutor(r.Config, http.MethodGet, u.String())
	if err != nil {
		return nil, err
	}
	// The predicates of k8s.io/streaming, not the apimachinery look-alikes:
	// client-go's transports return that package's error types.
	return remotecommand.NewFallbackExecutor(ws, spdy, func(err error) bool {
		return httpstream.IsUpgradeFailure(err) || httpstream.IsHTTPSProxyError(err)
	})
}
