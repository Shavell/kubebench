package kube

import (
	"context"
	"io"
	"slices"
	"strings"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/kubectl/pkg/drain"
)

// defaultEvictRetryDelay is kubectl drain's pause before it asks again to
// evict a pod whose PodDisruptionBudget refused.
const defaultEvictRetryDelay = 5 * time.Second

// DrainOptions are the kubectl drain flags DrainNode supports.
type DrainOptions struct {
	// IgnoreDaemonSets leaves DaemonSet pods on the node; without it their
	// presence stops the drain.
	IgnoreDaemonSets bool
	// DeleteEmptyDirData also evicts pods with emptyDir volumes, whose data
	// is lost.
	DeleteEmptyDirData bool
	// Force also evicts pods that no controller manages; nothing recreates
	// them.
	Force bool
	// GracePeriodSeconds each pod gets to terminate; negative means the
	// pod's own terminationGracePeriodSeconds.
	GracePeriodSeconds int
	// Timeout bounds the evictions; zero waits forever.
	Timeout time.Duration
	// RetryDelay is the pause before an eviction a PodDisruptionBudget
	// refused is tried again; zero means 5s.
	RetryDelay time.Duration
}

// DrainResult reports what a drain did (src/data/types.ts DrainResult).
// Pods are named "<namespace>/<name>".
type DrainResult struct {
	Node string `json:"node"`
	// Cordoned: the node is unschedulable, the first step of every drain.
	Cordoned bool `json:"cordoned"`
	// Completed: every pod that had to leave the node is gone.
	Completed bool     `json:"completed"`
	Evicted   []string `json:"evicted"`
	// Failed lists the pods that should have left but are still there.
	Failed []PodFailure `json:"failed"`
	// BlockedByPDB lists the failed pods whose last eviction a
	// PodDisruptionBudget refused.
	BlockedByPDB []string `json:"blockedByPDB"`
	// Skipped lists the pods a drain leaves alone: DaemonSet pods (with
	// IgnoreDaemonSets) and static pods, whose mirror pods cannot be evicted.
	Skipped []SkippedPod `json:"skipped"`
	// Errors are problems that stopped the drain before any eviction, e.g.
	// pods without a controller when Force is off.
	Errors []string `json:"errors"`
}

// PodFailure is a pod a drain could not remove, with the reason: the
// Kubernetes API's refusal, or KubeBench's own explanation with its reason
// code (§3.3).
type PodFailure struct {
	Pod    string `json:"pod"`
	Error  string `json:"error"`
	Reason string `json:"reason,omitempty"`
}

// SkippedPod is a pod a drain left on the node on purpose.
type SkippedPod struct {
	Pod string `json:"pod"`
	// Reason is "daemonSet" or "mirror".
	Reason string `json:"reason"`
}

// DrainNode cordons the node and evicts its pods through the eviction API,
// like kubectl drain (k8s.io/kubectl/pkg/drain): PodDisruptionBudgets are
// honoured, and an eviction a budget refuses is retried until
// o.Timeout. The result lists every pod's outcome; it is returned even when
// pods remain, so only a failure to reach the node or list its pods is an
// error.
func (r *Runtime) DrainNode(ctx context.Context, name string, o DrainOptions) (*DrainResult, error) {
	// Fail on an unknown node rather than reporting "nothing to drain".
	if _, err := r.CS.CoreV1().Nodes().Get(ctx, name, metav1.GetOptions{}); err != nil {
		return nil, err
	}
	if err := r.CordonNode(ctx, name, true); err != nil {
		return nil, err
	}
	res := &DrainResult{
		Node: name, Cordoned: true, Evicted: []string{}, Failed: []PodFailure{},
		BlockedByPDB: []string{}, Skipped: []SkippedPod{}, Errors: []string{},
	}

	rec := &evictionRecorder{
		started: map[string]bool{}, accepted: map[string]bool{},
		gone: map[string]bool{}, lastErr: map[string]error{},
	}
	retryDelay := o.RetryDelay
	if retryDelay <= 0 {
		retryDelay = defaultEvictRetryDelay
	}
	helper := &drain.Helper{
		Ctx:                             ctx,
		Client:                          evictionObserver{Interface: r.CS, observe: rec.evicted},
		Force:                           o.Force,
		GracePeriodSeconds:              o.GracePeriodSeconds,
		IgnoreAllDaemonSets:             o.IgnoreDaemonSets,
		DeleteEmptyDirData:              o.DeleteEmptyDirData,
		Timeout:                         o.Timeout,
		EvictErrorRetryDelay:            retryDelay,
		Out:                             io.Discard,
		ErrOut:                          io.Discard,
		OnPodDeletionOrEvictionStarted:  rec.attempt,
		OnPodDeletionOrEvictionFinished: rec.finished,
	}

	list, errs := helper.GetPodsForDeletion(name)
	if list == nil && len(errs) > 0 {
		return nil, errs[0] // the pods could not be listed
	}
	if len(errs) > 0 {
		// Like kubectl drain: refuse up front instead of half draining.
		for _, err := range errs {
			res.Errors = append(res.Errors, err.Error())
		}
		slices.Sort(res.Errors)
		return res, nil
	}
	pods := list.Pods()
	skipped, err := r.skippedPods(ctx, name, pods)
	if err != nil {
		return nil, err
	}
	res.Skipped = skipped
	drainErr := helper.DeleteOrEvictPods(pods)
	rec.report(res, pods, drainErr)
	return res, nil
}

