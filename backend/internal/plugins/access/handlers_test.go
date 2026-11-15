package access

import (
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/runtime"

	"kubebench/internal/plugin"
)

func TestRoutesOnlyRead(t *testing.T) {
	e := newEnv(t)
	want := []string{
		"GET /subjects", "GET /subjects/{kind}/{name}", "GET /who-can",
		"GET /risky", "GET /graph", "POST /review",
	}
	if len(e.access) != len(want) {
		t.Fatalf("routes = %v", e.access)
	}
	for _, route := range want {
		if a, ok := e.access[route]; !ok || a != plugin.AccessRead {
			t.Errorf("%s: access %v, want read", route, a)
		}
	}
}

// names renders who-can subjects as "Kind ns/name via binding".
func names(subjects []SubjectGrants) []string {
	var out []string
	for _, s := range subjects {
		for _, g := range s.Grants {
			out = append(out, s.Kind+" "+s.Namespace+"/"+s.Name+" via "+g.Binding.Kind+" "+g.Binding.Name)
		}
	}
	return out
}

func TestWhoCan(t *testing.T) {
	e := newEnv(t, fixture()...)
	for _, tc := range []struct {
		name  string
		query url.Values
		want  []string
	}{
		{"pods in shop", url.Values{"verb": {"get"}, "resource": {"pods"}, "namespace": {"shop"}}, []string{
			"User /alice via RoleBinding view-shop",
			"Group /devs via RoleBinding view-shop",
			"Group /system:masters via ClusterRoleBinding cluster-admin",
			"ServiceAccount argocd/argocd-application-controller via ClusterRoleBinding argocd-application-controller",
			"ServiceAccount shop/ghost via RoleBinding view-shop",
		}},
		{"a RoleBinding grants in its namespace only", url.Values{"verb": {"get"}, "resource": {"pods"}, "namespace": {"default"}}, []string{
			"Group /system:masters via ClusterRoleBinding cluster-admin",
			"ServiceAccount argocd/argocd-application-controller via ClusterRoleBinding argocd-application-controller",
		}},
		{"subresources need their own grant", url.Values{"verb": {"get"}, "resource": {"pods"}, "subresource": {"log"}, "namespace": {"shop"}}, []string{
			"Group /system:masters via ClusterRoleBinding cluster-admin",
			"ServiceAccount argocd/argocd-application-controller via ClusterRoleBinding argocd-application-controller",
		}},
		{"resourceNames", url.Values{"verb": {"update"}, "resource": {"configmaps"}, "namespace": {"argocd"}, "name": {"argocd-cm"}}, []string{
			"Group /system:masters via ClusterRoleBinding cluster-admin",
			"ServiceAccount argocd/argocd-application-controller via ClusterRoleBinding argocd-application-controller",
			"ServiceAccount argocd/argocd-server via RoleBinding argocd-server",
		}},
		{"resourceNames exclude other names", url.Values{"verb": {"update"}, "resource": {"configmaps"}, "namespace": {"argocd"}, "name": {"other"}}, []string{
			"Group /system:masters via ClusterRoleBinding cluster-admin",
			"ServiceAccount argocd/argocd-application-controller via ClusterRoleBinding argocd-application-controller",
		}},
		{"non-resource URL", url.Values{"verb": {"get"}, "path": {"/api/v1"}}, []string{
			"Group /system:authenticated via ClusterRoleBinding system:discovery",
			"Group /system:masters via ClusterRoleBinding cluster-admin",
		}},
	} {
		body := expect(t, e.do(http.MethodGet, "/who-can?"+tc.query.Encode(), nil), http.StatusOK, "")
		got := names(decode[whoCanResponse](t, body).Subjects)
		if !slices.Equal(got, tc.want) {
			t.Errorf("%s:\n got %q\nwant %q", tc.name, got, tc.want)
		}
	}
	expect(t, e.do(http.MethodGet, "/who-can?resource=pods", nil), http.StatusBadRequest, plugin.CodeBadRequest)
}

