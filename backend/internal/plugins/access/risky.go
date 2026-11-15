package access

import (
	"net/http"
	"slices"
	"sort"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"

	"kubebench/internal/plugin"
)

// Severities of a risk, highest first.
const (
	severityCritical = "critical"
	severityHigh     = "high"
	severityMedium   = "medium"
)

var severityRank = map[string]int{severityCritical: 3, severityHigh: 2, severityMedium: 1}

// riskCheck flags rules that grant a dangerous permission. IDs are part of
// the API: the UI explains each one.
type riskCheck struct {
	id       string
	severity string
	matches  func(rule *rbacv1.PolicyRule) bool
}

// can reports whether rule grants one of verbs on group/resource (a
// "pods/exec" style resource names a subresource) for any object, so
// rules limited to resourceNames do not count.
func can(rule *rbacv1.PolicyRule, group, resource string, verbs ...string) bool {
	res, sub, _ := strings.Cut(resource, "/")
	for _, verb := range verbs {
		if ruleAllows(rule, Request{Verb: verb, Group: group, Resource: res, Subresource: sub}) {
			return true
		}
	}
	return false
}

func canAny(rule *rbacv1.PolicyRule, group string, resources []string, verbs ...string) bool {
	return slices.ContainsFunc(resources, func(res string) bool { return can(rule, group, res, verbs...) })
}

var mutating = []string{"create", "update", "patch"}

// riskChecks are evaluated per rule. "full-access" supersedes the two
// wildcard checks on the same role.
var riskChecks = []riskCheck{
	{"full-access", severityCritical, func(r *rbacv1.PolicyRule) bool {
		return slices.Contains(r.Verbs, rbacv1.VerbAll) && slices.Contains(r.APIGroups, rbacv1.APIGroupAll) &&
			slices.Contains(r.Resources, rbacv1.ResourceAll) && len(r.ResourceNames) == 0
	}},
	{"escalate-bind", severityCritical, func(r *rbacv1.PolicyRule) bool {
		return canAny(r, rbacv1.GroupName, []string{"roles", "clusterroles"}, "escalate", "bind")
	}},
	{"impersonate", severityCritical, func(r *rbacv1.PolicyRule) bool {
		return canAny(r, "", []string{"users", "groups", "serviceaccounts"}, "impersonate") ||
			can(r, "authentication.k8s.io", "userextras", "impersonate")
	}},
	{"wildcard-verbs", severityHigh, func(r *rbacv1.PolicyRule) bool {
		return slices.Contains(r.Verbs, rbacv1.VerbAll) && len(r.Resources) > 0
	}},
	{"wildcard-resources", severityHigh, func(r *rbacv1.PolicyRule) bool {
		return slices.Contains(r.Resources, rbacv1.ResourceAll)
	}},
	{"secrets-read", severityHigh, func(r *rbacv1.PolicyRule) bool {
		return can(r, "", "secrets", "get", "list", "watch")
	}},
	{"pod-exec", severityHigh, func(r *rbacv1.PolicyRule) bool {
		return canAny(r, "", []string{"pods/exec", "pods/attach"}, "create", "get")
	}},
	{"token-create", severityHigh, func(r *rbacv1.PolicyRule) bool {
		return can(r, "", "serviceaccounts/token", "create")
	}},
	{"rbac-write", severityHigh, func(r *rbacv1.PolicyRule) bool {
		return canAny(r, rbacv1.GroupName, []string{"roles", "clusterroles", "rolebindings", "clusterrolebindings"}, mutating...)
	}},
	{"node-proxy", severityHigh, func(r *rbacv1.PolicyRule) bool {
		return can(r, "", "nodes/proxy", "get", "create")
	}},
	{"webhook-write", severityHigh, func(r *rbacv1.PolicyRule) bool {
		return canAny(r, "admissionregistration.k8s.io",
			[]string{"mutatingwebhookconfigurations", "validatingwebhookconfigurations"}, mutating...)
	}},
	{"csr-approve", severityHigh, func(r *rbacv1.PolicyRule) bool {
		return can(r, "certificates.k8s.io", "certificatesigningrequests/approval", "update", "patch")
	}},
	{"workload-create", severityMedium, func(r *rbacv1.PolicyRule) bool {
		return can(r, "", "pods", "create") ||
			canAny(r, "apps", []string{"deployments", "daemonsets", "statefulsets", "replicasets"}, "create") ||
			canAny(r, "batch", []string{"jobs", "cronjobs"}, "create")
	}},
	{"persistentvolume-create", severityMedium, func(r *rbacv1.PolicyRule) bool {
		return can(r, "", "persistentvolumes", "create")
	}},
}

// Risk is one finding on a role, with the first rule that triggers it.
type Risk struct {
	ID       string            `json:"id"`
	Severity string            `json:"severity"`
	Rule     rbacv1.PolicyRule `json:"rule"`
}

// RiskyRole is one row of GET /risky.
type RiskyRole struct {
	Role       Ref    `json:"role"`
	Aggregated bool   `json:"aggregated,omitempty"`
	Severity   string `json:"severity"`
	Risks      []Risk `json:"risks"`
	// Bindings refer to the role; Subjects counts the distinct subjects
	// they name.
	Bindings []Ref `json:"bindings"`
	Subjects int   `json:"subjects"`
}

// listRisky serves GET /risky.
func listRisky(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
	snap, err := loadSnapshot(r.Context(), c.Runtime.CS)
	if err != nil {
		plugin.WriteK8sError(w, err)
		return
	}
	plugin.WriteJSON(w, snap.risky())
}

// risksOf evaluates every check against rules.
func risksOf(rules []rbacv1.PolicyRule) []Risk {
	var out []Risk
	for _, check := range riskChecks {
		for i := range rules {
			if check.matches(&rules[i]) {
				out = append(out, Risk{ID: check.id, Severity: check.severity, Rule: rules[i]})
				break
			}
		}
	}
	if slices.ContainsFunc(out, func(r Risk) bool { return r.ID == "full-access" }) {
		out = slices.DeleteFunc(out, func(r Risk) bool { return r.ID == "wildcard-verbs" || r.ID == "wildcard-resources" })
	}
	return out
}

// risky lists the roles with at least one risk, most severe first, then
// those bound to the most subjects.
func (s *snapshot) risky() []RiskyRole {
	usedBy := map[string][]*binding{}
	for i := range s.bindings {
		b := &s.bindings[i]
		usedBy[b.RoleRef.key()] = append(usedBy[b.RoleRef.key()], b)
	}
	out := []RiskyRole{}
	for _, ro := range s.sortedRoles() {
		risks := risksOf(ro.Rules)
		if len(risks) == 0 {
			continue
		}
		row := RiskyRole{Role: ro.Ref, Aggregated: ro.Aggregated, Risks: risks, Bindings: []Ref{}}
		for _, r := range risks {
			if severityRank[r.Severity] > severityRank[row.Severity] {
				row.Severity = r.Severity
			}
		}
		subjects := map[string]bool{}
		for _, b := range usedBy[ro.Ref.key()] {
			row.Bindings = append(row.Bindings, b.Ref)
			for _, bs := range b.Subjects {
				subjects[subjectOf(bs, b.Scope).key()] = true
			}
		}
		row.Subjects = len(subjects)
		out = append(out, row)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if a, b := severityRank[out[i].Severity], severityRank[out[j].Severity]; a != b {
			return a > b
		}
		return out[i].Subjects > out[j].Subjects
	})
	return out
}
