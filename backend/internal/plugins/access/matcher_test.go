package access

import (
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
)

func rule(verbs, groups, resources []string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{Verbs: verbs, APIGroups: groups, Resources: resources}
}

func l(s ...string) []string { return s }

func TestRuleAllows(t *testing.T) {
	pods := rule(l("get", "list"), l(""), l("pods"))
	named := rbacv1.PolicyRule{Verbs: l("get", "update"), APIGroups: l(""), Resources: l("configmaps"), ResourceNames: l("app-config")}
	starName := rbacv1.PolicyRule{Verbs: l("get"), APIGroups: l(""), Resources: l("secrets"), ResourceNames: l("*")}
	for _, tc := range []struct {
		name string
		rule rbacv1.PolicyRule
		req  Request
		want bool
	}{
		{"exact", pods, Request{Verb: "get", Resource: "pods"}, true},
		{"other verb", pods, Request{Verb: "delete", Resource: "pods"}, false},
		{"other group", pods, Request{Verb: "get", Group: "apps", Resource: "pods"}, false},
		{"other resource", pods, Request{Verb: "get", Resource: "secrets"}, false},
		{"plain resource does not cover a subresource", pods, Request{Verb: "get", Resource: "pods", Subresource: "log"}, false},
		{"subresource", rule(l("get"), l(""), l("pods/log")), Request{Verb: "get", Resource: "pods", Subresource: "log"}, true},
		{"subresource rule is not the resource", rule(l("get"), l(""), l("pods/log")), Request{Verb: "get", Resource: "pods"}, false},
		{"*/subresource", rule(l("update"), l("*"), l("*/scale")), Request{Verb: "update", Group: "apps", Resource: "deployments", Subresource: "scale"}, true},
		{"*/subresource needs that subresource", rule(l("update"), l("*"), l("*/scale")), Request{Verb: "update", Group: "apps", Resource: "deployments"}, false},
		{"verb wildcard", rule(l("*"), l(""), l("pods")), Request{Verb: "deletecollection", Resource: "pods"}, true},
		{"group wildcard", rule(l("get"), l("*"), l("deployments")), Request{Verb: "get", Group: "apps", Resource: "deployments"}, true},
		{"resource wildcard covers subresources", rule(l("get"), l(""), l("*")), Request{Verb: "get", Resource: "pods", Subresource: "exec"}, true},
		{"cluster-admin", rule(l("*"), l("*"), l("*")), Request{Verb: "escalate", Group: "rbac.authorization.k8s.io", Resource: "clusterroles", Name: "x"}, true},
		{"literal wildcard verb asks for a wildcard grant", pods, Request{Verb: "*", Resource: "pods"}, false},
		{"resourceNames: listed name", named, Request{Verb: "get", Resource: "configmaps", Name: "app-config"}, true},
		{"resourceNames: other name", named, Request{Verb: "get", Resource: "configmaps", Name: "other"}, false},
		{"resourceNames: no name (list)", named, Request{Verb: "get", Resource: "configmaps"}, false},
		{"resourceNames: * is a literal name", starName, Request{Verb: "get", Resource: "secrets", Name: "db"}, false},
		{"no resourceNames covers any name", pods, Request{Verb: "get", Resource: "pods", Name: "web-1"}, true},
		{"resource rule never grants a URL", rule(l("*"), l("*"), l("*")), Request{Verb: "get", Path: "/healthz"}, false},
		{"URL exact", rbacv1.PolicyRule{Verbs: l("get"), NonResourceURLs: l("/healthz")}, Request{Verb: "get", Path: "/healthz"}, true},
		{"URL exact is not a prefix", rbacv1.PolicyRule{Verbs: l("get"), NonResourceURLs: l("/healthz")}, Request{Verb: "get", Path: "/healthz/etcd"}, false},
		{"URL trailing star", rbacv1.PolicyRule{Verbs: l("get"), NonResourceURLs: l("/healthz/*")}, Request{Verb: "get", Path: "/healthz/etcd"}, true},
		{"URL trailing star needs the prefix", rbacv1.PolicyRule{Verbs: l("get"), NonResourceURLs: l("/healthz/*")}, Request{Verb: "get", Path: "/livez"}, false},
		{"URL wildcard", rbacv1.PolicyRule{Verbs: l("*"), NonResourceURLs: l("*")}, Request{Verb: "post", Path: "/anything"}, true},
		{"URL verb", rbacv1.PolicyRule{Verbs: l("get"), NonResourceURLs: l("*")}, Request{Verb: "post", Path: "/metrics"}, false},
	} {
		if got := ruleAllows(&tc.rule, tc.req); got != tc.want {
			t.Errorf("%s: ruleAllows = %v, want %v", tc.name, got, tc.want)
		}
	}
}

