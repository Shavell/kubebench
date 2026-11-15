package access

import (
	"context"
	"fmt"
	"sort"
	"sync"

	corev1 "k8s.io/api/core/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

// Kinds of RBAC objects and subjects, as Kubernetes spells them.
const (
	kindRole               = "Role"
	kindClusterRole        = "ClusterRole"
	kindRoleBinding        = "RoleBinding"
	kindClusterRoleBinding = "ClusterRoleBinding"
)

// Ref names an RBAC object: a Role, ClusterRole or one of their bindings.
type Ref struct {
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

func (r Ref) key() string { return r.Kind + "/" + r.Namespace + "/" + r.Name }

// binding is a RoleBinding or ClusterRoleBinding in one shape. Scope is
// the namespace a RoleBinding grants in; "" for a ClusterRoleBinding,
// which grants cluster-wide.
type binding struct {
	Ref      Ref
	Scope    string
	RoleRef  Ref
	Subjects []rbacv1.Subject
}

// role is a Role or ClusterRole. ClusterRoles with an aggregationRule are
// used as stored: the aggregation controller keeps their rules filled in.
type role struct {
	Ref        Ref
	Rules      []rbacv1.PolicyRule
	Aggregated bool
}

// snapshot holds the cluster's RBAC objects for answering one request.
type snapshot struct {
	roles           map[string]*role // by Ref.key()
	bindings        []binding        // ClusterRoleBindings first, then by namespace and name
	serviceAccounts []corev1.ServiceAccount
}

// loadSnapshot lists Roles, ClusterRoles, both kinds of bindings and
// ServiceAccounts in all namespaces, in parallel.
func loadSnapshot(ctx context.Context, cs kubernetes.Interface) (*snapshot, error) {
	var (
		roles  *rbacv1.RoleList
		croles *rbacv1.ClusterRoleList
		rbs    *rbacv1.RoleBindingList
		crbs   *rbacv1.ClusterRoleBindingList
		sas    *corev1.ServiceAccountList
	)
	rbac := cs.RbacV1()
	opts := metav1.ListOptions{}
	errs := make([]error, 5)
	var wg sync.WaitGroup
	for i, list := range []func() (err error){
		func() (err error) { roles, err = rbac.Roles("").List(ctx, opts); return },
		func() (err error) { croles, err = rbac.ClusterRoles().List(ctx, opts); return },
		func() (err error) { rbs, err = rbac.RoleBindings("").List(ctx, opts); return },
		func() (err error) { crbs, err = rbac.ClusterRoleBindings().List(ctx, opts); return },
		func() (err error) { sas, err = cs.CoreV1().ServiceAccounts("").List(ctx, opts); return },
	} {
		wg.Go(func() { errs[i] = list() })
	}
	wg.Wait()
	for i, what := range []string{"Roles", "ClusterRoles", "RoleBindings", "ClusterRoleBindings", "ServiceAccounts"} {
		if errs[i] != nil {
			return nil, fmt.Errorf("listing %s: %w", what, errs[i])
		}
	}

	s := &snapshot{roles: map[string]*role{}, serviceAccounts: sas.Items}
	for i := range roles.Items {
		r := &roles.Items[i]
		s.addRole(&role{Ref: Ref{Kind: kindRole, Namespace: r.Namespace, Name: r.Name}, Rules: r.Rules})
	}
	for i := range croles.Items {
		r := &croles.Items[i]
		s.addRole(&role{Ref: Ref{Kind: kindClusterRole, Name: r.Name}, Rules: r.Rules, Aggregated: r.AggregationRule != nil})
	}
	for i := range crbs.Items {
		b := &crbs.Items[i]
		s.bindings = append(s.bindings, binding{
			Ref:      Ref{Kind: kindClusterRoleBinding, Name: b.Name},
			RoleRef:  Ref{Kind: b.RoleRef.Kind, Name: b.RoleRef.Name},
			Subjects: b.Subjects,
		})
	}
	for i := range rbs.Items {
		b := &rbs.Items[i]
		ref := Ref{Kind: b.RoleRef.Kind, Name: b.RoleRef.Name}
		if ref.Kind == kindRole {
			ref.Namespace = b.Namespace
		}
		s.bindings = append(s.bindings, binding{
			Ref:      Ref{Kind: kindRoleBinding, Namespace: b.Namespace, Name: b.Name},
			Scope:    b.Namespace,
			RoleRef:  ref,
			Subjects: b.Subjects,
		})
	}
	sort.SliceStable(s.bindings, func(i, j int) bool {
		a, b := s.bindings[i].Ref, s.bindings[j].Ref
		if a.Kind != b.Kind {
			return a.Kind == kindClusterRoleBinding
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
	sort.Slice(s.serviceAccounts, func(i, j int) bool {
		a, b := &s.serviceAccounts[i], &s.serviceAccounts[j]
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
	return s, nil
}

func (s *snapshot) addRole(r *role) { s.roles[r.Ref.key()] = r }

// roleOf returns the role b refers to, or nil when it does not exist (a
// dangling roleRef grants nothing).
func (s *snapshot) roleOf(b *binding) *role { return s.roles[b.RoleRef.key()] }

// sortedRoles returns every role, ClusterRoles first, then by namespace
// and name.
func (s *snapshot) sortedRoles() []*role {
	out := make([]*role, 0, len(s.roles))
	for _, r := range s.roles {
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i].Ref, out[j].Ref
		if a.Kind != b.Kind {
			return a.Kind == kindClusterRole
		}
		if a.Namespace != b.Namespace {
			return a.Namespace < b.Namespace
		}
		return a.Name < b.Name
	})
	return out
}

// appliesIn reports whether b's grants count for a request in namespace
// ns ("" = cluster-scoped or all namespaces): ClusterRoleBindings always,
// RoleBindings only in their own namespace and never for non-resource
// URLs, which have no namespace.
func (b *binding) appliesIn(r Request) bool {
	if b.Scope == "" {
		return true
	}
	return r.IsResource() && r.Namespace == b.Scope
}
