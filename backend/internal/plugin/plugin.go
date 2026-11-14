// Package plugin is the contract between KubeBench and its built-in backend
// plugins (docs/design-v0.5.md §5).
//
// A plugin is a Go package under internal/plugins/<id> that calls Register
// from init(). The API server mounts its routes under /api/plugins/<id>/
// behind the same gates as core routes: session, password-change, role and
// read-only checks, auditing, the admin's enable/disable choice and the
// plugin's own Detect result for the caller's active cluster.
package plugin

import (
	"context"
	"net/http"
	"regexp"

	"kubebench/internal/kube"
	"kubebench/internal/store"
)

// Access is the permission a plugin route requires (§3.2).
type Access int

const (
	// AccessRead needs a session, an active cluster and any role on it.
	AccessRead Access = iota + 1
	// AccessWrite also needs the operator or admin role and a cluster
	// that is not read-only. Requests are audited.
	AccessWrite
	// AccessExec has the same gate as AccessWrite. Requests are audited.
	AccessExec
	// AccessAdmin needs a global admin. Mutating requests are audited.
	AccessAdmin
)

// DryRunnable may be combined with AccessWrite or AccessExec
// (AccessWrite|DryRunnable) for a route whose handler honours the
// dryRun=true query parameter. Such requests are checked as AccessRead and
// are not audited, so whenever IsDryRun(r) holds the handler must really
// perform a dry run (e.g. metav1.DryRunAll).
const DryRunnable Access = 1 << 8

// IsDryRun reports whether r asks for a dry run: exactly dryRun=true. The
// router uses it to relax DryRunnable routes; their handlers must use it
// too, so both always agree.
func IsDryRun(r *http.Request) bool { return r.URL.Query().Get("dryRun") == "true" }

// Level is the access level without the DryRunnable flag.
func (a Access) Level() Access { return a &^ DryRunnable }

// IsDryRunnable reports whether the DryRunnable flag is set.
func (a Access) IsDryRunnable() bool { return a&DryRunnable != 0 }

// Manifest describes a plugin in the Extensions view (GET /api/plugins).
type Manifest struct {
	ID, Name, Version, Description, Category string
}

// Status is a plugin's availability on one cluster.
type Status struct {
	Available bool   `json:"available"`
	Reason    string `json:"reason,omitempty"`
}

// Context is what a plugin handler gets for one request.
type Context struct {
	// Runtime is the caller's active cluster.
	Runtime *kube.Runtime
	User    *store.User
	// KV is the plugin's persistent key/value space (SQLite plugin_kv).
	// Per-cluster data should prefix keys with "cluster/<clusterID>/".
	KV KV
	// Audit appends an extra audit row, e.g. for an action whose target is
	// only known after the request body was read. The router already
	// records every write, exec and admin mutation.
	Audit func(action, target, detail string)
}

// KV is a plugin-scoped persistent key/value store.
type KV interface {
	Get(key string) (value string, ok bool, err error)
	Set(key, value string) error
	Delete(key string) error
	// List returns every entry whose key starts with prefix.
	List(prefix string) (map[string]string, error)
}

// HandlerFunc serves one plugin request.
type HandlerFunc func(w http.ResponseWriter, r *http.Request, c *Context)

// Router registers a plugin's routes.
type Router interface {
	// Handle registers e.g. ("GET", "/applications/{ns}/{name}", AccessRead, h),
	// mounted at /api/plugins/<id>/applications/{ns}/{name}. Paths use
	// net/http ServeMux wildcards and must not be "/" itself.
	Handle(method, path string, access Access, h HandlerFunc)
}

// Plugin is a built-in backend plugin.
type Plugin interface {
	Manifest() Manifest
	Routes(r Router)
	// Detect reports whether the plugin can work on rt's cluster, e.g.
	// whether the CRDs it needs are served. Results are cached per cluster
	// for a minute; unavailable plugins answer 503 plugin_unavailable.
	Detect(ctx context.Context, rt *kube.Runtime) Status
}

var idPattern = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,39}$`)

// ValidID reports whether id can name a plugin (built-in, external or
// frontend-only): lower-case letters, digits and dashes, at most 40.
func ValidID(id string) bool { return idPattern.MatchString(id) }
