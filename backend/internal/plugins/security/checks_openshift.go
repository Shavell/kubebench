package security

import (
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"

	"kubebench/internal/openshift"
)

// OpenShift checks (§7.15) on SecurityContextConstraints, after the
// OpenShift security guide ("Managing security context constraints") and
// the CIS Red Hat OpenShift Container Platform Benchmark (5.2: minimize
// the admission of privileged, root and host-access containers).

// checkSCCSubjects reports a privileged or anyuid SCC that lists users or
// groups that are not the cluster's own in its users and groups fields.
func checkSCCSubjects(s *scan, scc *openshift.SCC) ([]string, bool) {
	if !scc.Risky() {
		return nil, false
	}
	var ev []string
	for _, u := range scc.Users {
		if s.grantee(rbacv1.Subject{Kind: rbacv1.UserKind, Name: u}, "") {
			ev = append(ev, "User "+u)
		}
	}
	for _, g := range scc.Groups {
		if s.grantee(rbacv1.Subject{Kind: rbacv1.GroupKind, Name: g}, "") {
			ev = append(ev, "Group "+g)
		}
	}
	return found(ev)
}

// checkSCCBinding reports a binding whose role may "use" privileged or
// anyuid SCCs, for the subjects that are not the cluster's own.
func checkSCCBinding(s *scan, b *bindingInfo) ([]string, bool) {
	if s.scc == nil { // not an OpenShift cluster
		return nil, false
	}
	ns := b.Namespace
	if b.roleRef.Kind == "ClusterRole" {
		ns = ""
	}
	sccs := s.scc.riskySCCsOf(s.scc.rules[objectKey(b.roleRef.Kind, ns, b.roleRef.Name)])
	if len(sccs) == 0 {
		return nil, false
	}
	var ev []string
	for _, subject := range b.subjects {
		if s.grantee(subject, b.Namespace) {
			ev = append(ev, subjectText(subject, b.Namespace)+" → "+strings.Join(sccs, ", "))
		}
	}
	return found(ev)
}

// checkPrivilegedAdmission reports workloads whose pods OpenShift admitted
// with a privileged or anyuid SCC (the pods' openshift.io/scc annotation).
func checkPrivilegedAdmission(s *scan, w *workload) ([]string, bool) {
	if s.scc == nil { // not an OpenShift cluster
		return nil, false
	}
	var ev []string
	for _, name := range s.scc.admitted[objectKey(w.Kind, w.Namespace, w.Name)] {
		if s.scc.risky[name] {
			ev = append(ev, "scc: "+name)
		}
	}
	return found(ev)
}

// checkCustomSCC reports an SCC that OpenShift does not ship and that
// allows privileged containers or host directories.
func checkCustomSCC(_ *scan, scc *openshift.SCC) ([]string, bool) {
	if scc.Default() {
		return nil, false
	}
	var ev []string
	if scc.AllowPrivilegedContainer {
		ev = append(ev, "allowPrivilegedContainer: true")
	}
	if scc.AllowHostDirVolumePlugin {
		ev = append(ev, "allowHostDirVolumePlugin: true")
	}
	return found(ev)
}