func TestBindingAppliesIn(t *testing.T) {
	crb := binding{Ref: Ref{Kind: kindClusterRoleBinding, Name: "c"}}
	rb := binding{Ref: Ref{Kind: kindRoleBinding, Namespace: "shop", Name: "r"}, Scope: "shop"}
	for _, tc := range []struct {
		b    binding
		req  Request
		want bool
	}{
		{crb, Request{Verb: "get", Resource: "pods", Namespace: "shop"}, true},
		{crb, Request{Verb: "list", Resource: "nodes"}, true},
		{crb, Request{Verb: "get", Path: "/metrics"}, true},
		{rb, Request{Verb: "get", Resource: "pods", Namespace: "shop"}, true},
		{rb, Request{Verb: "get", Resource: "pods", Namespace: "other"}, false},
		{rb, Request{Verb: "list", Resource: "pods"}, false}, // all namespaces
		{rb, Request{Verb: "get", Path: "/metrics"}, false},
	} {
		if got := tc.b.appliesIn(tc.req); got != tc.want {
			t.Errorf("%s in %+v: %v, want %v", tc.b.Ref.Kind, tc.req, got, tc.want)
		}
	}
}

func TestIdentityMatches(t *testing.T) {
	sa := identityOf(Subject{Kind: rbacv1.ServiceAccountKind, Namespace: "argocd", Name: "server"})
	alice := identityOf(Subject{Kind: rbacv1.UserKind, Name: "alice"})
	devs := identityOf(Subject{Kind: rbacv1.GroupKind, Name: "devs"})
	for _, tc := range []struct {
		name      string
		id        identity
		subject   rbacv1.Subject
		bindingNS string
		want      bool
		via       string
	}{
		{"sa by name", sa, rbacv1.Subject{Kind: "ServiceAccount", Namespace: "argocd", Name: "server"}, "", true, ""},
		{"sa other namespace", sa, rbacv1.Subject{Kind: "ServiceAccount", Namespace: "default", Name: "server"}, "", false, ""},
		{"sa namespace defaults to the binding's", sa, rbacv1.Subject{Kind: "ServiceAccount", Name: "server"}, "argocd", true, ""},
		{"sa as a user name", sa, rbacv1.Subject{Kind: "User", Name: "system:serviceaccount:argocd:server"}, "", true, ""},
		{"sa via all-serviceaccounts group", sa, rbacv1.Subject{Kind: "Group", Name: "system:serviceaccounts"}, "", true, "system:serviceaccounts"},
		{"sa via namespace group", sa, rbacv1.Subject{Kind: "Group", Name: "system:serviceaccounts:argocd"}, "", true, "system:serviceaccounts:argocd"},
		{"sa via authenticated", sa, rbacv1.Subject{Kind: "Group", Name: "system:authenticated"}, "", true, "system:authenticated"},
		{"sa not another namespace's group", sa, rbacv1.Subject{Kind: "Group", Name: "system:serviceaccounts:default"}, "", false, ""},
		{"user", alice, rbacv1.Subject{Kind: "User", Name: "alice"}, "", true, ""},
		{"user other", alice, rbacv1.Subject{Kind: "User", Name: "bob"}, "", false, ""},
		{"user authenticated", alice, rbacv1.Subject{Kind: "Group", Name: "system:authenticated"}, "", true, "system:authenticated"},
		{"group itself", devs, rbacv1.Subject{Kind: "Group", Name: "devs"}, "", true, "devs"},
		{"group is no user", devs, rbacv1.Subject{Kind: "User", Name: ""}, "", false, ""},
		{"group is not authenticated", devs, rbacv1.Subject{Kind: "Group", Name: "system:authenticated"}, "", false, ""},
	} {
		ok, via := tc.id.matches(tc.subject, tc.bindingNS)
		if ok != tc.want || via != tc.via {
			t.Errorf("%s: matches = %v %q, want %v %q", tc.name, ok, via, tc.want, tc.via)
		}
	}
	anon := identityOf(Subject{Kind: rbacv1.UserKind, Name: "system:anonymous"})
	if ok, _ := anon.matches(rbacv1.Subject{Kind: "Group", Name: "system:authenticated"}, ""); ok {
		t.Error("system:anonymous is not authenticated")
	}
}

func TestValidateRequest(t *testing.T) {
	for _, tc := range []struct {
		req   Request
		field string // "" = valid
	}{
		{Request{Verb: "get", Resource: "pods", Namespace: "shop", Name: "web-1"}, ""},
		{Request{Verb: "get", Path: "/healthz"}, ""},
		{Request{Resource: "pods"}, "verb"},
		{Request{Verb: "get"}, "resource"},
		{Request{Verb: "get", Path: "healthz"}, "path"},
		{Request{Verb: "get", Path: "/x", Resource: "pods"}, "path"},
		{Request{Verb: "get", Resource: "pods", Namespace: "Bad_NS"}, "namespace"},
		{Request{Verb: "get", Resource: "pods/log"}, "resource"},
	} {
		err := validateRequest(tc.req)
		if tc.field == "" {
			if err != nil {
				t.Errorf("%+v: %v", tc.req, err)
			}
			continue
		}
		bad, ok := err.(*invalid)
		if !ok || bad.err.Params["field"] != tc.field {
			t.Errorf("%+v: error %v, want one on %s", tc.req, err, tc.field)
		}
	}
}
