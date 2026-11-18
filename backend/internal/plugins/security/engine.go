package security

import (
	"fmt"
	"slices"
	"strings"
	"time"

	corev1 "k8s.io/api/core/v1"

	"kubebench/internal/reason"
)

// maxFindings bounds a report; a cluster with more findings gets the first
// ones and a warning.
const maxFindings = 20000

// scan is the input of one run of the checks: the subjects in scope plus
// indexes some checks consult.
type scan struct {
	opts       ScanOptions
	workloads  []*workload
	namespaces []*corev1.Namespace
	// accounts are the namespaces' default ServiceAccounts.
	accounts []*corev1.ServiceAccount
	roles    []*roleInfo
	// bindings are those in scope; allBindings includes the cluster's own.
	bindings    []*bindingInfo
	allBindings []*bindingInfo
	// automount is each ServiceAccount's automountServiceAccountToken, by
	// "<namespace>/<name>".
	automount map[string]*bool
	// policies counts the NetworkPolicies of each namespace.
	policies map[string]int

	// unavailable says, per target, why its checks cannot run.
	unavailable map[string]*reason.Error
	warnings    []*reason.Error
	inventory   map[string]int
}

// newScan prepares the subjects of inv for opts.
func newScan(inv *inventory, opts ScanOptions) *scan {
	inScope := func(ns string) bool { return opts.IncludeSystem || !isSystemNamespace(ns) }
	s := &scan{
		opts: opts, automount: map[string]*bool{}, policies: map[string]int{},
		unavailable: map[string]*reason.Error{}, inventory: inv.counts(),
	}
	s.workloads = workloads(inv, inScope)
	for i := range inv.namespaces {
		if ns := &inv.namespaces[i]; inScope(ns.Name) {
			s.namespaces = append(s.namespaces, ns)
		}
	}
	for i := range inv.serviceAccounts {
		sa := &inv.serviceAccounts[i]
		s.automount[sa.Namespace+"/"+sa.Name] = sa.AutomountServiceAccountToken
		if sa.Name == "default" && inScope(sa.Namespace) {
			s.accounts = append(s.accounts, sa)
		}
	}
	for _, p := range inv.networkPolicies {
		s.policies[p.Namespace]++
	}
	s.allBindings = bindings(inv)
	for _, b := range s.allBindings {
		if opts.IncludeSystem || !b.system {
			s.bindings = append(s.bindings, b)
		}
	}
	s.roles = roles(inv, s.allBindings, opts.IncludeSystem)
	s.availability(inv)
	return s
}

// availability decides which targets can be checked and what the report
// should warn about.
func (s *scan) availability(inv *inventory) {
	if !inv.anyListed(workloadKinds...) {
		s.unavailable[targetWorkload] = inv.failure(workloadKinds...)
	}
	if !inv.listed(kindNamespaces) {
		s.unavailable[targetNamespace] = inv.failure(kindNamespaces)
	}
	if !inv.listed(kindServiceAccounts) {
		s.unavailable[targetServiceAccount] = inv.failure(kindServiceAccounts)
	}
	if !inv.anyListed(kindRoles, kindClusterRoles) {
		s.unavailable[targetRole] = inv.failure(kindRoles, kindClusterRoles)
	}
	if !inv.anyListed(kindRoleBindings, kindClusterRoleBindings) {
		s.unavailable[targetBinding] = inv.failure(kindRoleBindings, kindClusterRoleBindings)
	}
	for _, kind := range allKinds {
		if err := inv.errs[kind]; err != nil {
			s.warnings = append(s.warnings, err)
		}
		if inv.truncated[kind] {
			s.warnings = append(s.warnings, reason.New(reasonTruncatedKind, reason.Params{"max": maxPerKind, "kind": kind}))
		}
	}
}

