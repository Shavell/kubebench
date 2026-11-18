package access

import (
	"net/http"
	"slices"
	"testing"

	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"

	"kubebench/internal/openshift"
)

func useSCC(names ...string) rbacv1.PolicyRule {
	return rbacv1.PolicyRule{Verbs: l("use"), APIGroups: l("security.openshift.io"), Resources: l("securitycontextconstraints"), ResourceNames: names}
}

func TestSCCRisks(t *testing.T) {
	levels := map[string]string{"privileged": openshift.LevelPrivileged, "anyuid": openshift.LevelAnyUID}
	ids := func(rules ...rbacv1.PolicyRule) []string {
		var out []string
		for _, r := range sccRisks(rules, levels) {
			out = append(out, r.ID)
		}
		return out
	}
	for _, tc := range []struct {
		name string
		rule rbacv1.PolicyRule
		want []string
	}{
		{"use privileged", useSCC("privileged"), []string{riskSCCPrivileged}},
		{"use anyuid", useSCC("anyuid"), []string{riskSCCAnyUID}},
		{"use restricted-v2", useSCC("restricted-v2"), nil},
		{"use any SCC", useSCC(), []string{riskSCCPrivileged, riskSCCAnyUID}},
		{"wildcard verbs", rbacv1.PolicyRule{Verbs: l("*"), APIGroups: l("security.openshift.io"), Resources: l("*")}, []string{riskSCCPrivileged, riskSCCAnyUID}},
		{"read SCCs", rbacv1.PolicyRule{Verbs: l("get", "list"), APIGroups: l("security.openshift.io"), Resources: l("securitycontextconstraints")}, nil},
	} {
		if got := ids(tc.rule); !slices.Equal(got, tc.want) {
			t.Errorf("%s: risks %v, want %v", tc.name, got, tc.want)
		}
	}
	if got := sccRisks([]rbacv1.PolicyRule{useSCC()}, nil); got != nil {
		t.Errorf("off OpenShift: %v", got)
	}
}

// On OpenShift, GET /risky lists roles that may use privileged SCCs,
// judged by the cluster's SCCs (a custom privileged one included).
func TestRiskySCCGrants(t *testing.T) {
	e := newEnv(t,
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "system:openshift:scc:privileged"}, Rules: []rbacv1.PolicyRule{useSCC("privileged")}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "use-mine"}, Rules: []rbacv1.PolicyRule{useSCC("mine")}},
		&rbacv1.ClusterRole{ObjectMeta: metav1.ObjectMeta{Name: "use-restricted"}, Rules: []rbacv1.PolicyRule{useSCC("restricted-v2")}},
	)
	e.dyn.ReactionChain = e.dyn.ReactionChain[1:] // now serving SCCs
	for name, fields := range map[string]map[string]any{
		"privileged":    {"allowPrivilegedContainer": true},
		"mine":          {"allowHostNetwork": true},
		"restricted-v2": {"runAsUser": map[string]any{"type": "MustRunAsRange"}},
	} {
		obj := map[string]any{"apiVersion": "security.openshift.io/v1", "kind": "SecurityContextConstraints", "metadata": map[string]any{"name": name}}
		for k, v := range fields {
			obj[k] = v
		}
		if _, err := e.dyn.Resource(openshift.SCCGVR).Create(t.Context(), &unstructured.Unstructured{Object: obj}, metav1.CreateOptions{}); err != nil {
			t.Fatal(err)
		}
	}
	rows := decode[[]RiskyRole](t, expect(t, e.do(http.MethodGet, "/risky", nil), http.StatusOK, ""))
	byName := map[string]RiskyRole{}
	for _, r := range rows {
		byName[r.Role.Name] = r
	}
	for _, name := range []string{"system:openshift:scc:privileged", "use-mine"} {
		r, ok := byName[name]
		if !ok || r.Severity != severityCritical || len(r.Risks) != 1 || r.Risks[0].ID != riskSCCPrivileged {
			t.Errorf("%s = %+v", name, r)
		}
	}
	if _, ok := byName["use-restricted"]; ok {
		t.Error("use of restricted-v2 is not risky")
	}
}
