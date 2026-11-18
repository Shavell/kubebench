package security

import (
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// System namespaces are left out of a scan unless it includes them: the
// Kubernetes ones and, on OpenShift (§7.15), the platform's own.
var (
	// kubernetesNamespaces are the system namespaces of every cluster.
	// Other kube-* names (kube-flannel, kube-prometheus-stack) are
	// ordinary namespaces there.
	kubernetesNamespaces = []string{"kube-system", "kube-public", "kube-node-lease"}
	// openShiftPrefixes start the names OpenShift reserves for the
	// platform (kube-*, openshift-*: its operators and components), and
	// "openshift" is its own namespace too.
	openShiftPrefixes  = []string{"kube-", "openshift-"}
	openShiftNamespace = "openshift"
)

// OpenShift labels that mark a namespace as part of the platform: the
// run level of its control-plane components (any value), and cluster
// monitoring of a platform component (true).
const (
	labelRunLevel          = "openshift.io/run-level"
	labelClusterMonitoring = "openshift.io/cluster-monitoring"
)

// systemNamespaces names the system namespaces of one scan: Kubernetes'
// and, on OpenShift, the names, prefixes and labels of the platform's.
type systemNamespaces struct {
	openShift bool
	labeled   map[string]bool
}

// newSystemNamespaces reads the labels of the cluster's namespaces;
// openShift is whether the cluster serves security.openshift.io.
func newSystemNamespaces(nss []corev1.Namespace, openShift bool) systemNamespaces {
	sys := systemNamespaces{openShift: openShift, labeled: map[string]bool{}}
	if !openShift {
		return sys
	}
	for i := range nss {
		labels := nss[i].Labels
		if _, ok := labels[labelRunLevel]; ok || labels[labelClusterMonitoring] == "true" {
			sys.labeled[nss[i].Name] = true
		}
	}
	return sys
}

// has reports whether ns is a system namespace. The empty namespace (a
// cluster-scoped object) is not.
func (sys systemNamespaces) has(ns string) bool {
	if ns == "" {
		return false
	}
	if slices.Contains(kubernetesNamespaces, ns) {
		return true
	}
	return sys.openShift && (ns == openShiftNamespace || sys.labeled[ns] ||
		slices.ContainsFunc(openShiftPrefixes, func(p string) bool { return strings.HasPrefix(ns, p) }))
}

// list is the report's systemNamespaces: the cluster's system namespaces,
// sorted, or the fixed names when the namespaces could not be listed.
func (sys systemNamespaces) list(nss []corev1.Namespace) []string {
	if len(nss) == 0 {
		if sys.openShift {
			return append(slices.Clone(kubernetesNamespaces), openShiftNamespace)
		}
		return slices.Clone(kubernetesNamespaces)
	}
	out := []string{}
	for i := range nss {
		if sys.has(nss[i].Name) {
			out = append(out, nss[i].Name)
		}
	}
	slices.Sort(out)
	return out
}