// result is what a scan produced, before accepted risks are applied: the
// report is computed from it on every read, so accepting or expiring a
// risk changes the score without another scan.
type result struct {
	generatedAt time.Time
	cluster     string
	opts        ScanOptions
	findings    []Finding
	// evaluated counts the objects each check that ran looked at.
	evaluated map[string]int
	skipped   []Skipped
	warnings  []*reason.Error
	inventory map[string]int
}

// run evaluates every check of the catalog on s.
func (s *scan) run(checks []*Check) *result {
	res := &result{opts: s.opts, evaluated: map[string]int{}, warnings: s.warnings, inventory: s.inventory, findings: []Finding{}}
	truncated := false
	report := func(c *Check, r Resource, evidence []string) {
		if len(res.findings) >= maxFindings {
			truncated = true
			return
		}
		res.findings = append(res.findings, newFinding(c, r, evidence))
	}
	for _, c := range checks {
		if why, ok := s.unavailable[c.Target]; ok {
			res.skipped = append(res.skipped, Skipped{CheckID: c.ID, Reason: why.Error(), ReasonCode: why.Code.String(), ReasonParams: why.Params})
			continue
		}
		n := 0
		switch c.Target {
		case targetWorkload:
			for _, w := range s.workloads {
				if c.applies != nil && !c.applies(w) {
					continue
				}
				n++
				if ev, failed := c.workload(s, w); failed {
					report(c, w.Resource, ev)
				}
			}
		case targetNamespace:
			for _, ns := range s.namespaces {
				n++
				if ev, failed := c.namespace(s, ns); failed {
					report(c, Resource{APIVersion: "v1", Kind: "Namespace", Name: ns.Name}, ev)
				}
			}
		case targetServiceAccount:
			for _, sa := range s.accounts {
				n++
				if ev, failed := c.account(s, sa); failed {
					report(c, Resource{APIVersion: "v1", Kind: "ServiceAccount", Namespace: sa.Namespace, Name: sa.Name}, ev)
				}
			}
		case targetRole:
			for _, r := range s.roles {
				n++
				if ev, failed := c.role(s, r); failed {
					report(c, r.Resource, ev)
				}
			}
		case targetBinding:
			for _, b := range s.bindings {
				n++
				if ev, failed := c.binding(s, b); failed {
					report(c, b.Resource, ev)
				}
			}
		}
		res.evaluated[c.ID] = n
	}
	if truncated {
		res.warnings = append(res.warnings, reason.New(reasonTruncatedFindings, reason.Params{"max": maxFindings}))
	}
	slices.SortStableFunc(res.findings, compareFindings)
	return res
}

// findingKey identifies a check's finding on one object across scans:
// "<checkId>:<Kind>:<namespace>:<name>". Check ids, kinds and namespaces
// never contain a colon, so the name (RBAC names may) comes last.
func findingKey(checkID string, r Resource) string {
	return checkID + ":" + r.Kind + ":" + r.Namespace + ":" + r.Name
}

func newFinding(c *Check, r Resource, evidence []string) Finding {
	msg := c.message
	if strings.Contains(msg, "%s") {
		msg = fmt.Sprintf(msg, strings.Join(evidence, ", "))
	}
	return Finding{
		Key: findingKey(c.ID, r), CheckID: c.ID, Title: c.Title,
		Severity: c.Severity, Category: c.Category, Frameworks: c.Frameworks,
		Resource: r, Message: msg, Evidence: evidence, Remediation: c.Remediation,
	}
}

// compareFindings orders by severity, then check, namespace, kind and name.
func compareFindings(a, b Finding) int {
	if d := slices.Index(severities, a.Severity) - slices.Index(severities, b.Severity); d != 0 {
		return d
	}
	return strings.Compare(
		a.CheckID+"\x00"+a.Resource.Namespace+"\x00"+a.Resource.Kind+"\x00"+a.Resource.Name,
		b.CheckID+"\x00"+b.Resource.Namespace+"\x00"+b.Resource.Kind+"\x00"+b.Resource.Name,
	)
}