func TestWhoCanGrantCarriesRule(t *testing.T) {
	e := newEnv(t, fixture()...)
	body := expect(t, e.do(http.MethodGet, "/who-can?verb=list&resource=secrets&namespace=argocd", nil), http.StatusOK, "")
	res := decode[whoCanResponse](t, body)
	i := slices.IndexFunc(res.Subjects, func(s SubjectGrants) bool { return s.Name == "argocd-server" })
	if i < 0 {
		t.Fatalf("argocd-server missing: %s", body)
	}
	g := res.Subjects[i].Grants[0]
	if g.Role != (Ref{Kind: kindRole, Namespace: "argocd", Name: "argocd-server"}) || g.Scope != "argocd" ||
		!slices.Contains(g.Rule.Resources, "secrets") {
		t.Fatalf("grant = %+v", g)
	}
}

func TestSubjects(t *testing.T) {
	e := newEnv(t, fixture()...)
	rows := decode[[]SubjectSummary](t, expect(t, e.do(http.MethodGet, "/subjects", nil), http.StatusOK, ""))
	byKey := map[string]SubjectSummary{}
	var order []string
	for _, r := range rows {
		byKey[r.key()] = r
		order = append(order, r.Kind)
	}
	if !slices.IsSortedFunc(order, func(a, b string) int { return subjectKindOrder[a] - subjectKindOrder[b] }) {
		t.Errorf("not ordered by kind: %v", order)
	}
	alice := byKey["User//alice"]
	if alice.ClusterBindings != 0 || !slices.Equal(alice.Namespaces, []string{"shop"}) ||
		!slices.Equal(alice.Roles, []Ref{{Kind: kindClusterRole, Name: "view"}}) {
		t.Errorf("alice = %+v", alice)
	}
	if d, ok := byKey["ServiceAccount/default/default"]; !ok || d.ClusterBindings != 0 || len(d.Namespaces) != 0 || d.Missing {
		t.Errorf("an unbound ServiceAccount is listed too: %+v", d)
	}
	if g := byKey["ServiceAccount/shop/ghost"]; !g.Missing {
		t.Errorf("a bound ServiceAccount that does not exist is missing: %+v", g)
	}
	if s := byKey["ServiceAccount/argocd/argocd-server"]; s.Missing || !slices.Equal(s.Namespaces, []string{"argocd"}) {
		t.Errorf("defaulted subject namespace: %+v", s)
	}
}

func TestSubjectDetail(t *testing.T) {
	e := newEnv(t, fixture()...)
	body := expect(t, e.do(http.MethodGet, "/subjects/ServiceAccount/argocd-server?namespace=argocd", nil), http.StatusOK, "")
	d := decode[SubjectDetail](t, body)
	if d.User != "system:serviceaccount:argocd:argocd-server" || d.Missing ||
		!slices.Equal(d.Groups, []string{"system:serviceaccounts", "system:serviceaccounts:argocd", "system:authenticated"}) {
		t.Fatalf("identity = %+v", d)
	}
	var bindings []string
	for _, p := range d.Bindings {
		bindings = append(bindings, p.Binding.Name+"|"+p.Via)
	}
	if want := []string{"system:discovery|system:authenticated", "argocd-server|"}; !slices.Equal(bindings, want) {
		t.Errorf("bindings = %q, want %q", bindings, want)
	}
	if len(d.Rules) != 2 || d.Rules[0].Namespace != "" || d.Rules[1].Namespace != "argocd" || len(d.Rules[1].Rules) != 2 {
		t.Fatalf("rules = %+v", d.Rules)
	}
	if r := d.Rules[1].Rules[1]; !slices.Equal(r.ResourceNames, []string{"argocd-cm"}) || r.Path.Role.Name != "argocd-server" {
		t.Errorf("rule with resourceNames = %+v", r)
	}

	// A user bound to a role that does not exist: the path is shown, no rules.
	d = decode[SubjectDetail](t, expect(t, e.do(http.MethodGet, "/subjects/User/bob", nil), http.StatusOK, ""))
	if len(d.Bindings) != 2 || !d.Bindings[0].RoleMissing || len(d.Rules) != 1 {
		t.Errorf("bob = %+v", d)
	}
	d = decode[SubjectDetail](t, expect(t, e.do(http.MethodGet, "/subjects/ServiceAccount/nobody?namespace=default", nil), http.StatusOK, ""))
	if !d.Missing {
		t.Errorf("unknown ServiceAccount: %+v", d)
	}

	expect(t, e.do(http.MethodGet, "/subjects/Robot/x", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodGet, "/subjects/ServiceAccount/x", nil), http.StatusBadRequest, plugin.CodeBadRequest)
	expect(t, e.do(http.MethodGet, "/subjects/User/alice?namespace=shop", nil), http.StatusBadRequest, plugin.CodeBadRequest)
}

