package security

import (
	"slices"
	"strings"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const rbacAPIVersion = "rbac.authorization.k8s.io/v1"

// systemNamespaces are left out of a scan unless it includes them.
var systemNamespaces = []string{"kube-system", "kube-public", "kube-node-lease"}

func isSystemNamespace(ns string) bool { return slices.Contains(systemNamespaces, ns) }

// workload is a pod template the workload checks read: a controller's
// template, or a Pod no scanned controller manages. Pods of Deployments,
// Jobs of CronJobs and so on are left out, so each problem is reported
// once, on the object to fix.
type workload struct {
	Resource
	spec        *corev1.PodSpec
	annotations map[string]string // the pod template's
}

// containers are every container of the pod: init, regular and ephemeral.
func (w *workload) containers() []corev1.Container {
	all := slices.Concat(w.spec.InitContainers, w.spec.Containers)
	for _, ec := range w.spec.EphemeralContainers {
		all = append(all, corev1.Container(ec.EphemeralContainerCommon))
	}
	return all
}

// longRunning reports whether the workload serves rather than runs to
// completion; only those need probes.
func (w *workload) longRunning() bool { return w.Kind != "Job" && w.Kind != "CronJob" }

// roleInfo is a Role or ClusterRole.
type roleInfo struct {
	Resource
	rules []rbacv1.PolicyRule
}

// bindingInfo is a RoleBinding or ClusterRoleBinding.
type bindingInfo struct {
	Resource
	roleRef  rbacv1.RoleRef
	subjects []rbacv1.Subject
	system   bool
}

// subjectText names a binding subject, e.g. "ServiceAccount shop/web".
func subjectText(s rbacv1.Subject, bindingNS string) string {
	if s.Kind == rbacv1.ServiceAccountKind {
		ns := s.Namespace
		if ns == "" {
			ns = bindingNS
		}
		return s.Kind + " " + ns + "/" + s.Name
	}
	return s.Kind + " " + s.Name
}

// controlledBy reports whether obj is controlled by an object of one of kinds.
func controlledBy(obj metav1.Object, kinds ...string) bool {
	owner := metav1.GetControllerOf(obj)
	return owner != nil && slices.Contains(kinds, owner.Kind)
}

// workloads builds the workload subjects of inv in the scan's scope.
func workloads(inv *inventory, inScope func(ns string) bool) []*workload {
	var out []*workload
	add := func(apiVersion, kind string, meta metav1.ObjectMeta, tmpl *corev1.PodTemplateSpec) {
		if !inScope(meta.Namespace) {
			return
		}
		out = append(out, &workload{
			Resource:    Resource{APIVersion: apiVersion, Kind: kind, Namespace: meta.Namespace, Name: meta.Name},
			spec:        &tmpl.Spec,
			annotations: tmpl.Annotations,
		})
	}
	for i := range inv.deployments {
		d := &inv.deployments[i]
		add("apps/v1", "Deployment", d.ObjectMeta, &d.Spec.Template)
	}
	for i := range inv.statefulSets {
		s := &inv.statefulSets[i]
		add("apps/v1", "StatefulSet", s.ObjectMeta, &s.Spec.Template)
	}
	for i := range inv.daemonSets {
		d := &inv.daemonSets[i]
		add("apps/v1", "DaemonSet", d.ObjectMeta, &d.Spec.Template)
	}
	for i := range inv.replicaSets {
		r := &inv.replicaSets[i]
		if !controlledBy(r, "Deployment") {
			add("apps/v1", "ReplicaSet", r.ObjectMeta, &r.Spec.Template)
		}
	}
	for i := range inv.cronJobs {
		c := &inv.cronJobs[i]
		add("batch/v1", "CronJob", c.ObjectMeta, &c.Spec.JobTemplate.Spec.Template)
	}
	for i := range inv.jobs {
		j := &inv.jobs[i]
		if !controlledBy(j, "CronJob") {
			add("batch/v1", "Job", j.ObjectMeta, &j.Spec.Template)
		}
	}
	for i := range inv.pods {
		p := &inv.pods[i]
		// A pod whose controller is scanned is reported through it; one
		// managed by anything else (an operator, a mirror pod's node) is
		// scanned itself.
		if controlledBy(p, "ReplicaSet", "StatefulSet", "DaemonSet", "Job") {
			continue
		}
		add("v1", "Pod", p.ObjectMeta, &corev1.PodTemplateSpec{ObjectMeta: p.ObjectMeta, Spec: p.Spec})
	}
	return out
}

// systemRBAC reports whether an RBAC object belongs to the cluster itself:
// the bootstrap roles and bindings (system:* names, the rbac-defaults label).
func systemRBAC(meta metav1.ObjectMeta) bool {
	return strings.HasPrefix(meta.Name, "system:") || bootstrapped(meta)
}

// bootstrapped reports whether the API server created the object from its
// bootstrap policy.
func bootstrapped(meta metav1.ObjectMeta) bool {
	return meta.Labels["kubernetes.io/bootstrapping"] == "rbac-defaults"
}

// systemBinding reports whether a binding (in namespace ns, "" for a
// ClusterRoleBinding) is the cluster's own: bootstrapped, or not granting
// to everyone and in a system namespace, system:-named or bound to system
// subjects only. A binding to everyone is otherwise always in scope.
func systemBinding(meta metav1.ObjectMeta, subjects []rbacv1.Subject, ns string) bool {
	if bootstrapped(meta) {
		return true
	}
	if grantsEveryone(subjects) {
		return false
	}
	return isSystemNamespace(ns) || systemRBAC(meta) || systemSubjects(subjects, ns)
}

// systemSubjects reports whether every subject belongs to the cluster: a
// system: user or group, or a ServiceAccount of a system namespace (also
// when named as a system:serviceaccount(s): user or group). Such bindings
// are add-ons' own (e.g. k3s's helm-kube-system-* bindings).
func systemSubjects(subjects []rbacv1.Subject, bindingNS string) bool {
	return len(subjects) > 0 && !slices.ContainsFunc(subjects, func(s rbacv1.Subject) bool { return !systemSubject(s, bindingNS) })
}

func systemSubject(s rbacv1.Subject, bindingNS string) bool {
	if s.Kind == rbacv1.ServiceAccountKind {
		ns := s.Namespace
		if ns == "" {
			ns = bindingNS
		}
		return isSystemNamespace(ns)
	}
	if ns, ok := serviceAccountNamespace(s); ok {
		return isSystemNamespace(ns)
	}
	return strings.HasPrefix(s.Name, "system:")
}

// everyoneSubjects are the system: user and groups that stand for
// requesters at large: anonymous requests, every authenticated user,
// every ServiceAccount.
var everyoneSubjects = []string{
	"system:anonymous",
	"system:unauthenticated",
	"system:authenticated",
	"system:serviceaccounts",
}

// grantsEveryone reports whether a subject is one of everyoneSubjects.
func grantsEveryone(subjects []rbacv1.Subject) bool {
	return slices.ContainsFunc(subjects, func(s rbacv1.Subject) bool {
		return s.Kind != rbacv1.ServiceAccountKind && slices.Contains(everyoneSubjects, s.Name)
	})
}

// serviceAccountNamespace is the namespace of a user or group that names
// ServiceAccounts: "system:serviceaccount:<ns>:<name>" (a user) or
// "system:serviceaccounts:<ns>" (a group).
func serviceAccountNamespace(s rbacv1.Subject) (string, bool) {
	prefix := "system:serviceaccounts:"
	if s.Kind == rbacv1.UserKind {
		prefix = "system:serviceaccount:"
	}
	rest, ok := strings.CutPrefix(s.Name, prefix)
	if !ok {
		return "", false
	}
	ns, _, _ := strings.Cut(rest, ":")
	return ns, true
}

// bindings builds every binding of inv, marking the cluster's own.
func bindings(inv *inventory) []*bindingInfo {
	var out []*bindingInfo
	for i := range inv.roleBindings {
		b := &inv.roleBindings[i]
		out = append(out, &bindingInfo{
			Resource: Resource{APIVersion: rbacAPIVersion, Kind: "RoleBinding", Namespace: b.Namespace, Name: b.Name},
			roleRef:  b.RoleRef, subjects: b.Subjects,
			system: systemBinding(b.ObjectMeta, b.Subjects, b.Namespace),
		})
	}
	for i := range inv.clusterRoleBindings {
		b := &inv.clusterRoleBindings[i]
		out = append(out, &bindingInfo{
			Resource: Resource{APIVersion: rbacAPIVersion, Kind: "ClusterRoleBinding", Name: b.Name},
			roleRef:  b.RoleRef, subjects: b.Subjects,
			system: systemBinding(b.ObjectMeta, b.Subjects, ""),
		})
	}
	return out
}

// roles builds the role subjects in the scan's scope. Without system
// objects, a ClusterRole is left out when it is the cluster's own, or when
// only the cluster's own bindings use it: its risk is theirs.
func roles(inv *inventory, all []*bindingInfo, includeSystem bool) []*roleInfo {
	var out []*roleInfo
	for i := range inv.roles {
		r := &inv.roles[i]
		if !includeSystem && (isSystemNamespace(r.Namespace) || systemRBAC(r.ObjectMeta)) {
			continue
		}
		out = append(out, &roleInfo{
			Resource: Resource{APIVersion: rbacAPIVersion, Kind: "Role", Namespace: r.Namespace, Name: r.Name},
			rules:    r.Rules,
		})
	}
	for i := range inv.clusterRoles {
		r := &inv.clusterRoles[i]
		if !includeSystem && (systemRBAC(r.ObjectMeta) || onlySystemBound(r.Name, all)) {
			continue
		}
		out = append(out, &roleInfo{
			Resource: Resource{APIVersion: rbacAPIVersion, Kind: "ClusterRole", Name: r.Name},
			rules:    r.Rules,
		})
	}
	return out
}

// onlySystemBound reports whether ClusterRole name is used by at least one
// binding and by system bindings only.
func onlySystemBound(name string, all []*bindingInfo) bool {
	used := false
	for _, b := range all {
		if b.roleRef.Kind != "ClusterRole" || b.roleRef.Name != name {
			continue
		}
		if !b.system {
			return false
		}
		used = true
	}
	return used
}
