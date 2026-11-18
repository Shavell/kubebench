package access

import (
	"context"
	"log"
	"slices"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/client-go/dynamic"

	"kubebench/internal/openshift"
)

// OpenShift SCC grants (§7.15): a role that may "use" a privileged or
// anyuid SecurityContextConstraints lets its subjects run pods with it.

// Risk ids of SCC grants.
const (
	riskSCCPrivileged = "scc-privileged" // use of an SCC with privileged or host access
	riskSCCAnyUID     = "scc-anyuid"     // use of an SCC that allows running as root
)

// sccLevels maps the cluster's risky SCCs to their level
// (openshift.LevelPrivileged or LevelAnyUID); nil on a cluster without
// SCCs, where no role's SCC rules mean anything. When the SCCs cannot be
// listed, OpenShift's default risky SCCs stand in.
func sccLevels(ctx context.Context, dyn dynamic.Interface) map[string]string {
	sccs, served, err := openshift.ListSCCs(ctx, dyn)
	switch {
	case !served:
		return nil
	case err != nil:
		log.Printf("[access] listing SCCs: %v; judging SCC grants by OpenShift's default SCC names", err)
		return openshift.RiskyLevels(nil)
	default:
		return openshift.RiskyLevels(sccs)
	}
}

// sccRisks reports the first rule that may use a privileged SCC
// (critical) and the first that may use an anyuid one (high). A rule
// without resourceNames covers every SCC; one with names, those SCCs.
func sccRisks(rules []rbacv1.PolicyRule, levels map[string]string) []Risk {
	if len(levels) == 0 {
		return nil
	}
	found := map[string]*rbacv1.PolicyRule{}
	for i := range rules {
		r := &rules[i]
		unnamed := *r
		unnamed.ResourceNames = nil
		if !can(&unnamed, "security.openshift.io", "securitycontextconstraints", "use") {
			continue
		}
		covered := func(name string) bool { return len(r.ResourceNames) == 0 || slices.Contains(r.ResourceNames, name) }
		for name, level := range levels {
			if covered(name) && found[level] == nil {
				found[level] = r
			}
		}
	}
	var out []Risk
	if r := found[openshift.LevelPrivileged]; r != nil {
		out = append(out, Risk{ID: riskSCCPrivileged, Severity: severityCritical, Rule: *r})
	}
	if r := found[openshift.LevelAnyUID]; r != nil {
		out = append(out, Risk{ID: riskSCCAnyUID, Severity: severityHigh, Rule: *r})
	}
	return out
}