func nodeNames(nodes []GraphNode) []string {
	var out []string
	for _, n := range nodes {
		out = append(out, n.ID)
	}
	return out
}

func TestGraph(t *testing.T) {
	e := newEnv(t, fixture()...)
	g := decode[Graph](t, expect(t, e.do(http.MethodGet, "/graph?namespace=argocd", nil), http.StatusOK, ""))
	if got, want := nodeNames(g.Bindings), []string{
		"b:ClusterRoleBinding//argocd-application-controller", "b:RoleBinding/argocd/argocd-server",
	}; !slices.Equal(got, want) {
		t.Errorf("bindings = %q, want %q", got, want)
	}
	if got, want := nodeNames(g.Subjects), []string{
		"s:ServiceAccount/argocd/argocd-application-controller", "s:ServiceAccount/argocd/argocd-server",
	}; !slices.Equal(got, want) {
		t.Errorf("subjects = %q, want %q", got, want)
	}
	if len(g.Edges) != 4 || g.Truncated {
		t.Errorf("edges = %+v", g.Edges)
	}

	// Without a namespace: every binding but the system: ones, missing roles marked.
	g = decode[Graph](t, expect(t, e.do(http.MethodGet, "/graph", nil), http.StatusOK, ""))
	if slices.ContainsFunc(g.Bindings, func(n GraphNode) bool { return isSystem(n.Name) }) || len(g.Bindings) != 5 {
		t.Errorf("bindings = %q", nodeNames(g.Bindings))
	}
	i := slices.IndexFunc(g.Roles, func(n GraphNode) bool { return n.Name == "no-such-role" })
	if i < 0 || !g.Roles[i].Missing {
		t.Errorf("missing role not marked: %+v", g.Roles)
	}
	ghost := slices.IndexFunc(g.Subjects, func(n GraphNode) bool { return n.Name == "ghost" })
	if ghost < 0 || !g.Subjects[ghost].Missing {
		t.Errorf("missing ServiceAccount not marked: %+v", g.Subjects)
	}
	g = decode[Graph](t, expect(t, e.do(http.MethodGet, "/graph?system=true", nil), http.StatusOK, ""))
	if len(g.Bindings) != 6 {
		t.Errorf("with system: %q", nodeNames(g.Bindings))
	}
	expect(t, e.do(http.MethodGet, "/graph?namespace=Not_Valid", nil), http.StatusBadRequest, plugin.CodeBadRequest)
}

func TestGraphTruncates(t *testing.T) {
	var objects []runtime.Object
	for i := range maxGraphBindings + 5 {
		objects = append(objects, crb(fmt.Sprintf("b-%03d", i), "view", userSubject("u")))
	}
	e := newEnv(t, objects...)
	g := decode[Graph](t, expect(t, e.do(http.MethodGet, "/graph", nil), http.StatusOK, ""))
	if !g.Truncated || len(g.Bindings) != maxGraphBindings || len(g.Subjects) != 1 {
		t.Fatalf("truncated=%v bindings=%d subjects=%d", g.Truncated, len(g.Bindings), len(g.Subjects))
	}
}

func TestRisky(t *testing.T) {
	e := newEnv(t, fixture()...)
	rows := decode[[]RiskyRole](t, expect(t, e.do(http.MethodGet, "/risky", nil), http.StatusOK, ""))
	byName := map[string]RiskyRole{}
	for _, r := range rows {
		byName[r.Role.Name] = r
	}
	if _, ok := byName["view"]; ok {
		t.Error("view is not risky")
	}
	admin := byName["cluster-admin"]
	if admin.Severity != severityCritical || admin.Risks[0].ID != "full-access" || admin.Subjects != 1 ||
		slices.ContainsFunc(admin.Risks, func(r Risk) bool { return r.ID == "wildcard-verbs" }) {
		t.Errorf("cluster-admin = %+v", admin)
	}
	server := byName["argocd-server"]
	if server.Severity != severityHigh || len(server.Risks) != 1 || server.Risks[0].ID != "secrets-read" ||
		!slices.Equal(server.Bindings, []Ref{{Kind: kindRoleBinding, Namespace: "argocd", Name: "argocd-server"}}) {
		t.Errorf("argocd-server = %+v", server)
	}
	if rows[0].Severity != severityCritical || rows[len(rows)-1].Severity != severityHigh {
		t.Errorf("not ordered by severity: %+v", rows)
	}
}