// skippedPods lists the pods on the node that a drain leaves alone. With
// the filters' errors ruled out, only DaemonSet pods and mirror pods remain
// outside the pods to evict.
func (r *Runtime) skippedPods(ctx context.Context, node string, evicting []corev1.Pod) ([]SkippedPod, error) {
	selector := fields.OneTermEqualSelector("spec.nodeName", node).String()
	all, err := r.CS.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{FieldSelector: selector})
	if err != nil {
		return nil, err
	}
	leaving := map[string]bool{}
	for i := range evicting {
		leaving[podKey(&evicting[i])] = true
	}
	skipped := []SkippedPod{}
	for i := range all.Items {
		pod := &all.Items[i]
		if leaving[podKey(pod)] {
			continue
		}
		reason := "daemonSet"
		if _, mirror := pod.Annotations[corev1.MirrorPodAnnotationKey]; mirror {
			reason = "mirror"
		}
		skipped = append(skipped, SkippedPod{Pod: podKey(pod), Reason: reason})
	}
	slices.SortFunc(skipped, func(a, b SkippedPod) int { return strings.Compare(a.Pod, b.Pod) })
	return skipped, nil
}

func podKey(p *corev1.Pod) string { return p.Namespace + "/" + p.Name }

// evictionRecorder collects the outcome of each pod of a drain. The drain
// helper evicts pods concurrently, so every method locks.
type evictionRecorder struct {
	mu       sync.Mutex
	started  map[string]bool  // an eviction was attempted
	accepted map[string]bool  // the API server accepted an eviction
	gone     map[string]bool  // the pod disappeared after its eviction
	lastErr  map[string]error // the latest refusal or lookup failure
}

func (e *evictionRecorder) attempt(pod *corev1.Pod, _ bool) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.started[podKey(pod)] = true
}

// evicted records the answer to one eviction request.
func (e *evictionRecorder) evicted(namespace, name string, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	key := namespace + "/" + name
	switch {
	case err == nil:
		e.accepted[key] = true
		delete(e.lastErr, key)
	case apierrors.IsNotFound(err):
		e.gone[key] = true // already gone; the helper does not report it as finished
	default:
		e.lastErr[key] = err
	}
}

// finished is the helper's OnPodDeletionOrEvictionFinished: err is nil
// once the evicted pod is gone.
func (e *evictionRecorder) finished(pod *corev1.Pod, _ bool, err error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if err == nil {
		e.gone[podKey(pod)] = true
		return
	}
	e.lastErr[podKey(pod)] = err
}

// report fills the per-pod lists of res once the evictions ended.
func (e *evictionRecorder) report(res *DrainResult, pods []corev1.Pod, drainErr error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	for i := range pods {
		key := podKey(&pods[i])
		if e.gone[key] {
			res.Evicted = append(res.Evicted, key)
			continue
		}
		res.Failed = append(res.Failed, e.failure(key))
		if apierrors.IsTooManyRequests(e.lastErr[key]) {
			res.BlockedByPDB = append(res.BlockedByPDB, key)
		}
	}
	// A failure before any eviction (e.g. discovering the eviction API)
	// names no pod; report it as a whole.
	if drainErr != nil && len(e.started) == 0 {
		res.Errors = append(res.Errors, drainErr.Error())
	}
	slices.Sort(res.Evicted)
	slices.Sort(res.BlockedByPDB)
	slices.SortFunc(res.Failed, func(a, b PodFailure) int { return strings.Compare(a.Pod, b.Pod) })
	res.Completed = len(res.Failed) == 0 && len(res.Errors) == 0
}

// failure explains why the pod key is still on the node.
func (e *evictionRecorder) failure(key string) PodFailure {
	why := reasonNotEvicted
	switch {
	case e.lastErr[key] != nil:
		return PodFailure{Pod: key, Error: e.lastErr[key].Error()}
	case e.accepted[key]:
		why = reasonNotTerminated
	}
	return PodFailure{Pod: key, Error: why.Text(nil), Reason: why.String()}
}
