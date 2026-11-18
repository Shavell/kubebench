package security

import (
	"encoding/csv"
	"errors"
	"net/http"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	rbacv1 "k8s.io/api/rbac/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	k8stesting "k8s.io/client-go/testing"

	"kubebench/internal/plugin"
)

func TestScanFindsTheInsecureWorkloadOnce(t *testing.T) {
	e := newEnv(t, lab()...)
	rep := e.scan(false)
	got := findingsOf(rep)

	for _, key := range []string{
		"privileged:Deployment:shop:insecure",
		"host-namespaces:Deployment:shop:insecure",
		"host-path:Deployment:shop:insecure",
		"image-tag:Deployment:shop:insecure",
		"resource-limits:Deployment:shop:insecure",
		"probes:Deployment:shop:insecure",
		"secret-env:Deployment:shop:insecure",
		"cluster-admin-binding:ClusterRoleBinding::ci-admin",
		"network-policy:Namespace::shop",
		"secrets-access:ClusterRole::unused-reader",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing finding %s", key)
		}
	}
	f := got["privileged:Deployment:shop:insecure"]
	if f.Severity != SeverityCritical || !slices.Contains(f.Frameworks, "CIS 5.2.2") || f.Resource.APIVersion != "apps/v1" ||
		f.Message != "Containers run privileged, with full access to the node: app" {
		t.Errorf("privileged finding = %+v", f)
	}
	for key := range got {
		switch {
		case strings.Contains(key, ":shop:compliant"), strings.Contains(key, ":shop:debug"), strings.Contains(key, ":shop:nightly"):
			t.Errorf("compliant workload has finding %s", key)
		case strings.Contains(key, "insecure-5d8"), strings.Contains(key, "nightly-29"):
			t.Errorf("controlled object reported instead of its controller: %s", key)
		case strings.Contains(key, "kube-system"), strings.HasSuffix(key, "::cluster-admin"), strings.Contains(key, "addon-role"):
			t.Errorf("system object scanned by default: %s", key)
		}
	}
	if rep.Options.IncludeSystem || rep.Cluster != "lab" || !rep.GeneratedAt.Equal(testNow) {
		t.Errorf("report header = %+v %q %v", rep.Options, rep.Cluster, rep.GeneratedAt)
	}
	if rep.Inventory[kindDeployments] != 3 || len(rep.Skipped) != 0 || len(rep.Warnings) != 0 {
		t.Errorf("inventory %v, skipped %v, warnings %v", rep.Inventory, rep.Skipped, rep.Warnings)
	}
}

func TestScanIncludesSystemObjectsOnRequest(t *testing.T) {
	e := newEnv(t, lab()...)
	got := findingsOf(e.scan(true))
	for _, key := range []string{
		"privileged:Deployment:kube-system:coredns",
		"cluster-admin-binding:ClusterRoleBinding::cluster-admin",
		"rbac-wildcards:ClusterRole::addon-role",
		"rbac-wildcards:ClusterRole::cluster-admin",
		"network-policy:Namespace::kube-system",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("missing finding %s", key)
		}
	}
}

func TestScoreAndCounts(t *testing.T) {
	e := newEnv(t, lab()...)
	rep := e.scan(false)
	if rep.Score.Overall == nil || *rep.Score.Overall <= 0 || *rep.Score.Overall >= 100 {
		t.Fatalf("overall score = %v", rep.Score.Overall)
	}
	// Not an OpenShift cluster: every category but OpenShift's.
	if len(rep.Score.ByCategory) != len(categories)-1 || slices.ContainsFunc(rep.Score.ByCategory, func(c CategoryScore) bool {
		return c.Category == CategoryOpenShift
	}) {
		t.Fatalf("byCategory = %+v", rep.Score.ByCategory)
	}
	total := 0
	for _, c := range rep.Score.ByCategory {
		total += c.Failed
		sum := 0
		for _, n := range c.BySeverity {
			sum += n
		}
		if sum != c.Failed || (c.Evaluated > 0) != (c.Score != nil) {
			t.Errorf("category %+v is inconsistent", c)
		}
	}
	counts := rep.Counts
	if total != counts.Total || counts.Critical+counts.High+counts.Medium+counts.Low != counts.Total || counts.Critical < 2 {
		t.Errorf("counts = %+v, failed %d", counts, total)
	}
}

