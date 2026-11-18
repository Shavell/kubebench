package security

import (
	"context"
	"slices"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/dynamic"

	"kubebench/internal/openshift"
	"kubebench/internal/reason"
)

// OpenShift's part of a scan (§7.15): its SecurityContextConstraints, the
// SCC every pod was admitted with and the RBAC grants to use SCCs. The
// OpenShift checks run only on clusters that serve security.openshift.io.

const (
	kindSCCs        = "securitycontextconstraints"
	sccAPIVersion   = "security.openshift.io/v1"
	sccKind         = "SecurityContextConstraints"
	sccGroup        = "security.openshift.io"
	sccResourceName = "securitycontextconstraints"
)

// openShiftInventory is what a scan reads from the OpenShift APIs.
type openShiftInventory struct {
	// served: the cluster serves security.openshift.io; the OpenShift
	// checks run only then.
	served bool
	sccs   []openshift.SCC
	// err is why the SCCs could not be listed although served.
	err *reason.Error
}

// collectOpenShift lists the SCCs into inv. A cluster without the API, or
// a credential that may not list them, is no error; only an ended context
// stops the scan.
func collectOpenShift(ctx context.Context, dyn dynamic.Interface, inv *inventory) error {
	sccs, served, err := openshift.ListSCCs(ctx, dyn)
	if ctx.Err() != nil {
		return ctx.Err()
	}
	inv.openShift = openShiftInventory{served: served, sccs: sccs}
	if err != nil {
		inv.openShift.err = reason.Wrap(err, reasonCannotList, reason.Params{"kind": kindSCCs, "detail": err.Error()})
	}
	return nil
}

// sccIndex is the scan's view of the SCCs: how risky each one is, which
// SCCs pods were admitted with, and which roles may use which SCCs.
type sccIndex struct {
	sccs []*openshift.SCC
	// risky names the privileged and anyuid SCCs; when the SCCs could not
	// be listed, OpenShift's default risky ones.
	risky map[string]bool
	// admitted lists, per workload ("<Kind>/<namespace>/<name>"), the SCCs
	// its pods were admitted with, in order of first appearance.
	admitted map[string][]string
	// rules are every role's rules, by "<Kind>/<namespace>/<name>".
	rules map[string][]rbacv1.PolicyRule
}

func newSCCIndex(inv *inventory) *sccIndex {
	idx := &sccIndex{risky: map[string]bool{}, admitted: map[string][]string{}, rules: map[string][]rbacv1.PolicyRule{}}
	var listed []openshift.SCC
	if inv.openShift.err == nil {
		listed = inv.openShift.sccs
		if listed == nil {
			listed = []openshift.SCC{}
		}
	}
	for i := range inv.openShift.sccs {
		idx.sccs = append(idx.sccs, &inv.openShift.sccs[i])
	}
	for name := range openshift.RiskyLevels(listed) {
		idx.risky[name] = true
	}
	for i := range inv.roles {
		r := &inv.roles[i]
		idx.rules[objectKey("Role", r.Namespace, r.Name)] = r.Rules
	}
	for i := range inv.clusterRoles {
		r := &inv.clusterRoles[i]
		idx.rules[objectKey("ClusterRole", "", r.Name)] = r.Rules
	}
	idx.indexAdmitted(inv)
	return idx
}

// objectKey names an object in the scan's indexes: "<Kind>/<namespace>/<name>".
func objectKey(kind, namespace, name string) string { return kind + "/" + namespace + "/" + name }

