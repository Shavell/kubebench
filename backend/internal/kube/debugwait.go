package kube

import (
	"context"
	"errors"
	"slices"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	utilwait "k8s.io/apimachinery/pkg/util/wait"

	"kubebench/internal/reason"
)

// debugPollInterval is how often WaitForDebugContainer reads the pod.
const debugPollInterval = 500 * time.Millisecond

// debugStartFailures are the waiting reasons after which an ephemeral
// container will not start without a change (another image, a fixed
// pod): waiting longer only delays the answer.
var debugStartFailures = []string{
	"ImagePullBackOff", "InvalidImageName", "ErrImageNeverPull",
	"CreateContainerConfigError", "CreateContainerError", "RunContainerError",
}

// WaitForDebugContainer waits until the ephemeral container a debug
// session added runs, so that a terminal can attach to it: the kubelet
// starts it (pulling the image first) some time after AddDebugContainer
// returned. It fails with an OpConflict when the container cannot start,
// has ended already or is still waiting after timeout, naming the reason
// the kubelet reports.
func (r *Runtime) WaitForDebugContainer(ctx context.Context, namespace, podName, container string, timeout time.Duration) error {
	pods := r.CS.CoreV1().Pods(namespace)
	var state corev1.ContainerState
	err := utilwait.PollUntilContextTimeout(ctx, debugPollInterval, timeout, true, func(ctx context.Context) (bool, error) {
		pod, err := pods.Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			return false, err
		}
		state = ephemeralState(pod, container)
		switch {
		case state.Running != nil:
			return true, nil
		case state.Terminated != nil:
			t := state.Terminated
			return false, opError(OpConflict, reasonDebugEnded,
				reason.Params{"container": container, "exitCode": t.ExitCode, "detail": t.Reason})
		case state.Waiting != nil && slices.Contains(debugStartFailures, state.Waiting.Reason):
			return false, opError(OpConflict, reasonDebugCannotStart,
				reason.Params{"container": container, "detail": waitingText(state.Waiting)})
		}
		return false, nil
	})
	if err == nil || ctx.Err() != nil {
		return err
	}
	var op *OpError
	if errors.As(err, &op) || !utilwait.Interrupted(err) {
		return err
	}
	params := reason.Params{"container": container, "timeout": timeout.String()}
	if state.Waiting == nil {
		return opError(OpConflict, reasonDebugNoStatus, params)
	}
	params["detail"] = waitingText(state.Waiting)
	return opError(OpConflict, reasonDebugNotRunning, params)
}

// ephemeralState is the reported state of the pod's ephemeral container
// name; zero while the kubelet has not reported it yet.
func ephemeralState(pod *corev1.Pod, name string) corev1.ContainerState {
	for _, s := range pod.Status.EphemeralContainerStatuses {
		if s.Name == name {
			return s.State
		}
	}
	return corev1.ContainerState{}
}

func waitingText(w *corev1.ContainerStateWaiting) string {
	if w.Message == "" {
		return w.Reason
	}
	return w.Reason + ": " + w.Message
}
