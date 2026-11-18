package security

import (
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// System namespaces are left out of a scan unless it includes them: the
// Kubernetes ones and, on OpenShift (§7.15), the platform's own.
var (
	// systemNamespaceNames are always system namespaces.
	systemNamespaceNames = []string{"kube-system", "kube-public", "kube-node-lease", "openshift"}
	// systemNamespacePrefixes start the names of system namespaces
	// (kube-*, openshift-*): OpenShift's operators and components.
	systemNamespacePrefixes = []string{"kube-", "openshift-"}
)

// OpenShift labels that mark a namespace as part of the platform: the
// run level of its control-plane components (any value), and cluster
// monitoring of a platform component (true).
const (
	labelRunLevel          = "openshift.io/run-level"
	labelClusterMonitoring = "openshift.io/cluster-monitoring"
)

// systemNamespaces names the system namespaces of one scan: the names
// and prefixes above, plus the namespaces whose labels mark them.
type systemNamespaces struct {
	labeled map[string]bool
}

// newSystemNamespaces reads the labels of the cluster's namespaces.
func newSystemNamespaces(nss []corev1.Namespace) systemNamespaces {
	sys := systemNamespaces{labeled: map[string]bool{}}
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
	return slices.Contains(systemNamespaceNames, ns) || sys.labeled[ns] ||
		slices.ContainsFunc(systemNamespacePrefixes, func(p string) bool { return strings.HasPrefix(ns, p) })
}

// list is the report's systemNamespaces: the cluster's system namespaces,
// sorted, or the fixed names when the namespaces could not be listed.
func (sys systemNamespaces) list(nss []corev1.Namespace) []string {
	if len(nss) == 0 {
		return slices.Clone(systemNamespaceNames)
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
