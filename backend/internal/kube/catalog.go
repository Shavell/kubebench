package kube

import "k8s.io/apimachinery/pkg/runtime/schema"

// ResourceType describes a browsable Kubernetes resource. The generic
// lister/getter handle any type registered here via the dynamic client,
// so adding a type is a one-line catalog entry (frontend + backend).
type ResourceType struct {
	Key        string // stable id used by the API (e.g. "configmaps")
	Kind       string // for YAML rendering / detail
	GVR        schema.GroupVersionResource
	Namespaced bool
}

func gvr(group, version, resource string) schema.GroupVersionResource {
	return schema.GroupVersionResource{Group: group, Version: version, Resource: resource}
}

// Catalog is the set of resource types the generic browser supports.
// Pods/Deployments/Services/Nodes/Namespaces keep their rich typed
// handlers; everything else flows through the generic path.
var Catalog = map[string]ResourceType{
	// Config
	"configmaps": {"configmaps", "ConfigMap", gvr("", "v1", "configmaps"), true},
	"secrets":    {"secrets", "Secret", gvr("", "v1", "secrets"), true},
	// Workloads (beyond deployments)
	"statefulsets": {"statefulsets", "StatefulSet", gvr("apps", "v1", "statefulsets"), true},
	"daemonsets":   {"daemonsets", "DaemonSet", gvr("apps", "v1", "daemonsets"), true},
	"replicasets":  {"replicasets", "ReplicaSet", gvr("apps", "v1", "replicasets"), true},
	"jobs":         {"jobs", "Job", gvr("batch", "v1", "jobs"), true},
	"cronjobs":     {"cronjobs", "CronJob", gvr("batch", "v1", "cronjobs"), true},
	// Network
	"ingresses":       {"ingresses", "Ingress", gvr("networking.k8s.io", "v1", "ingresses"), true},
	"endpoints":       {"endpoints", "Endpoints", gvr("", "v1", "endpoints"), true},
	"networkpolicies": {"networkpolicies", "NetworkPolicy", gvr("networking.k8s.io", "v1", "networkpolicies"), true},
	// Storage
	"persistentvolumeclaims": {"persistentvolumeclaims", "PersistentVolumeClaim", gvr("", "v1", "persistentvolumeclaims"), true},
	"persistentvolumes":      {"persistentvolumes", "PersistentVolume", gvr("", "v1", "persistentvolumes"), false},
	"storageclasses":         {"storageclasses", "StorageClass", gvr("storage.k8s.io", "v1", "storageclasses"), false},
	// Access control
	"serviceaccounts":     {"serviceaccounts", "ServiceAccount", gvr("", "v1", "serviceaccounts"), true},
	"roles":               {"roles", "Role", gvr("rbac.authorization.k8s.io", "v1", "roles"), true},
	"rolebindings":        {"rolebindings", "RoleBinding", gvr("rbac.authorization.k8s.io", "v1", "rolebindings"), true},
	"clusterroles":        {"clusterroles", "ClusterRole", gvr("rbac.authorization.k8s.io", "v1", "clusterroles"), false},
	"clusterrolebindings": {"clusterrolebindings", "ClusterRoleBinding", gvr("rbac.authorization.k8s.io", "v1", "clusterrolebindings"), false},
	// Cluster
	"events": {"events", "Event", gvr("", "v1", "events"), true},
}

// catalogByKind resolves a Kind (used by /api/yaml) to its ResourceType.
var catalogByKind = func() map[string]ResourceType {
	m := make(map[string]ResourceType, len(Catalog))
	for _, rt := range Catalog {
		m[rt.Kind] = rt
	}
	return m
}()
