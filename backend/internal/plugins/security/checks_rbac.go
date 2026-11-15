package security

import (
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
)

// RBAC checks (CIS 5.1.x) on Roles, ClusterRoles and their bindings.

// grants reports whether rule allows one of verbs on resource in group,
// counting "*" in each list.
func grants(rule rbacv1.PolicyRule, group, resource string, verbs ...string) bool {
	return matches(rule.APIGroups, group) && matches(rule.Resources, resource) &&
		slices.ContainsFunc(verbs, func(v string) bool { return matches(rule.Verbs, v) })
}

func matches(list []string, v string) bool {
	return slices.Contains(list, rbacv1.ResourceAll) || slices.Contains(list, v)
}

// ruleText summarises a rule, e.g. "get,list secrets" or "* apps/*".
func ruleText(rule rbacv1.PolicyRule) string {
	if len(rule.NonResourceURLs) > 0 {
		return strings.Join(rule.Verbs, ",") + " " + strings.Join(rule.NonResourceURLs, ",")
	}
	var resources []string
	for _, g := range rule.APIGroups {
		for _, r := range rule.Resources {
			if g == "" {
				resources = append(resources, r)
			} else {
				resources = append(resources, g+"/"+r)
			}
		}
	}
	text := strings.Join(rule.Verbs, ",") + " " + strings.Join(resources, ",")
	if len(rule.ResourceNames) > 0 {
		text += " (" + strings.Join(rule.ResourceNames, ",") + ")"
	}
	return text
}

func checkWildcards(_ *scan, r *roleInfo) ([]string, bool) {
	var ev []string
	for _, rule := range r.rules {
		if slices.Contains(rule.Verbs, rbacv1.VerbAll) || slices.Contains(rule.Resources, rbacv1.ResourceAll) ||
			slices.Contains(rule.APIGroups, rbacv1.APIGroupAll) || slices.Contains(rule.NonResourceURLs, rbacv1.NonResourceAll) {
			ev = append(ev, ruleText(rule))
		}
	}
	return found(ev)
}

func checkSecretsAccess(_ *scan, r *roleInfo) ([]string, bool) {
	var ev []string
	for _, rule := range r.rules {
		if grants(rule, "", "secrets", "get", "list", "watch") {
			ev = append(ev, ruleText(rule))
		}
	}
	return found(ev)
}

func checkPodCreate(_ *scan, r *roleInfo) ([]string, bool) {
	var ev []string
	for _, rule := range r.rules {
		if grants(rule, "", "pods", "create") || grants(rule, "", "pods/exec", "create", "get") ||
			grants(rule, "", "pods/attach", "create", "get") {
			ev = append(ev, ruleText(rule))
		}
	}
	return found(ev)
}

// checkEscalation reports bind and escalate on roles and impersonate on
// identities: each lets a subject exceed the permissions it holds.
func checkEscalation(_ *scan, r *roleInfo) ([]string, bool) {
	var ev []string
	for _, rule := range r.rules {
		var verbs []string
		for _, res := range []string{"roles", "clusterroles"} {
			for _, verb := range []string{"bind", "escalate"} {
				if grants(rule, rbacv1.GroupName, res, verb) && !slices.Contains(verbs, verb) {
					verbs = append(verbs, verb)
				}
			}
		}
		for _, res := range []string{"users", "groups", "serviceaccounts"} {
			if grants(rule, "", res, "impersonate") && !slices.Contains(verbs, "impersonate") {
				verbs = append(verbs, "impersonate")
			}
		}
		if len(verbs) > 0 {
			ev = append(ev, strings.Join(verbs, ", ")+": "+ruleText(rule))
		}
	}
	return found(ev)
}

func checkClusterAdminBinding(_ *scan, b *bindingInfo) ([]string, bool) {
	if b.roleRef.Kind != "ClusterRole" || b.roleRef.Name != "cluster-admin" {
		return nil, false
	}
	var ev []string
	for _, s := range b.subjects {
		ev = append(ev, subjectText(s, b.Namespace))
	}
	return found(ev)
}
