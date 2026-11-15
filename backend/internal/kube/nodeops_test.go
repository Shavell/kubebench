package kube

import (
	"context"
	"reflect"
	"testing"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func TestCordonNode(t *testing.T) {
	rt, cs := opsRuntime(t, &corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}})
	ctx := context.Background()
	unschedulable := func() bool {
		n, err := cs.CoreV1().Nodes().Get(ctx, "worker", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		return n.Spec.Unschedulable
	}

	if err := rt.CordonNode(ctx, "worker", true); err != nil || !unschedulable() {
		t.Fatalf("cordon: %v, unschedulable = %v", err, unschedulable())
	}
	if info := MapNode(mustNode(t, rt, "worker")); !info.Unschedulable {
		t.Fatal("the node list does not report the cordon")
	}
	if err := rt.CordonNode(ctx, "worker", false); err != nil || unschedulable() {
		t.Fatalf("uncordon: %v, unschedulable = %v", err, unschedulable())
	}
	if err := rt.CordonNode(ctx, "ghost", true); err == nil {
		t.Fatal("cordoning an unknown node must fail")
	}
}

func TestSetNodeTaints(t *testing.T) {
	added := metav1.NewTime(time.Date(2026, 11, 1, 8, 0, 0, 0, time.UTC))
	rt, cs := opsRuntime(t, &corev1.Node{
		ObjectMeta: metav1.ObjectMeta{Name: "worker"},
		Spec: corev1.NodeSpec{Taints: []corev1.Taint{
			{Key: "maintenance", Effect: corev1.TaintEffectNoExecute, TimeAdded: &added},
			{Key: "dedicated", Value: "gpu", Effect: corev1.TaintEffectNoSchedule},
		}},
	})
	ctx := context.Background()

	want := []Taint{
		{Key: "maintenance", Effect: "NoExecute"},
		{Key: "example.com/zone", Value: "b", Effect: "PreferNoSchedule"},
	}
	got, err := rt.SetNodeTaints(ctx, "worker", want)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("SetNodeTaints = %v, %v", got, err)
	}
	node := mustNode(t, rt, "worker")
	if len(node.Spec.Taints) != 2 || node.Spec.Taints[0].TimeAdded == nil || !node.Spec.Taints[0].TimeAdded.Equal(&added) {
		t.Fatalf("a kept NoExecute taint must keep its timeAdded: %+v", node.Spec.Taints)
	}
	if info := MapNode(node); !reflect.DeepEqual(info.Taints, want) {
		t.Fatalf("node list taints = %v", info.Taints)
	}

	if _, err := rt.SetNodeTaints(ctx, "worker", []Taint{}); err != nil || len(mustNode(t, rt, "worker").Spec.Taints) != 0 {
		t.Fatalf("an empty list removes every taint: %v", err)
	}

	before := len(writes(cs.Actions()))
	for _, bad := range [][]Taint{
		{{Key: "", Effect: "NoSchedule"}},
		{{Key: "bad key", Effect: "NoSchedule"}},
		{{Key: "ok", Value: "not valid!", Effect: "NoSchedule"}},
		{{Key: "ok", Effect: "Sometimes"}},
		{{Key: "twice", Effect: "NoSchedule"}, {Key: "twice", Value: "x", Effect: "NoSchedule"}},
	} {
		_, err := rt.SetNodeTaints(ctx, "worker", bad)
		expectOpError(t, err, OpInvalid)
	}
	if after := len(writes(cs.Actions())); after != before {
		t.Fatal("invalid taints reached the cluster")
	}
	// The same key with different effects is fine.
	if _, err := rt.SetNodeTaints(ctx, "worker", []Taint{
		{Key: "k", Effect: "NoSchedule"}, {Key: "k", Effect: "NoExecute"},
	}); err != nil {
		t.Fatal(err)
	}
}

func mustNode(t *testing.T, rt *Runtime, name string) *corev1.Node {
	t.Helper()
	n, err := rt.CS.CoreV1().Nodes().Get(context.Background(), name, metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	return n
}