func TestRiskChecks(t *testing.T) {
	ids := func(rules ...rbacv1.PolicyRule) []string {
		var out []string
		for _, r := range risksOf(rules) {
			out = append(out, r.ID)
		}
		return out
	}
	for _, tc := range []struct {
		name string
		rule rbacv1.PolicyRule
		want []string
	}{
		{"read pods", rule(l("get", "list"), l(""), l("pods")), nil},
		{"one named secret", rbacv1.PolicyRule{Verbs: l("get"), APIGroups: l(""), Resources: l("secrets"), ResourceNames: l("tls")}, nil},
		{"bind", rule(l("bind"), l("rbac.authorization.k8s.io"), l("clusterroles")), []string{"escalate-bind"}},
		{"impersonate", rule(l("impersonate"), l(""), l("serviceaccounts")), []string{"impersonate"}},
		{"exec", rule(l("create"), l(""), l("pods/exec")), []string{"pod-exec"}},
		{"tokens", rule(l("create"), l(""), l("serviceaccounts/token")), []string{"token-create"}},
		{"rbac", rule(l("patch"), l("rbac.authorization.k8s.io"), l("rolebindings")), []string{"rbac-write"}},
		{"nodes/proxy", rule(l("get"), l(""), l("nodes/proxy")), []string{"node-proxy"}},
		{"webhooks", rule(l("update"), l("admissionregistration.k8s.io"), l("validatingwebhookconfigurations")), []string{"webhook-write"}},
		{"csr", rule(l("update"), l("certificates.k8s.io"), l("certificatesigningrequests/approval")), []string{"csr-approve"}},
		{"deployments", rule(l("create"), l("apps"), l("deployments")), []string{"workload-create"}},
		{"pv", rule(l("create"), l(""), l("persistentvolumes")), []string{"persistentvolume-create"}},
		{"all verbs on pods (not pods/exec)", rule(l("*"), l(""), l("pods")), []string{"wildcard-verbs", "workload-create"}},
		{"get on everything", rule(l("get"), l("*"), l("*")), []string{"wildcard-resources", "secrets-read", "pod-exec", "node-proxy"}},
	} {
		if got := ids(tc.rule); !slices.Equal(got, tc.want) {
			t.Errorf("%s: risks %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A RoleBinding never grants non-resource URLs, so its role's URL rules
// are not effective rules in that namespace; a ClusterRoleBinding's are.
func TestSubjectDetailNonResourceRulesNeedClusterScope(t *testing.T) {
	metrics := clusterRole("metrics",
		rbacv1.PolicyRule{Verbs: l("get"), NonResourceURLs: l("/metrics")},
		rule(l("get"), l(""), l("pods")))
	e := newEnv(t, metrics,
		rb("shop", "metrics", kindClusterRole, "metrics", userSubject("carol")),
		crb("metrics", "metrics", userSubject("dave")))

	d := decode[SubjectDetail](t, expect(t, e.do(http.MethodGet, "/subjects/User/carol", nil), http.StatusOK, ""))
	if len(d.Rules) != 1 || d.Rules[0].Namespace != "shop" || len(d.Rules[0].Rules) != 1 ||
		!slices.Equal(d.Rules[0].Rules[0].Resources, []string{"pods"}) {
		t.Errorf("carol (RoleBinding) rules = %+v", d.Rules)
	}
	d = decode[SubjectDetail](t, expect(t, e.do(http.MethodGet, "/subjects/User/dave", nil), http.StatusOK, ""))
	if len(d.Rules) != 1 || d.Rules[0].Namespace != "" || len(d.Rules[0].Rules) != 2 {
		t.Errorf("dave (ClusterRoleBinding) rules = %+v", d.Rules)
	}
}
