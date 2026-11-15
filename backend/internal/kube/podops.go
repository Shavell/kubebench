package kube

import (
	"context"
	"encoding/json"
	"strings"
	"sync"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/apimachinery/pkg/util/rand"
	"k8s.io/apimachinery/pkg/util/strategicpatch"
)

const (
	// MaxBulkDelete is the most pods one DeletePods call deletes.
	MaxBulkDelete = 200
	// bulkDeleteWorkers bounds the deletions in flight at once.
	bulkDeleteWorkers = 8
	// DefaultDebugImage is the image of a debug container when none is given.
	DefaultDebugImage = "busybox:1.37"
)

// PodKey names a pod (src/data/types.ts PodKey).
type PodKey struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
}

// PodDeleteResult is the outcome of deleting one pod of a bulk delete;
// Error is empty on success.
type PodDeleteResult struct {
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	Error     string `json:"error,omitempty"`
}

// DeletePod deletes a pod gracefully, or with force immediately: grace
// period 0 removes the object without waiting for the kubelet to stop the
// containers (kubectl delete --force --grace-period=0).
func (r *Runtime) DeletePod(ctx context.Context, namespace, name string, force bool) error {
	opts := metav1.DeleteOptions{}
	if force {
		var immediately int64
		opts.GracePeriodSeconds = &immediately
	}
	return r.CS.CoreV1().Pods(namespace).Delete(ctx, name, opts)
}

// DeletePods deletes up to MaxBulkDelete pods, a few at a time, and reports
// each pod's outcome in the order given.
func (r *Runtime) DeletePods(ctx context.Context, pods []PodKey, force bool) ([]PodDeleteResult, error) {
	if len(pods) == 0 || len(pods) > MaxBulkDelete {
		return nil, opError(OpInvalid, "a bulk delete takes 1 to %d pods, not %d", MaxBulkDelete, len(pods))
	}
	for _, p := range pods {
		if p.Namespace == "" || p.Name == "" {
			return nil, opError(OpInvalid, "every pod needs a namespace and a name")
		}
	}
	results := make([]PodDeleteResult, len(pods))
	slots := make(chan struct{}, bulkDeleteWorkers)
	var wg sync.WaitGroup
	for i, p := range pods {
		wg.Add(1)
		go func() {
			defer wg.Done()
			slots <- struct{}{}
			defer func() { <-slots }()
			results[i] = PodDeleteResult{Namespace: p.Namespace, Name: p.Name}
			if err := r.DeletePod(ctx, p.Namespace, p.Name, force); err != nil {
				results[i].Error = err.Error()
			}
		}()
	}
	wg.Wait()
	return results, nil
}

// DebugContainer names the ephemeral container AddDebugContainer added.
type DebugContainer struct {
	Namespace string `json:"namespace"`
	Pod       string `json:"pod"`
	Container string `json:"container"`
}

// AddDebugContainer adds an interactive ephemeral container
// ("debugger-xxxxx", stdin and TTY on) to a running pod through the
// pods/ephemeralcontainers subresource, like kubectl debug. With target it
// shares that container's process namespace. Ephemeral containers cannot be
// removed; this one stays until the pod is deleted.
func (r *Runtime) AddDebugContainer(ctx context.Context, namespace, podName, image, target string) (*DebugContainer, error) {
	if image == "" {
		image = DefaultDebugImage
	}
	if strings.ContainsAny(image, " \t\r\n") {
		return nil, opError(OpInvalid, "the image %q contains white space", image)
	}
	pods := r.CS.CoreV1().Pods(namespace)
	pod, err := pods.Get(ctx, podName, metav1.GetOptions{})
	if err != nil {
		return nil, err
	}
	if target != "" && !hasContainer(pod, target) {
		return nil, opError(OpInvalid, "pod %s has no container %q to target", podName, target)
	}
	debugger := corev1.EphemeralContainer{
		TargetContainerName: target,
		EphemeralContainerCommon: corev1.EphemeralContainerCommon{
			Name:                     debuggerName(pod),
			Image:                    image,
			Stdin:                    true,
			TTY:                      true,
			TerminationMessagePolicy: corev1.TerminationMessageReadFile,
		},
	}
	// Like kubectl debug: a strategic merge patch from the pod to the pod
	// with the container appended adds it last and keeps the ephemeral
	// containers already there, which cannot change.
	patch, err := appendPatch(pod, debugger)
	if err != nil {
		return nil, err
	}
	if _, err := pods.Patch(ctx, podName, types.StrategicMergePatchType, patch, metav1.PatchOptions{}, "ephemeralcontainers"); err != nil {
		return nil, err
	}
	return &DebugContainer{Namespace: namespace, Pod: podName, Container: debugger.Name}, nil
}

// appendPatch is the strategic merge patch that appends c to the pod's
// ephemeral containers.
func appendPatch(pod *corev1.Pod, c corev1.EphemeralContainer) ([]byte, error) {
	before, err := json.Marshal(pod)
	if err != nil {
		return nil, err
	}
	debugPod := pod.DeepCopy()
	debugPod.Spec.EphemeralContainers = append(debugPod.Spec.EphemeralContainers, c)
	after, err := json.Marshal(debugPod)
	if err != nil {
		return nil, err
	}
	return strategicpatch.CreateTwoWayMergePatch(before, after, pod)
}

func hasContainer(pod *corev1.Pod, name string) bool {
	for _, c := range pod.Spec.Containers {
		if c.Name == name {
			return true
		}
	}
	return false
}

// debuggerName picks a "debugger-xxxxx" name no container of the pod uses.
func debuggerName(pod *corev1.Pod) string {
	taken := map[string]bool{}
	for _, c := range pod.Spec.Containers {
		taken[c.Name] = true
	}
	for _, c := range pod.Spec.InitContainers {
		taken[c.Name] = true
	}
	for _, c := range pod.Spec.EphemeralContainers {
		taken[c.Name] = true
	}
	for {
		if name := "debugger-" + rand.String(5); !taken[name] {
			return name
		}
	}
}