func TestScoreFormula(t *testing.T) {
	// One critical check over 2 objects with 1 failure, one low check over
	// 4 objects without: (10·1 + 1·4) / (10·2 + 1·4) = 14/24 → 58.
	priv, probes := checksByID["privileged"], checksByID["probes"]
	res := &result{
		evaluated: map[string]int{priv.ID: 2, probes.ID: 4},
		findings:  []Finding{newFinding(priv, Resource{Kind: "Pod", Namespace: "a", Name: "x"}, []string{"c"})},
	}
	rep := buildReport(res, nil, testNow)
	if rep.Score.Overall == nil || *rep.Score.Overall != 58 {
		t.Fatalf("overall = %v, want 58", rep.Score.Overall)
	}
	byCat := map[string]CategoryScore{}
	for _, c := range rep.Score.ByCategory {
		byCat[c.Category] = c
	}
	if s := byCat[CategoryBaseline].Score; s == nil || *s != 50 {
		t.Errorf("baseline = %v, want 50", s)
	}
	if s := byCat[CategoryHygiene].Score; s == nil || *s != 100 {
		t.Errorf("hygiene = %v, want 100", s)
	}
	if byCat[CategoryRBAC].Score != nil {
		t.Error("a category without evaluated checks has a score")
	}
	// Accepting the failure makes it pass.
	key := res.findings[0].Key
	rep = buildReport(res, map[string]Acceptance{key: {By: "a", Reason: "r", At: testNow}}, testNow)
	if *rep.Score.Overall != 100 || rep.Counts.Accepted != 1 || rep.Counts.Critical != 0 {
		t.Errorf("accepted: score %v counts %+v", *rep.Score.Overall, rep.Counts)
	}
	// An expired acceptance does not.
	past := testNow.Add(-time.Hour)
	rep = buildReport(res, map[string]Acceptance{key: {By: "a", Reason: "r", At: past, ExpiresAt: &past}}, testNow)
	if *rep.Score.Overall != 58 || rep.Findings[0].Accepted != nil {
		t.Errorf("expired acceptance still applies: %+v", rep.Findings[0])
	}
}

func TestReportBeforeAndAfterScan(t *testing.T) {
	e := newEnv(t, lab()...)
	if body := expect(t, e.do(http.MethodGet, "/report", nil), http.StatusOK, ""); string(body) != "null\n" {
		t.Fatalf("report before a scan = %s", body)
	}
	expect(t, e.do(http.MethodGet, "/export?format=md", nil), http.StatusNotFound, plugin.CodeNotFound)
	scanned := e.scan(false)
	rep := decode[Report](t, expect(t, e.do(http.MethodGet, "/report", nil), http.StatusOK, ""))
	if len(rep.Findings) != len(scanned.Findings) || rep.Counts != scanned.Counts {
		t.Fatalf("cached report differs from the scan")
	}
}

func TestScanRejectsUnknownOptions(t *testing.T) {
	e := newEnv(t, lab()...)
	expect(t, e.do(http.MethodPost, "/scan", map[string]any{"includeSystems": true}), http.StatusBadRequest, plugin.CodeBadRequest)
	// No body scans with the defaults.
	expect(t, e.do(http.MethodPost, "/scan", nil), http.StatusOK, "")
}

func TestForbiddenKindsSkipTheirChecks(t *testing.T) {
	e := newEnv(t, lab()...)
	forbid := func(resource string) {
		e.cs.PrependReactor("list", resource, func(k8stesting.Action) (bool, runtime.Object, error) {
			return true, nil, apierrors.NewForbidden(schema.GroupResource{Group: "rbac.authorization.k8s.io", Resource: resource}, "", errors.New("RBAC"))
		})
	}
	forbid("roles")
	forbid("clusterroles")
	forbid("statefulsets")
	rep := e.scan(false)
	skipped := map[string]string{}
	for _, s := range rep.Skipped {
		skipped[s.CheckID] = s.Reason
	}
	for _, id := range []string{"rbac-wildcards", "secrets-access", "pod-create", "rbac-escalation"} {
		if !strings.Contains(skipped[id], "cannot list roles") {
			t.Errorf("%s skipped = %q", id, skipped[id])
		}
	}
	if _, ok := skipped["privileged"]; ok || len(skipped) != 4 {
		t.Errorf("skipped = %v", skipped)
	}
	if !slices.ContainsFunc(rep.Warnings, func(w string) bool { return strings.Contains(w, "cannot list statefulsets") }) {
		t.Errorf("warnings = %v", rep.Warnings)
	}
}

