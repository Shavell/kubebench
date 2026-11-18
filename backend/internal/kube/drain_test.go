package kube

import (
	"context"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes/fake"
	k8stesting "k8s.io/client-go/testing"
)

var podsResource = corev1.SchemeGroupVersion.WithResource("pods")

// drainPod is a running pod on node; controller names its owner as
// "<Kind>/<name>" ("" = none).
func drainPod(namespace, name, node, controller string) *corev1.Pod {
	pod := &corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace, UID: types.UID(namespace + "-" + name)},
		Spec:       corev1.PodSpec{NodeName: node},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	if kind, owner, ok := strings.Cut(controller, "/"); ok {
		yes := true
		pod.OwnerReferences = []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: kind, Name: owner, Controller: &yes}}
	}
	return pod
}

// fakeEvictions makes cs behave like an API server for drains: pod lists
// honour the spec.nodeName field selector, the eviction API is discovered,
// and an eviction deletes the pod unless blocked names it (then it answers
// 429, as for a PodDisruptionBudget) or keep does (accepted, but the pod
// stays, like one that is slow to terminate). It returns the evictions seen.
func fakeEvictions(cs *fake.Clientset, blocked, keep map[string]bool) func() []policyv1.Eviction {
	cs.Resources = []*metav1.APIResourceList{{
		GroupVersion: "v1",
		APIResources: []metav1.APIResource{{Name: "pods/eviction", Kind: "Eviction", Group: "policy", Version: "v1"}},
	}}
	cs.PrependReactor("list", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		list := action.(k8stesting.ListAction)
		node, ok := list.GetListRestrictions().Fields.RequiresExactMatch("spec.nodeName")
		if !ok {
			return false, nil, nil
		}
		obj, err := cs.Tracker().List(podsResource, corev1.SchemeGroupVersion.WithKind("Pod"), list.GetNamespace())
		if err != nil {
			return true, nil, err
		}
		all := obj.(*corev1.PodList)
		filtered := &corev1.PodList{}
		for _, p := range all.Items {
			if p.Spec.NodeName == node {
				filtered.Items = append(filtered.Items, p)
			}
		}
		return true, filtered, nil
	})
	var mu sync.Mutex
	var seen []policyv1.Eviction
	cs.PrependReactor("create", "pods", func(action k8stesting.Action) (bool, runtime.Object, error) {
		if action.GetSubresource() != "eviction" {
			return false, nil, nil
		}
		eviction := action.(k8stesting.CreateAction).GetObject().(*policyv1.Eviction)
		mu.Lock()
		seen = append(seen, *eviction)
		mu.Unlock()
		switch {
		case blocked[eviction.Name]:
			return true, nil, apierrors.NewTooManyRequests("Cannot evict pod as it would violate the pod's disruption budget.", 0)
		case keep[eviction.Name]:
			return true, nil, nil
		}
		// The tracker has its own lock, so it can be used inside a reactor.
		return true, nil, cs.Tracker().Delete(podsResource, eviction.Namespace, eviction.Name)
	})
	return func() []policyv1.Eviction {
		mu.Lock()
		defer mu.Unlock()
		return append([]policyv1.Eviction(nil), seen...)
	}
}

func drainNodes() []runtime.Object {
	return []runtime.Object{
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}},
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "other"}},
		&appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Name: "proxy", Namespace: "kube-system"}},
	}
}

func podExists(t *testing.T, cs *fake.Clientset, namespace, name string) bool {
	t.Helper()
	_, err := cs.CoreV1().Pods(namespace).Get(context.Background(), name, metav1.GetOptions{})
	if err != nil && !apierrors.IsNotFound(err) {
		t.Fatal(err)
	}
	return err == nil
}

