// Package access is the built-in "access" backend plugin
// (docs/design-v0.5.md §7.8): an RBAC explorer. It answers who may do
// what from the cluster's Roles, ClusterRoles and their bindings — who
// can perform a request and through which binding, a subject's effective
// rules by namespace, risky roles and the subject → binding → role graph —
// and asks the API server itself through SubjectAccessReviews.
//
// The evaluation follows the Kubernetes RBAC authorizer: wildcards in
// verbs, API groups, resources ("*" and "*/subresource") and
// non-resource URLs (a trailing "*"), resourceNames, RoleBindings that
// grant a ClusterRole's rules in their namespace only, and ServiceAccount
// group memberships. Aggregated ClusterRoles are used with the rules the
// aggregation controller filled in. Every route reads; nothing is changed.
package access

import (
	"context"
	"net/http"
	"time"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// ID is the plugin id; its routes live under /api/plugins/access/.
const ID = "access"

func init() { plugin.Register(New()) }

// Plugin implements plugin.Plugin; create it with New.
type Plugin struct {
	limiter *reviewLimiter
}

// New returns the plugin with its own review rate limiter.
func New() *Plugin { return newPlugin(time.Now) }

func newPlugin(now func() time.Time) *Plugin {
	return &Plugin{limiter: newReviewLimiter(now)}
}

func (*Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          ID,
		Name:        "Access",
		Version:     "0.5.0",
		Description: "RBAC explorer: who can, effective permissions, risky roles and the binding graph",
		Category:    "security",
	}
}

func (p *Plugin) Routes(r plugin.Router) {
	r.Handle(http.MethodGet, "/subjects", plugin.AccessRead, listSubjects)
	r.Handle(http.MethodGet, "/subjects/{kind}/{name}", plugin.AccessRead, getSubject)
	r.Handle(http.MethodGet, "/who-can", plugin.AccessRead, whoCan)
	r.Handle(http.MethodGet, "/risky", plugin.AccessRead, listRisky)
	r.Handle(http.MethodGet, "/graph", plugin.AccessRead, graph)
	// Read: a SubjectAccessReview changes nothing; it is rate-limited instead.
	r.Handle(http.MethodPost, "/review", plugin.AccessRead, p.review)
}

// Detect: RBAC is part of every supported cluster, so the plugin is always
// available; a credential that may not list RBAC objects gets the API
// server's 403 per request.
func (*Plugin) Detect(context.Context, *kube.Runtime) plugin.Status {
	return plugin.Status{Available: true}
}