func TestScanFailsWhenNothingCanBeListed(t *testing.T) {
	e := newEnv(t, lab()...)
	e.cs.PrependReactor("list", "*", func(k8stesting.Action) (bool, runtime.Object, error) {
		return true, nil, apierrors.NewUnauthorized("token expired")
	})
	expect(t, e.do(http.MethodPost, "/scan", nil), http.StatusBadGateway, plugin.CodeKubernetes)
}

func TestConcurrentScansShareOneRun(t *testing.T) {
	e := newEnv(t, lab()...)
	release := make(chan struct{})
	var once sync.Once
	started := make(chan struct{})
	lists := 0
	var mu sync.Mutex
	e.cs.PrependReactor("list", "namespaces", func(k8stesting.Action) (bool, runtime.Object, error) {
		mu.Lock()
		lists++
		mu.Unlock()
		once.Do(func() { close(started) })
		<-release
		return false, nil, nil
	})
	done := make(chan int, 2)
	go func() { done <- e.do(http.MethodPost, "/scan", nil).Code }()
	<-started
	// Other options are refused while the scan runs; the same ones join it.
	expect(t, e.do(http.MethodPost, "/scan", map[string]bool{"includeSystem": true}), http.StatusConflict, plugin.CodeConflict)
	go func() { done <- e.do(http.MethodPost, "/scan", map[string]bool{"includeSystem": false}).Code }()
	time.Sleep(100 * time.Millisecond) // lets the second request reach the running scan
	close(release)
	for range 2 {
		if code := <-done; code != http.StatusOK {
			t.Fatalf("scan status = %d", code)
		}
	}
	if lists != 1 {
		t.Fatalf("namespaces listed %d times, want once", lists)
	}
}

func TestAcceptAndUnaccept(t *testing.T) {
	e := newEnv(t, lab()...)
	e.scan(false)
	key := "privileged:Deployment:shop:insecure"
	expires := testNow.Add(30 * 24 * time.Hour)

	a := decode[Acceptance](t, expect(t, e.do(http.MethodPost, "/accept", map[string]any{
		"key": key, "reason": "  legacy agent, replaced in Q1  ", "expiresAt": expires.Format(time.RFC3339),
	}), http.StatusOK, ""))
	if a.By != "alice" || a.Reason != "legacy agent, replaced in Q1" || !a.At.Equal(testNow) || !a.ExpiresAt.Equal(expires) {
		t.Fatalf("acceptance = %+v", a)
	}
	if want := "finding/" + key + ` {"checkId":"privileged","expiresAt":"2026-12-14T12:00:00Z"}`; len(e.audits) != 1 || e.audits[0] != want {
		t.Fatalf("audits = %q, want %q", e.audits, want)
	}
	if _, ok := e.kv.m["cluster/7/accepted/"+key]; !ok {
		t.Fatalf("kv = %v", e.kv.m)
	}

	rep := decode[Report](t, expect(t, e.do(http.MethodGet, "/report", nil), http.StatusOK, ""))
	if f := findingsOf(&rep)[key]; f.Accepted == nil || f.Accepted.By != "alice" || rep.Counts.Accepted != 1 {
		t.Fatalf("finding = %+v, counts %+v", f, rep.Counts)
	}
	list := decode[[]AcceptedRisk](t, expect(t, e.do(http.MethodGet, "/accepted", nil), http.StatusOK, ""))
	if len(list) != 1 || list[0].Key != key || list[0].CheckID != "privileged" || list[0].Expired ||
		list[0].Resource != (Resource{APIVersion: "apps/v1", Kind: "Deployment", Namespace: "shop", Name: "insecure"}) {
		t.Fatalf("accepted = %+v", list)
	}

	// Past its expiry the risk is open again, and listed as expired.
	e.p.now = func() time.Time { return expires.Add(time.Second) }
	rep = decode[Report](t, expect(t, e.do(http.MethodGet, "/report", nil), http.StatusOK, ""))
	if findingsOf(&rep)[key].Accepted != nil {
		t.Fatal("expired acceptance still applies")
	}
	list = decode[[]AcceptedRisk](t, expect(t, e.do(http.MethodGet, "/accepted", nil), http.StatusOK, ""))
	if !list[0].Expired {
		t.Fatal("expired acceptance not marked")
	}

	expect(t, e.do(http.MethodDelete, "/accept/"+key, nil), http.StatusOK, "")
	expect(t, e.do(http.MethodDelete, "/accept/"+key, nil), http.StatusNotFound, plugin.CodeNotFound)
	if len(e.kv.m) != 0 {
		t.Fatalf("kv = %v", e.kv.m)
	}
}

