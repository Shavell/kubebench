package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/runtime/schema"
)

func TestResolveTopic(t *testing.T) {
	rt, _ := editingRuntime(t)
	defer rt.Close()
	for topic, want := range map[string]schema.GroupVersionResource{
		"deployments":                            {Group: "apps", Version: "v1", Resource: "deployments"},
		"deployments.apps":                       {Group: "apps", Version: "v1", Resource: "deployments"},
		"configmaps":                             {Version: "v1", Resource: "configmaps"},
		"clusterroles.rbac.authorization.k8s.io": {Group: "rbac.authorization.k8s.io", Version: "v1", Resource: "clusterroles"},
	} {
		got, err := rt.resolveTopic(topic)
		if err != nil || got != want {
			t.Errorf("resolveTopic(%q) = %v, %v; want %v", topic, got, err, want)
		}
	}
	if _, err := rt.resolveTopic("widgets.example.com"); err == nil || err.Error() != "the cluster does not serve widgets.example.com" {
		t.Errorf("unknown topic: %v", err)
	}
}

// Closing a runtime closes its hub: subscriptions end.
func TestCloseEndsWatchSubscriptions(t *testing.T) {
	rt, _ := editingRuntime(t)
	sub, err := rt.Watch().Subscribe([]string{"widgets.example.com"})
	if err != nil {
		t.Fatal(err)
	}
	rt.Close()
	select {
	case <-sub.Done():
	default:
		t.Fatal("the subscription outlived its runtime")
	}
}
