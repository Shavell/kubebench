// Package helm is the built-in Helm plugin (docs/design-v0.5.md §7.4):
// releases (list, detail, values and manifest per revision, install,
// upgrade, rollback, uninstall), chart repositories, chart search and
// chart uploads for offline installs, through the Helm v3 SDK.
//
// Releases live in the cluster (Secrets, like the helm CLI keeps them);
// repositories, caches and uploads under $KUBEBENCH_DATA_DIR/helm, shared
// by every cluster. Routes are served under /api/plugins/helm/.
package helm

import (
	"context"
	"errors"
	"net/http"

	"helm.sh/helm/v3/pkg/action"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

func init() { plugin.Register(New()) }

// Plugin implements plugin.Plugin and plugin.Starter.
type Plugin struct {
	// env is set by Start, before the server accepts requests.
	env *environment
	// actions builds the Helm action configuration for a namespace of a
	// cluster ("" = all namespaces); tests replace it with in-memory
	// release storage.
	actions func(rt *kube.Runtime, namespace string) (*action.Configuration, error)
}

// New returns the plugin; it serves once Start has prepared its data
// directory.
func New() *Plugin {
	p := &Plugin{}
	p.actions = func(rt *kube.Runtime, namespace string) (*action.Configuration, error) {
		return p.env.newActionConfig(rt, namespace)
	}
	return p
}

func (p *Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          "helm",
		Name:        "Helm",
		Version:     "0.5.0",
		Description: "Helm releases, repositories and charts",
		Category:    "tools",
	}
}

// Start prepares $KUBEBENCH_DATA_DIR/helm and removes expired uploads
// until ctx ends.
func (p *Plugin) Start(ctx context.Context, host plugin.Host) error {
	env, err := newEnvironment(host.DataDir)
	if err != nil {
		return err
	}
	p.env = env
	go env.sweepUploads(ctx)
	return nil
}

// Detect needs the release Secrets: Helm keeps every release in Secrets
// labelled owner=helm, so listing them in all namespaces covers both
// reading releases and RBAC. The router runs it after the runtime's
// capability probe and caches it for a minute.
func (p *Plugin) Detect(ctx context.Context, rt *kube.Runtime) plugin.Status {
	_, err := rt.CS.CoreV1().Secrets("").List(ctx, metav1.ListOptions{LabelSelector: "owner=helm", Limit: 1})
	if err != nil {
		return plugin.Status{Reason: "Helm is not available on this cluster: cannot list the release Secrets (" + err.Error() + ")"}
	}
	return plugin.Status{Available: true}
}

var errNotStarted = errors.New("the Helm plugin was not started")

// Routes registers §7.4's endpoints. Dry runs of install and upgrade need
// only read access (DryRunnable); repositories are added and removed by
// admins, while updating their indexes is a write.
func (p *Plugin) Routes(r plugin.Router) {
	handle := func(method, path string, access plugin.Access, h plugin.HandlerFunc) {
		r.Handle(method, path, access, func(w http.ResponseWriter, req *http.Request, c *plugin.Context) {
			if p.env == nil {
				plugin.WriteError(w, http.StatusServiceUnavailable, plugin.CodeInternal, errNotStarted)
				return
			}
			h(w, req, c)
		})
	}
	handle(http.MethodGet, "/releases", plugin.AccessRead, p.handleListReleases)
	handle(http.MethodGet, "/releases/{ns}/{name}", plugin.AccessRead, p.handleGetRelease)
	handle(http.MethodGet, "/releases/{ns}/{name}/values", plugin.AccessRead, p.handleReleaseValues)
	handle(http.MethodGet, "/releases/{ns}/{name}/manifest", plugin.AccessRead, p.handleReleaseManifest)
	handle(http.MethodPost, "/releases/{ns}/{name}/rollback", plugin.AccessWrite, p.handleRollback)
	handle(http.MethodDelete, "/releases/{ns}/{name}", plugin.AccessWrite, p.handleUninstall)
	handle(http.MethodPost, "/releases/{ns}/{name}/upgrade", plugin.AccessWrite|plugin.DryRunnable, p.handleUpgrade)
	handle(http.MethodPost, "/install", plugin.AccessWrite|plugin.DryRunnable, p.handleInstall)

	handle(http.MethodGet, "/repos", plugin.AccessRead, p.handleListRepos)
	handle(http.MethodPost, "/repos", plugin.AccessAdmin, p.handleAddRepo)
	handle(http.MethodDelete, "/repos/{name}", plugin.AccessAdmin, p.handleRemoveRepo)
	handle(http.MethodPost, "/repos/update", plugin.AccessWrite, p.handleUpdateRepos)

	handle(http.MethodGet, "/charts/search", plugin.AccessRead, p.handleSearchCharts)
	handle(http.MethodGet, "/charts/values", plugin.AccessRead, p.handleChartValues)
	handle(http.MethodPost, "/charts/upload", plugin.AccessWrite, p.handleUploadChart)
}
