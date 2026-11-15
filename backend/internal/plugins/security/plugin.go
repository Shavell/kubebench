// Package security is the built-in "security" backend plugin
// (docs/design-v0.5.md §7.7): a posture scan of the active cluster against
// the Pod Security Standards, workload hygiene rules and the CIS
// Kubernetes Benchmark's RBAC, network and secrets controls.
//
// A scan lists the pods, workload controllers, namespaces, ServiceAccounts,
// RBAC objects and NetworkPolicies once, in pages and up to a limit per
// kind, and runs every check of the catalog on them. The last result is
// kept in memory per cluster; reports are computed from it on each read,
// with the accepted risks (plugin KV, shared by the cluster's users)
// applied, so accepting a risk changes the score without another scan.
package security

import (
	"context"
	"net/http"
	"time"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// ID is the plugin id; its routes live under /api/plugins/security/.
const ID = "security"

func init() { plugin.Register(New()) }

// Plugin implements plugin.Plugin.
type Plugin struct {
	scans *scanner
	// now is the clock, replaced in tests.
	now func() time.Time
}

// New returns the plugin with an empty report cache.
func New() *Plugin {
	p := &Plugin{now: time.Now}
	p.scans = newScanner(p.clock)
	return p
}

func (p *Plugin) clock() time.Time { return p.now() }

func (*Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          ID,
		Name:        "Security",
		Version:     "0.5.0",
		Description: "Security posture: Pod Security Standards, RBAC, network and secrets checks with accepted risks",
		Category:    "security",
	}
}

func (p *Plugin) Routes(r plugin.Router) {
	// A scan only reads the cluster: read access, not audited.
	r.Handle(http.MethodPost, "/scan", plugin.AccessRead, p.scan)
	r.Handle(http.MethodGet, "/report", plugin.AccessRead, p.report)
	r.Handle(http.MethodGet, "/checks", plugin.AccessRead, listChecks)
	r.Handle(http.MethodGet, "/export", plugin.AccessRead, p.export)
	r.Handle(http.MethodGet, "/accepted", plugin.AccessRead, p.listAccepted)
	r.Handle(http.MethodPost, "/accept", plugin.AccessWrite, p.accept)
	r.Handle(http.MethodDelete, "/accept/{key}", plugin.AccessWrite, p.unaccept)
}

// Detect: the scan reads core APIs every cluster serves, and reports the
// kinds the credential may not list as skipped checks.
func (*Plugin) Detect(context.Context, *kube.Runtime) plugin.Status {
	return plugin.Status{Available: true}
}
