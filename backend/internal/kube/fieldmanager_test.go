package kube

import (
	"context"
	"testing"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8stesting "k8s.io/client-go/testing"
)

// fieldManagerOf is the field manager a write request named, and whether
// the request is one that records it (create, update, patch).
func fieldManagerOf(a k8stesting.Action) (string, bool) {
	switch a := a.(type) {
	case k8stesting.CreateActionImpl:
		return a.CreateOptions.FieldManager, true
	case k8stesting.UpdateActionImpl:
		return a.UpdateOptions.FieldManager, true
	case k8stesting.PatchActionImpl:
		return a.PatchOptions.FieldManager, true
	}
	return "", false
}

// Every change KubeBench makes is attributed to its field manager, so that
// managedFields name KubeBench rather than client-go's default
// ("Go-http-client").
func TestOperationsNameTheFieldManager(t *testing.T) {
	objects := append(rolloutObjects(false), cronJob("nightly-report"),
		&corev1.Node{ObjectMeta: metav1.ObjectMeta{Name: "worker"}})
	rt, cs := opsRuntime(t, objects...)
	ctx := context.Background()
	steps := map[string]func() error{
		"cordon": func() error { return rt.CordonNode(ctx, "worker", true) },
		"taints": func() error {
			_, err := rt.SetNodeTaints(ctx, "worker", []Taint{{Key: "dedicated", Value: "db", Effect: "NoSchedule"}})
			return err
		},
		"suspend":   func() error { return rt.SuspendCronJob(ctx, "shop", "nightly-report", true) },
		"trigger":   func() error { _, err := rt.TriggerCronJob(ctx, "shop", "nightly-report"); return err },
		"pause":     func() error { return rt.PauseDeployment(ctx, "shop", "web", true) },
		"restart":   func() error { return rt.RestartWorkload(ctx, "Deployment", "shop", "web") },
		"scale":     func() error { return rt.ScaleDeployment(ctx, "shop", "web", 3) },
		"namespace": func() error { return rt.CreateNamespace(ctx, "team-a", nil) },
	}
	for name, step := range steps {
		cs.ClearActions()
		if err := step(); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		recorded := 0
		for _, a := range writes(cs.Actions()) {
			if manager, ok := fieldManagerOf(a); ok {
				recorded++
				if manager != FieldManager {
					t.Errorf("%s: %s %s with field manager %q, want %q", name, a.GetVerb(), a.GetResource().Resource, manager, FieldManager)
				}
			}
		}
		if recorded == 0 {
			t.Errorf("%s made no create, update or patch request", name)
		}
	}
}
