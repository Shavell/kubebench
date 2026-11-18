package security

import (
	"slices"
	"strings"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// Off OpenShift only Kubernetes' own namespaces are system namespaces:
// kube-flannel or kube-prometheus-stack, and namespaces that merely look
// like OpenShift's, are scanned like any other.
func TestSystemNamespacesOffOpenShift(t *testing.T) {
	e := newEnv(t,
		namespace("kube-system", nil),
		namespace("kube-flannel", nil),
		namespace("kube-prometheus-stack", nil),
		namespace("openshift-demo", map[string]string{labelClusterMonitoring: "true"}),
		deployment("kube-system", "proxy", insecureSpec()),
		deployment("kube-prometheus-stack", "grafana", insecureSpec()),
		deployment("kube-flannel", "flannel", insecureSpec()),
		deployment("openshift-demo", "web", insecureSpec()),
	)
	rep := e.scan(false)
	if want := []string{"kube-system"}; !slices.Equal(rep.SystemNamespaces, want) {
		t.Errorf("systemNamespaces = %v, want %v", rep.SystemNamespaces, want)
	}
	scanned := map[string]bool{}
	for _, f := range rep.Findings {
		scanned[f.Resource.Namespace] = true
	}
	for _, ns := range []string{"kube-prometheus-stack", "kube-flannel", "openshift-demo"} {
		if !scanned[ns] {
			t.Errorf("%s was left out of the scan", ns)
		}
	}
	if scanned["kube-system"] {
		t.Error("kube-system was scanned")
	}
}

func TestSystemNamespacesHas(t *testing.T) {
	for _, tc := range []struct {
		openShift bool
		ns        string
		want      bool
	}{
		{false, "kube-system", true},
		{false, "kube-node-lease", true},
		{false, "kube-flannel", false},
		{false, "openshift", false},
		{false, "openshift-monitoring", false},
		{false, "", false},
		{true, "kube-system", true},
		{true, "kube-flannel", true},
		{true, "openshift", true},
		{true, "openshift-monitoring", true},
		{true, "labelled", true},
		{true, "shop", false},
	} {
		sys := newSystemNamespaces([]corev1.Namespace{*namespace("labelled", map[string]string{labelRunLevel: "0"})}, tc.openShift)
		if got := sys.has(tc.ns); got != tc.want {
			t.Errorf("openShift=%v has(%q) = %v", tc.openShift, tc.ns, got)
		}
	}
	if got := newSystemNamespaces(nil, false).list(nil); strings.Join(got, ",") != "kube-system,kube-public,kube-node-lease" {
		t.Errorf("fixed list off OpenShift = %v", got)
	}
	if got := newSystemNamespaces(nil, true).list(nil); !slices.Contains(got, "openshift") {
		t.Errorf("fixed list on OpenShift = %v", got)
	}
}
