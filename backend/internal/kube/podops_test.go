package kube

import (
	"context"
	"regexp"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stesting "k8s.io/client-go/testing"
)

func shopPod(name string, containers ...string) *corev1.Pod {
	pod := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "shop"}}
	for _, c := range containers {
		pod.Spec.Containers = append(pod.Spec.Containers, corev1.Container{Name: c, Image: c + ":1"})
	}
	return pod
}

// lastDeleteOptions returns the options of the latest pod deletion.
func lastDeleteOptions(t *testing.T, actions []k8stesting.Action) metav1.DeleteOptions {
	t.Helper()
	for i := len(actions) - 1; i >= 0; i-- {
		if del, ok := actions[i].(k8stesting.DeleteAction); ok {
			return del.GetDeleteOptions()
		}
	}
	t.Fatal("no delete reached the cluster")
	return metav1.DeleteOptions{}
}

func TestDeletePodForce(t *testing.T) {
	rt, cs := opsRuntime(t, shopPod("a"), shopPod("b"))
	ctx := context.Background()
	if err := rt.DeletePod(ctx, "shop", "a", false); err != nil {
		t.Fatal(err)
	}
	if opts := lastDeleteOptions(t, cs.Actions()); opts.GracePeriodSeconds != nil {
		t.Fatalf("a normal delete keeps the pod's grace period: %+v", opts)
	}
	if err := rt.DeletePod(ctx, "shop", "b", true); err != nil {
		t.Fatal(err)
	}
	if opts := lastDeleteOptions(t, cs.Actions()); opts.GracePeriodSeconds == nil || *opts.GracePeriodSeconds != 0 {
		t.Fatalf("force deletes with grace period 0: %+v", opts)
	}
}

func TestDeletePods(t *testing.T) {
	rt, cs := opsRuntime(t, shopPod("a"), shopPod("b"), shopPod("c"))
	ctx := context.Background()
	results, err := rt.DeletePods(ctx, []PodKey{{"shop", "a"}, {"shop", "ghost"}, {"shop", "c"}}, true)
	if err != nil {
		t.Fatal(err)
	}
	if len(results) != 3 || results[0].Name != "a" || results[0].Error != "" ||
		results[1].Name != "ghost" || !strings.Contains(results[1].Error, "not found") ||
		results[2].Name != "c" || results[2].Error != "" {
		t.Fatalf("results = %+v", results)
	}
	if opts := lastDeleteOptions(t, cs.Actions()); opts.GracePeriodSeconds == nil || *opts.GracePeriodSeconds != 0 {
		t.Fatal("force applies to every pod")
	}
	if _, err := cs.CoreV1().Pods("shop").Get(ctx, "b", metav1.GetOptions{}); err != nil {
		t.Fatal("a pod that was not listed was deleted")
	}

	before := len(writes(cs.Actions()))
	tooMany := make([]PodKey, MaxBulkDelete+1)
	for i := range tooMany {
		tooMany[i] = PodKey{"shop", "p"}
	}
	for _, bad := range [][]PodKey{nil, tooMany, {{"shop", ""}}, {{"", "a"}}} {
		_, err := rt.DeletePods(ctx, bad, false)
		expectOpError(t, err, OpInvalid)
	}
	if len(writes(cs.Actions())) != before {
		t.Fatal("an invalid bulk delete reached the cluster")
	}
}

func TestAddDebugContainer(t *testing.T) {
	existing := shopPod("web-1", "app", "sidecar")
	existing.Spec.EphemeralContainers = []corev1.EphemeralContainer{{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debugger-old", Image: "busybox:1.36"}}}
	rt, cs := opsRuntime(t, existing)
	ctx := context.Background()

	debug, err := rt.AddDebugContainer(ctx, "shop", "web-1", "", "app")
	if err != nil {
		t.Fatal(err)
	}
	if !regexp.MustCompile(`^debugger-[a-z0-9]{5}$`).MatchString(debug.Container) || debug.Pod != "web-1" || debug.Namespace != "shop" {
		t.Fatalf("debug container = %+v", debug)
	}
	patched := false
	for _, a := range cs.Actions() {
		if p, ok := a.(k8stesting.PatchAction); ok {
			patched = p.GetSubresource() == "ephemeralcontainers"
		}
	}
	if !patched {
		t.Fatal("the container must be added through pods/ephemeralcontainers")
	}
	pod, err := cs.CoreV1().Pods("shop").Get(ctx, "web-1", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(pod.Spec.EphemeralContainers) != 2 || pod.Spec.EphemeralContainers[0].Name != "debugger-old" {
		t.Fatalf("existing ephemeral containers must stay: %+v", pod.Spec.EphemeralContainers)
	}
	added := pod.Spec.EphemeralContainers[1]
	if added.Name != debug.Container || added.Image != DefaultDebugImage || !added.Stdin || !added.TTY || added.TargetContainerName != "app" {
		t.Fatalf("added container = %+v", added)
	}
	if len(pod.Spec.Containers) != 2 {
		t.Fatalf("the pod's containers must not change: %+v", pod.Spec.Containers)
	}

	other, err := rt.AddDebugContainer(ctx, "shop", "web-1", "nicolaka/netshoot:v0.13", "")
	if err != nil || other.Container == debug.Container {
		t.Fatalf("second debug container: %+v, %v", other, err)
	}

	before := len(writes(cs.Actions()))
	_, err = rt.AddDebugContainer(ctx, "shop", "web-1", "", "nope")
	expectOpError(t, err, OpInvalid)
	_, err = rt.AddDebugContainer(ctx, "shop", "web-1", "busy box", "")
	expectOpError(t, err, OpInvalid)
	if len(writes(cs.Actions())) != before {
		t.Fatal("an invalid debug request reached the cluster")
	}
	if _, err := rt.AddDebugContainer(ctx, "shop", "ghost", "", ""); err == nil {
		t.Fatal("an unknown pod must fail")
	}
}
