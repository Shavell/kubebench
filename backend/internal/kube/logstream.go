package kube

import (
	"context"
	"io"
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// LogSource is one container whose log a stream follows.
type LogSource struct {
	Pod       string
	Container string
}

// String is "pod/container", the prefix of the source's lines when a
// stream follows several pods.
func (s LogSource) String() string { return s.Pod + "/" + s.Container }

// LogFollowOptions selects the lines a followed log starts with.
type LogFollowOptions struct {
	// TailLines is the number of lines from the end to start with; nil
	// starts at the beginning of the log.
	TailLines *int64
	// Previous follows the previous (terminated) instance of the
	// container; such a stream ends after its last line.
	Previous bool
	// Timestamps prefixes every line with its RFC 3339 timestamp.
	Timestamps bool
	// SinceSeconds limits the lines to that many seconds back; 0 = no limit.
	SinceSeconds int64
}

// PodLogSource is the source of one pod's log: container, or the pod's
// default container when container is empty.
func (r *Runtime) PodLogSource(ctx context.Context, namespace, pod, container string) (LogSource, error) {
	name, err := r.PodContainer(ctx, namespace, pod, container)
	if err != nil {
		return LogSource{}, err
	}
	return LogSource{Pod: pod, Container: name}, nil
}

// SelectorLogSources lists the pods in namespace that selector matches, in
// name order, and returns the first limit of them as sources together with
// the number of matching pods. With a container name, pods without such a
// container do not match; without one, each pod's default container is
// followed.
func (r *Runtime) SelectorLogSources(ctx context.Context, namespace, selector, container string, limit int) ([]LogSource, int, error) {
	list, err := r.CS.CoreV1().Pods(namespace).List(ctx, metav1.ListOptions{LabelSelector: selector})
	if err != nil {
		return nil, 0, err
	}
	pods := list.Items
	slices.SortFunc(pods, func(a, b corev1.Pod) int { return strings.Compare(a.Name, b.Name) })
	var sources []LogSource
	matched := 0
	for i := range pods {
		name, err := containerOf(&pods[i], container)
		if err != nil {
			continue
		}
		matched++
		if len(sources) < limit {
			sources = append(sources, LogSource{Pod: pods[i].Name, Container: name})
		}
	}
	return sources, matched, nil
}

// FollowLog opens the live stream of src's log in namespace. The stream
// ends when the container terminates or ctx is cancelled.
func (r *Runtime) FollowLog(ctx context.Context, namespace string, src LogSource, o LogFollowOptions) (io.ReadCloser, error) {
	opts := &corev1.PodLogOptions{
		Container:  src.Container,
		Follow:     true,
		Previous:   o.Previous,
		Timestamps: o.Timestamps,
		TailLines:  o.TailLines,
	}
	if o.SinceSeconds > 0 {
		opts.SinceSeconds = &o.SinceSeconds
	}
	return r.CS.CoreV1().Pods(namespace).GetLogs(src.Pod, opts).Stream(ctx)
}