func TestAcceptValidation(t *testing.T) {
	e := newEnv(t)
	past := testNow.Add(-time.Minute).Format(time.RFC3339)
	for _, body := range []map[string]any{
		{"key": "privileged:Deployment:shop:web"},                                       // no reason
		{"key": "privileged:Deployment:shop:web", "reason": "   "},                      // blank reason
		{"key": "privileged:Deployment:shop:web", "reason": strings.Repeat("x", 501)},   // too long
		{"key": "privileged:Deployment:shop:web", "reason": "r", "expiresAt": past},     // expired
		{"key": "nope:Deployment:shop:web", "reason": "r"},                              // unknown check
		{"key": "privileged:Secret:shop:web", "reason": "r"},                            // unknown kind
		{"key": "privileged:Deployment:shop", "reason": "r"},                            // malformed
		{"key": "privileged:Deployment:shop:web", "reason": "r", "by": "mallory"},       // unknown field
		{"key": "privileged:Deployment::" + strings.Repeat("n", maxKey), "reason": "r"}, // too long
	} {
		expect(t, e.do(http.MethodPost, "/accept", body), http.StatusBadRequest, plugin.CodeBadRequest)
	}
	if len(e.kv.m) != 0 {
		t.Fatalf("refused acceptances were stored: %v", e.kv.m)
	}
	// RBAC names may contain colons: the name is the rest of the key.
	expect(t, e.do(http.MethodPost, "/accept", map[string]any{"key": "cluster-admin-binding:ClusterRoleBinding::system:ci", "reason": "r"}), http.StatusOK, "")
}

func TestExport(t *testing.T) {
	e := newEnv(t, lab()...)
	e.scan(false)
	key := "secret-env:Deployment:shop:insecure"
	expect(t, e.do(http.MethodPost, "/accept", map[string]any{"key": key, "reason": "=HYPERLINK(\"x\") | piped"}), http.StatusOK, "")
	html := "privileged:Deployment:shop:insecure"
	expect(t, e.do(http.MethodPost, "/accept", map[string]any{"key": html, "reason": "<img src=x onerror=alert(1)> [fix](https://evil.example) `x` **y**"}), http.StatusOK, "")

	rec := e.do(http.MethodGet, "/export?format=md", nil)
	md := string(expect(t, rec, http.StatusOK, ""))
	if rec.Header().Get("Content-Type") != "text/markdown; charset=utf-8" ||
		rec.Header().Get("Content-Disposition") != `attachment; filename="security-lab-20261114-120000.md"` {
		t.Fatalf("headers = %v", rec.Header())
	}
	for _, want := range []string{
		"# Security posture: lab", "system namespaces excluded (kube-system)",
		"| critical | Privileged containers | Deployment shop/insecure |",
		`accepted by alice: =HYPERLINK\("x"\) \| piped`, "## Remediation", "- **Privileged containers** (privileged): ",
		`accepted by alice: &lt;img src=x onerror=alert\(1\)&gt; \[fix\]\(https://evil.example\) \` + "`x\\`" + ` \*\*y\*\*`,
	} {
		if !strings.Contains(md, want) {
			t.Errorf("markdown lacks %q:\n%s", want, md)
		}
	}
	for _, raw := range []string{"<img", "[fix]", "](https"} {
		if strings.Contains(md, raw) {
			t.Errorf("markdown passes %q through unescaped", raw)
		}
	}

	rec = e.do(http.MethodGet, "/export?format=csv", nil)
	rows, err := csv.NewReader(strings.NewReader(string(expect(t, rec, http.StatusOK, "")))).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	rep := decode[Report](t, expect(t, e.do(http.MethodGet, "/report", nil), http.StatusOK, ""))
	if len(rows) != len(rep.Findings)+1 || rows[0][0] != "severity" || rows[0][12] != "accepted_reason" {
		t.Fatalf("csv has %d rows for %d findings; header %v", len(rows), len(rep.Findings), rows[0])
	}
	idx := slices.IndexFunc(rows, func(r []string) bool { return r[1] == "secret-env" && r[7] == "insecure" })
	if idx < 0 || rows[idx][10] != "accepted" || rows[idx][12] != `'=HYPERLINK("x") | piped` {
		t.Fatalf("accepted row = %v", rows[idx])
	}

	expect(t, e.do(http.MethodGet, "/export?format=pdf", nil), http.StatusBadRequest, plugin.CodeBadRequest)
}

