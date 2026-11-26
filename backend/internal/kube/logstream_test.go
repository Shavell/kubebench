package kube

import (
	"context"
	"slices"
	"testing"

	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/store"
)

func pod(name string, labels map[string]string, annotations map[string]string, containers ...string) *corev1.Pod {
	p := &corev1.Pod{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "shop", Labels: labels, Annotations: annotations}}
	for _, c := range containers {
		p.Spec.Containers = append(p.Spec.Containers, corev1.Container{Name: c})
	}
	return p
}

func podRuntime(objects ...runtime.Object) (*Runtime, *fake.Clientset) {
	cs := fake.NewClientset(objects...)
	return NewRuntimeForTest(store.Cluster{ID: 1, Name: "test"}, cs, dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())), cs
}

func TestPodContainer(t *testing.T) {
	debug := pod("debugged", nil, nil, "app")
	debug.Spec.InitContainers = []corev1.Container{{Name: "init"}}
	debug.Spec.EphemeralContainers = []corev1.EphemeralContainer{{EphemeralContainerCommon: corev1.EphemeralContainerCommon{Name: "debugger-x1"}}}
	rt, _ := podRuntime(
		pod("web", nil, nil, "app", "sidecar"),
		pod("annotated", nil, map[string]string{defaultContainerAnnotation: "sidecar"}, "app", "sidecar"),
		pod("stale", nil, map[string]string{defaultContainerAnnotation: "gone"}, "app"),
		debug,
	)
	for _, tc := range []struct{ pod, container, want string }{
		{"web", "", "app"},
		{"web", "sidecar", "sidecar"},
		{"annotated", "", "sidecar"},
		{"stale", "", "app"}, // an annotation naming no container is ignored, as by kubectl
		{"debugged", "init", "init"},
		{"debugged", "debugger-x1", "debugger-x1"},
	} {
		got, err := rt.PodContainer(context.Background(), "shop", tc.pod, tc.container)
		if err != nil || got != tc.want {
			t.Errorf("PodContainer(%s, %q) = %q, %v; want %q", tc.pod, tc.container, got, err, tc.want)
		}
	}
	if _, err := rt.PodContainer(context.Background(), "shop", "web", "nope"); !apierrors.IsBadRequest(err) {
		t.Errorf("unknown container: %v, want a bad request", err)
	}
	if _, err := rt.PodContainer(context.Background(), "shop", "gone", ""); !apierrors.IsNotFound(err) {
		t.Errorf("unknown pod: %v, want not found", err)
	}
}

func TestSelectorLogSources(t *testing.T) {
	api := map[string]string{"app": "api"}
	rt, _ := podRuntime(
		pod("api-c", api, nil, "app"),
		pod("api-a", api, nil, "app", "sidecar"),
		pod("api-b", api, map[string]string{defaultContainerAnnotation: "sidecar"}, "app", "sidecar"),
		pod("worker", map[string]string{"app": "worker"}, nil, "app"),
	)
	ctx := context.Background()
	sources, matched, err := rt.SelectorLogSources(ctx, "shop", "app=api", "", 10)
	want := []LogSource{{Pod: "api-a", Container: "app"}, {Pod: "api-b", Container: "sidecar"}, {Pod: "api-c", Container: "app"}}
	if err != nil || matched != 3 || !slices.Equal(sources, want) {
		t.Fatalf("sources = %v (%d), %v; want %v", sources, matched, err, want)
	}
	sources, matched, _ = rt.SelectorLogSources(ctx, "shop", "app=api", "", 2)
	if matched != 3 || !slices.Equal(sources, want[:2]) {
		t.Fatalf("limited sources = %v (%d)", sources, matched)
	}
	sources, matched, _ = rt.SelectorLogSources(ctx, "shop", "app=api", "sidecar", 10)
	if matched != 2 || !slices.Equal(sources, []LogSource{{Pod: "api-a", Container: "sidecar"}, {Pod: "api-b", Container: "sidecar"}}) {
		t.Fatalf("sidecar sources = %v (%d)", sources, matched)
	}
	if sources, matched, _ = rt.SelectorLogSources(ctx, "shop", "app=none", "", 10); len(sources) != 0 || matched != 0 {
		t.Fatalf("no match = %v (%d)", sources, matched)
	}
	if got := (LogSource{Pod: "api-a", Container: "app"}).String(); got != "api-a/app" {
		t.Fatalf("String = %q", got)
	}
}

func TestFollowLog(t *testing.T) {
	rt, cs := podRuntime(pod("web", nil, nil, "app"))
	tail := int64(50)
	body, err := rt.FollowLog(context.Background(), "shop", LogSource{Pod: "web", Container: "app"},
		LogFollowOptions{TailLines: &tail, Previous: true, Timestamps: true, SinceSeconds: 30})
	if err != nil {
		t.Fatal(err)
	}
	body.Close()
	var opts *corev1.PodLogOptions
	for _, a := range cs.Actions() {
		if a.GetSubresource() == "log" {
			opts = a.(k8stesting.GenericAction).GetValue().(*corev1.PodLogOptions)
		}
	}
	if opts == nil || !opts.Follow || opts.Container != "app" || *opts.TailLines != 50 || !opts.Previous ||
		!opts.Timestamps || opts.SinceSeconds == nil || *opts.SinceSeconds != 30 {
		t.Fatalf("log options = %+v", opts)
	}
}

func TestContainerStarted(t *testing.T) {
	status := func(st corev1.ContainerStatus) *corev1.Pod {
		st.Name = "app"
		return &corev1.Pod{Status: corev1.PodStatus{ContainerStatuses: []corev1.ContainerStatus{st}}}
	}
	for name, tc := range map[string]struct {
		pod  *corev1.Pod
		want bool
	}{
		"no status": {&corev1.Pod{}, false},
		"creating":  {status(corev1.ContainerStatus{State: corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "ContainerCreating"}}}), false},
		"running":   {status(corev1.ContainerStatus{State: corev1.ContainerState{Running: &corev1.ContainerStateRunning{}}}), true},
		"completed": {status(corev1.ContainerStatus{State: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{}}}), true},
		"crash loop": {status(corev1.ContainerStatus{
			State:                corev1.ContainerState{Waiting: &corev1.ContainerStateWaiting{Reason: "CrashLoopBackOff"}},
			LastTerminationState: corev1.ContainerState{Terminated: &corev1.ContainerStateTerminated{ExitCode: 1}},
		}), true},
	} {
		if got := containerStarted(tc.pod, "app"); got != tc.want {
			t.Errorf("%s: started = %v, want %v", name, got, tc.want)
		}
	}
}