// indexAdmitted maps each pod's openshift.io/scc annotation to the
// workload the scan reports it on: its Deployment (through the
// ReplicaSet), CronJob (through the Job), StatefulSet or DaemonSet, or
// the pod itself (see workloads). Build pods are left out: OpenShift's
// build controller runs every build privileged, which who may start
// builds decides, not the pod.
func (idx *sccIndex) indexAdmitted(inv *inventory) {
	deploymentOf := map[string]string{} // ReplicaSet → Deployment
	for i := range inv.replicaSets {
		r := &inv.replicaSets[i]
		if o := metav1.GetControllerOf(r); o != nil && o.Kind == "Deployment" {
			deploymentOf[r.Namespace+"/"+r.Name] = o.Name
		}
	}
	cronJobOf := map[string]string{} // Job → CronJob
	for i := range inv.jobs {
		j := &inv.jobs[i]
		if o := metav1.GetControllerOf(j); o != nil && o.Kind == "CronJob" {
			cronJobOf[j.Namespace+"/"+j.Name] = o.Name
		}
	}
	for i := range inv.pods {
		p := &inv.pods[i]
		scc := p.Annotations[openshift.SCCAnnotation]
		if scc == "" || controlledBy(p, "Build") {
			continue
		}
		key := podWorkloadKey(p, deploymentOf, cronJobOf)
		if !slices.Contains(idx.admitted[key], scc) {
			idx.admitted[key] = append(idx.admitted[key], scc)
		}
	}
}

func podWorkloadKey(p *corev1.Pod, deploymentOf, cronJobOf map[string]string) string {
	owner := metav1.GetControllerOf(p)
	if owner == nil {
		return objectKey("Pod", p.Namespace, p.Name)
	}
	switch owner.Kind {
	case "ReplicaSet":
		if d, ok := deploymentOf[p.Namespace+"/"+owner.Name]; ok {
			return objectKey("Deployment", p.Namespace, d)
		}
		return objectKey("ReplicaSet", p.Namespace, owner.Name)
	case "Job":
		if c, ok := cronJobOf[p.Namespace+"/"+owner.Name]; ok {
			return objectKey("CronJob", p.Namespace, c)
		}
		return objectKey("Job", p.Namespace, owner.Name)
	case "StatefulSet", "DaemonSet":
		return objectKey(owner.Kind, p.Namespace, owner.Name)
	default:
		return objectKey("Pod", p.Namespace, p.Name)
	}
}

// everySCC is the evidence of a "use" rule without resourceNames.
const everySCC = "every SCC"

// riskySCCsOf lists the risky SCCs rules let a subject use: those a
// "use" rule names, or everySCC for a rule without resourceNames (when
// the cluster has risky SCCs at all).
func (idx *sccIndex) riskySCCsOf(rules []rbacv1.PolicyRule) []string {
	var out []string
	for _, rule := range rules {
		if !grantsNamed(rule, sccGroup, sccResourceName, "use") {
			continue
		}
		if len(rule.ResourceNames) == 0 {
			if len(idx.risky) > 0 {
				return []string{everySCC}
			}
			continue
		}
		for _, name := range rule.ResourceNames {
			if idx.risky[name] && !slices.Contains(out, name) {
				out = append(out, name)
			}
		}
	}
	return out
}

// grantee reports whether the SCC checks report a grant to a subject: one
// that is not the cluster's own (or stands for everyone), or any subject
// when the scan includes system objects.
func (s *scan) grantee(subject rbacv1.Subject, bindingNS string) bool {
	return s.opts.IncludeSystem || !s.system.systemSubject(subject, bindingNS) || grantsEveryone([]rbacv1.Subject{subject})
}

// sccBindings are the bindings the SCC grant check looks at: unlike the
// other RBAC checks, OpenShift's own "system:openshift:scc:*" names do not
// make a binding the cluster's (oc adm policy add-scc-to-user creates
// them), only the bootstrap label or a system namespace does, unless the
// scan includes system objects.
func sccBindings(all []*bindingInfo, sys systemNamespaces, includeSystem bool) []*bindingInfo {
	var out []*bindingInfo
	for _, b := range all {
		if includeSystem || (!b.bootstrap && !sys.has(b.Namespace)) {
			out = append(out, b)
		}
	}
	return out
}