func TestChecksCatalog(t *testing.T) {
	e := newEnv(t)
	checks := decode[[]Check](t, expect(t, e.do(http.MethodGet, "/checks", nil), http.StatusOK, ""))
	if len(checks) != len(catalog) || checks[0].ID != "privileged" || checks[0].Remediation == "" || checks[0].Target != targetWorkload {
		t.Fatalf("checks = %+v", checks[0])
	}
}

func TestRouteAccess(t *testing.T) {
	access := map[string]plugin.Access{}
	New().Routes(routerFunc(func(method, path string, a plugin.Access, _ plugin.HandlerFunc) { access[method+" "+path] = a }))
	want := map[string]plugin.Access{
		"POST /scan": plugin.AccessRead, "GET /report": plugin.AccessRead, "GET /checks": plugin.AccessRead,
		"GET /export": plugin.AccessRead, "GET /accepted": plugin.AccessRead,
		"POST /accept": plugin.AccessWrite, "DELETE /accept/{key}": plugin.AccessWrite,
	}
	if len(access) != len(want) {
		t.Fatalf("routes = %v", access)
	}
	for route, a := range want {
		if access[route] != a {
			t.Errorf("%s: access %v, want %v", route, access[route], a)
		}
	}
}

type routerFunc func(method, path string, a plugin.Access, h plugin.HandlerFunc)

func (f routerFunc) Handle(method, path string, a plugin.Access, h plugin.HandlerFunc) {
	f(method, path, a, h)
}

// Bindings to everyone (anonymous, every user, every ServiceAccount) and
// to ServiceAccounts of user namespaces named as users or groups are in a
// default scan, with the ClusterRoles only they use; the bootstrap
// bindings that grant discovery to everyone stay out.
func TestDefaultScanKeepsBindingsToEveryone(t *testing.T) {
	bootstrap := map[string]string{"kubernetes.io/bootstrapping": "rbac-defaults"}
	all := rbacv1.PolicyRule{APIGroups: []string{"*"}, Resources: []string{"*"}, Verbs: []string{"*"}}
	group := func(name string) rbacv1.Subject { return rbacv1.Subject{Kind: rbacv1.GroupKind, Name: name} }
	user := func(name string) rbacv1.Subject { return rbacv1.Subject{Kind: rbacv1.UserKind, Name: name} }
	e := newEnv(t,
		clusterRole("cluster-admin", bootstrap, all),
		clusterRole("everything", nil, all),
		clusterRole("system:discovery", bootstrap, rbacv1.PolicyRule{NonResourceURLs: []string{"/api"}, Verbs: []string{"get"}}),
		clusterRoleBinding("anon-admin", "cluster-admin", nil, group("system:unauthenticated")),
		clusterRoleBinding("system:everyone-admin", "cluster-admin", nil, group("system:authenticated")),
		clusterRoleBinding("anonymous-admin", "cluster-admin", nil, user("system:anonymous")),
		clusterRoleBinding("ci-user-admin", "cluster-admin", nil, user("system:serviceaccount:shop:ci")),
		clusterRoleBinding("kube-system-sas-admin", "cluster-admin", nil, group("system:serviceaccounts:kube-system")),
		clusterRoleBinding("all-sas", "everything", nil, group("system:serviceaccounts")),
		clusterRoleBinding("system:discovery", "system:discovery", bootstrap, group("system:authenticated")),
		&rbacv1.RoleBinding{
			ObjectMeta: meta("kube-system", "anon-admin"),
			RoleRef:    rbacv1.RoleRef{Kind: "ClusterRole", Name: "cluster-admin"},
			Subjects:   []rbacv1.Subject{group("system:unauthenticated")},
		},
	)
	got := findingsOf(e.scan(false))
	for _, key := range []string{
		"cluster-admin-binding:ClusterRoleBinding::anon-admin",
		"cluster-admin-binding:ClusterRoleBinding::system:everyone-admin",
		"cluster-admin-binding:ClusterRoleBinding::anonymous-admin",
		"cluster-admin-binding:ClusterRoleBinding::ci-user-admin",
		"cluster-admin-binding:RoleBinding:kube-system:anon-admin",
		"rbac-wildcards:ClusterRole::everything",
	} {
		if _, ok := got[key]; !ok {
			t.Errorf("default scan misses %s", key)
		}
	}
	for key := range got {
		if strings.Contains(key, "kube-system-sas-admin") || strings.Contains(key, "system:discovery") {
			t.Errorf("system binding scanned by default: %s", key)
		}
	}
}
