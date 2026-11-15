package access

import (
	"slices"
	"strings"

	rbacv1 "k8s.io/api/rbac/v1"
)

// Request is one access question: may a subject perform Verb on a
// resource (Group, Resource, Subresource, Namespace, Name) or on the
// non-resource URL Path? Exactly one of Resource and Path is set.
type Request struct {
	Verb        string `json:"verb"`
	Group       string `json:"group"`
	Resource    string `json:"resource,omitempty"`
	Subresource string `json:"subresource,omitempty"`
	// Namespace is empty for cluster-scoped resources and for requests
	// across all namespaces, which only ClusterRoleBindings can grant.
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name,omitempty"`
	// Path is a non-resource URL such as /healthz or /metrics.
	Path string `json:"path,omitempty"`
}

// IsResource reports whether r asks about an API resource rather than a
// non-resource URL.
func (r Request) IsResource() bool { return r.Path == "" }

// The functions below follow the Kubernetes RBAC authorizer
// (k8s.io/kubernetes/pkg/apis/rbac/v1/evaluation_helpers.go), so the
// answers agree with what the API server decides for the same rules.

// ruleAllows reports whether rule grants r.
func ruleAllows(rule *rbacv1.PolicyRule, r Request) bool {
	if !verbMatches(rule, r.Verb) {
		return false
	}
	if !r.IsResource() {
		return nonResourceURLMatches(rule, r.Path)
	}
	combined := r.Resource
	if r.Subresource != "" {
		combined += "/" + r.Subresource
	}
	return apiGroupMatches(rule, r.Group) &&
		resourceMatches(rule, combined, r.Subresource) &&
		resourceNameMatches(rule, r.Name)
}

// anyRuleAllows reports whether one of rules grants r and returns it.
func anyRuleAllows(rules []rbacv1.PolicyRule, r Request) (*rbacv1.PolicyRule, bool) {
	for i := range rules {
		if ruleAllows(&rules[i], r) {
			return &rules[i], true
		}
	}
	return nil, false
}

func verbMatches(rule *rbacv1.PolicyRule, verb string) bool {
	return slices.Contains(rule.Verbs, rbacv1.VerbAll) || slices.Contains(rule.Verbs, verb)
}

func apiGroupMatches(rule *rbacv1.PolicyRule, group string) bool {
	return slices.Contains(rule.APIGroups, rbacv1.APIGroupAll) || slices.Contains(rule.APIGroups, group)
}

// resourceMatches: "*" matches everything, "pods/log" only that
// subresource, and "*/scale" the scale subresource of every resource.
// A plain "pods" does not cover its subresources.
func resourceMatches(rule *rbacv1.PolicyRule, combined, subresource string) bool {
	for _, res := range rule.Resources {
		switch {
		case res == rbacv1.ResourceAll, res == combined:
			return true
		case subresource != "" && strings.HasPrefix(res, "*/") && res[2:] == subresource:
			return true
		}
	}
	return false
}

// resourceNameMatches: a rule without resourceNames covers every name; a
// rule with them covers only those names, so never a request without a
// name (list, watch, create, deletecollection). resourceNames know no
// wildcard: "*" is a literal name there.
func resourceNameMatches(rule *rbacv1.PolicyRule, name string) bool {
	if len(rule.ResourceNames) == 0 {
		return true
	}
	return slices.Contains(rule.ResourceNames, name)
}

// nonResourceURLMatches: "*" matches every path, "/healthz" that path and
// "/healthz/*" (a trailing star) every path with that prefix.
func nonResourceURLMatches(rule *rbacv1.PolicyRule, path string) bool {
	for _, url := range rule.NonResourceURLs {
		switch {
		case url == rbacv1.NonResourceAll, url == path:
			return true
		case strings.HasSuffix(url, "*") && strings.HasPrefix(path, strings.TrimSuffix(url, "*")):
			return true
		}
	}
	return false
}
