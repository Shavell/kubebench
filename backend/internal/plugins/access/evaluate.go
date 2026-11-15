package access

import (
	"sort"

	rbacv1 "k8s.io/api/rbac/v1"
)

// Path is how a role reaches a subject: through a binding, granting in
// Scope (a namespace, or "" cluster-wide), possibly via one of the
// subject's groups.
type Path struct {
	Binding Ref    `json:"binding"`
	Role    Ref    `json:"role"`
	Scope   string `json:"scope"`
	// Via is the group the binding names when it reaches the subject
	// through a group membership (e.g. system:serviceaccounts).
	Via string `json:"via,omitempty"`
	// RoleMissing marks a binding whose role does not exist: it grants nothing.
	RoleMissing bool `json:"roleMissing,omitempty"`
}

// Grant is one rule that allows a request, with the path that grants it.
type Grant struct {
	Path
	Rule rbacv1.PolicyRule `json:"rule"`
}

// SubjectGrants is a subject of a who-can answer with every grant of the
// request to it.
type SubjectGrants struct {
	Subject
	Grants []Grant `json:"grants"`
}

func pathOf(b *binding, via string, roleMissing bool) Path {
	return Path{Binding: b.Ref, Role: b.RoleRef, Scope: b.Scope, Via: via, RoleMissing: roleMissing}
}

// whoCan lists the binding subjects that r is granted to, as the bindings
// name them: groups stay groups, so "system:authenticated" stands for
// every signed-in user.
func (s *snapshot) whoCan(r Request) []SubjectGrants {
	bySubject := map[string]*SubjectGrants{}
	for i := range s.bindings {
		b := &s.bindings[i]
		if !b.appliesIn(r) {
			continue
		}
		ro := s.roleOf(b)
		if ro == nil {
			continue
		}
		rule, ok := anyRuleAllows(ro.Rules, r)
		if !ok {
			continue
		}
		for _, bs := range b.Subjects {
			sub := subjectOf(bs, b.Scope)
			entry := bySubject[sub.key()]
			if entry == nil {
				entry = &SubjectGrants{Subject: sub}
				bySubject[sub.key()] = entry
			}
			if n := len(entry.Grants); n > 0 && entry.Grants[n-1].Binding == b.Ref {
				continue // the binding names the subject twice
			}
			entry.Grants = append(entry.Grants, Grant{Path: pathOf(b, "", false), Rule: *rule})
		}
	}
	out := make([]SubjectGrants, 0, len(bySubject))
	for _, e := range bySubject {
		out = append(out, *e)
	}
	sort.Slice(out, func(i, j int) bool { return subjectLess(out[i].Subject, out[j].Subject) })
	return out
}

// pathsOf lists the bindings that reach id, in snapshot order.
func (s *snapshot) pathsOf(id identity) []Path {
	var out []Path
	for i := range s.bindings {
		b := &s.bindings[i]
		for _, bs := range b.Subjects {
			if ok, via := id.matches(bs, b.Scope); ok {
				out = append(out, pathOf(b, via, s.roleOf(b) == nil))
				break
			}
		}
	}
	return out
}

// allows answers r for id the way the RBAC authorizer does and returns
// every grant that allows it (none: denied).
func (s *snapshot) allows(id identity, r Request) []Grant {
	var out []Grant
	for i := range s.bindings {
		b := &s.bindings[i]
		if !b.appliesIn(r) {
			continue
		}
		ro := s.roleOf(b)
		if ro == nil {
			continue
		}
		rule, ok := anyRuleAllows(ro.Rules, r)
		if !ok {
			continue
		}
		for _, bs := range b.Subjects {
			if matched, via := id.matches(bs, b.Scope); matched {
				out = append(out, Grant{Path: pathOf(b, via, false), Rule: *rule})
				break
			}
		}
	}
	return out
}

// ScopedRule is one effective rule of a subject with the path it comes from.
type ScopedRule struct {
	rbacv1.PolicyRule
	Path Path `json:"path"`
}

// ScopeRules are a subject's rules that apply in one namespace, or
// cluster-wide when Namespace is "".
type ScopeRules struct {
	Namespace string       `json:"namespace"`
	Rules     []ScopedRule `json:"rules"`
}

// effectiveRules groups the rules of paths by the scope they grant in:
// cluster-wide first, then namespaces in order. Rules granted
// cluster-wide also hold in every namespace; they are listed once.
// Non-resource URL rules are left out of namespaced scopes: a RoleBinding
// never grants them (see binding.appliesIn).
func (s *snapshot) effectiveRules(paths []Path) []ScopeRules {
	byScope := map[string][]ScopedRule{}
	for _, p := range paths {
		ro := s.roles[p.Role.key()]
		if ro == nil {
			continue
		}
		for _, rule := range ro.Rules {
			if p.Scope != "" && len(rule.NonResourceURLs) > 0 {
				continue
			}
			byScope[p.Scope] = append(byScope[p.Scope], ScopedRule{PolicyRule: rule, Path: p})
		}
	}
	out := make([]ScopeRules, 0, len(byScope))
	for ns, rules := range byScope {
		out = append(out, ScopeRules{Namespace: ns, Rules: rules})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Namespace < out[j].Namespace })
	return out
}

var subjectKindOrder = map[string]int{rbacv1.UserKind: 0, rbacv1.GroupKind: 1, rbacv1.ServiceAccountKind: 2}

// subjectLess orders users, then groups, then ServiceAccounts by
// namespace, each by name.
func subjectLess(a, b Subject) bool {
	if a.Kind != b.Kind {
		return subjectKindOrder[a.Kind] < subjectKindOrder[b.Kind]
	}
	if a.Namespace != b.Namespace {
		return a.Namespace < b.Namespace
	}
	return a.Name < b.Name
}