func TestDrainNode(t *testing.T) {
	mirror := drainPod("kube-system", "etcd-worker", "worker", "")
	mirror.Annotations = map[string]string{corev1.MirrorPodAnnotationKey: "hash"}
	objects := append(drainNodes(),
		drainPod("shop", "web-1", "worker", "ReplicaSet/web-abc"),
		drainPod("shop", "guarded-1", "worker", "ReplicaSet/guarded-xyz"),
		drainPod("kube-system", "proxy-1", "worker", "DaemonSet/proxy"),
		mirror,
		drainPod("shop", "web-2", "other", "ReplicaSet/web-abc"),
	)
	rt, cs := opsRuntime(t, objects...)
	evictions := fakeEvictions(cs, map[string]bool{"guarded-1": true}, nil)

	res, err := rt.DrainNode(context.Background(), "worker", DrainOptions{
		IgnoreDaemonSets: true, GracePeriodSeconds: 15,
		Timeout: 300 * time.Millisecond, RetryDelay: 10 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	if !res.Cordoned || !mustNode(t, rt, "worker").Spec.Unschedulable {
		t.Fatal("a drain cordons the node first")
	}
	if res.Completed {
		t.Fatal("a drain with a blocked pod is not complete")
	}
	if !reflect.DeepEqual(res.Evicted, []string{"shop/web-1"}) {
		t.Fatalf("evicted = %v", res.Evicted)
	}
	if !reflect.DeepEqual(res.BlockedByPDB, []string{"shop/guarded-1"}) {
		t.Fatalf("blockedByPDB = %v", res.BlockedByPDB)
	}
	if len(res.Failed) != 1 || res.Failed[0].Pod != "shop/guarded-1" || !strings.Contains(res.Failed[0].Error, "disruption budget") {
		t.Fatalf("failed = %+v", res.Failed)
	}
	wantSkipped := []SkippedPod{{Pod: "kube-system/etcd-worker", Reason: "mirror"}, {Pod: "kube-system/proxy-1", Reason: "daemonSet"}}
	if !reflect.DeepEqual(res.Skipped, wantSkipped) {
		t.Fatalf("skipped = %+v", res.Skipped)
	}
	if len(res.Errors) != 0 {
		t.Fatalf("errors = %v", res.Errors)
	}

	if podExists(t, cs, "shop", "web-1") || !podExists(t, cs, "shop", "guarded-1") ||
		!podExists(t, cs, "kube-system", "proxy-1") || !podExists(t, cs, "shop", "web-2") {
		t.Fatal("only the evicted pod may be gone")
	}
	seen := evictions()
	if len(seen) < 3 {
		t.Fatalf("the blocked pod must be retried until the timeout, saw %d evictions", len(seen))
	}
	for _, e := range seen {
		if e.Name == "web-2" || e.Name == "proxy-1" || e.Name == "etcd-worker" {
			t.Fatalf("%s must not be evicted", e.Name)
		}
		if e.DeleteOptions == nil || e.DeleteOptions.GracePeriodSeconds == nil || *e.DeleteOptions.GracePeriodSeconds != 15 {
			t.Fatalf("the grace period is passed on: %+v", e.DeleteOptions)
		}
	}
}

func TestDrainRefusesPodsItWouldLose(t *testing.T) {
	scratch := drainPod("shop", "cache-1", "worker", "ReplicaSet/cache-abc")
	scratch.Spec.Volumes = []corev1.Volume{{Name: "tmp", VolumeSource: corev1.VolumeSource{EmptyDir: &corev1.EmptyDirVolumeSource{}}}}
	objects := append(drainNodes(),
		drainPod("shop", "standalone", "worker", ""),
		scratch,
		drainPod("kube-system", "proxy-1", "worker", "DaemonSet/proxy"),
	)

	t.Run("without overrides", func(t *testing.T) {
		rt, cs := opsRuntime(t, objects...)
		evictions := fakeEvictions(cs, nil, nil)
		res, err := rt.DrainNode(context.Background(), "worker", DrainOptions{Timeout: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		joined := strings.Join(res.Errors, "\n")
		for _, want := range []string{"declare no controller", "local storage", "DaemonSet-managed"} {
			if !strings.Contains(joined, want) {
				t.Errorf("errors %q lack %q", joined, want)
			}
		}
		if res.Completed || len(res.Evicted) != 0 || len(evictions()) != 0 {
			t.Fatalf("nothing may be evicted when the drain refuses: %+v", res)
		}
		if !res.Cordoned || !mustNode(t, rt, "worker").Spec.Unschedulable {
			t.Fatal("the node stays cordoned, like kubectl drain")
		}
	})

	t.Run("with overrides", func(t *testing.T) {
		rt, cs := opsRuntime(t, objects...)
		fakeEvictions(cs, nil, nil)
		res, err := rt.DrainNode(context.Background(), "worker", DrainOptions{
			IgnoreDaemonSets: true, DeleteEmptyDirData: true, Force: true, GracePeriodSeconds: -1, Timeout: time.Second,
		})
		if err != nil {
			t.Fatal(err)
		}
		if !res.Completed || !reflect.DeepEqual(res.Evicted, []string{"shop/cache-1", "shop/standalone"}) {
			t.Fatalf("result = %+v", res)
		}
		if !reflect.DeepEqual(res.Skipped, []SkippedPod{{Pod: "kube-system/proxy-1", Reason: "daemonSet"}}) {
			t.Fatalf("skipped = %+v", res.Skipped)
		}
	})
}

func TestDrainReportsPodsStillTerminating(t *testing.T) {
	rt, cs := opsRuntime(t, append(drainNodes(), drainPod("shop", "slow-1", "worker", "ReplicaSet/slow-abc"))...)
	fakeEvictions(cs, nil, map[string]bool{"slow-1": true})
	res, err := rt.DrainNode(context.Background(), "worker", DrainOptions{Timeout: 200 * time.Millisecond})
	if err != nil {
		t.Fatal(err)
	}
	if res.Completed || len(res.Failed) != 1 || !strings.Contains(res.Failed[0].Error, "had not terminated") ||
		res.Failed[0].Reason != "drain.not_terminated" || len(res.BlockedByPDB) != 0 {
		t.Fatalf("result = %+v", res)
	}
}

func TestDrainEmptyNodeAndUnknownNode(t *testing.T) {
	rt, cs := opsRuntime(t, drainNodes()...)
	fakeEvictions(cs, nil, nil)
	res, err := rt.DrainNode(context.Background(), "other", DrainOptions{Timeout: time.Second})
	if err != nil || !res.Completed || len(res.Evicted) != 0 || res.Evicted == nil || res.Failed == nil {
		t.Fatalf("an empty node drains at once, with empty (not null) lists: %+v, %v", res, err)
	}

	if _, err := rt.DrainNode(context.Background(), "ghost", DrainOptions{}); !apierrors.IsNotFound(err) {
		t.Fatalf("unknown node: %v", err)
	}
	for _, a := range writes(cs.Actions()) {
		if patch, ok := a.(k8stesting.PatchAction); ok && patch.GetName() == "ghost" {
			t.Fatal("an unknown node must not be patched")
		}
	}
}
