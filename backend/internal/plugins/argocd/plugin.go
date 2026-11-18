// Package argocd is the built-in Argo CD plugin (docs/design-v0.5.md §7.5).
//
// It works through the Argo CD custom resources only (argoproj.io/v1alpha1
// Applications, AppProjects and ApplicationSets), with the credential of
// the caller's active cluster, so no Argo CD API token is needed. Syncs,
// rollbacks and refreshes are requested the way the Argo CD API server
// requests them: by setting the Application's operation field or the
// refresh annotation, which the application controller then acts on.
package argocd

import (
	"context"
	"net/http"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
	"kubebench/internal/reason"
)

// ID is the plugin id: its routes are served under /api/plugins/argocd/.
const ID = "argocd"

// version is the plugin's own version, shown in the Extensions view.
const version = "0.5.0"

const groupVersion = "argoproj.io/v1alpha1"

var (
	applicationsGVR    = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applications"}
	projectsGVR        = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "appprojects"}
	applicationSetsGVR = schema.GroupVersionResource{Group: "argoproj.io", Version: "v1alpha1", Resource: "applicationsets"}
)

func init() { plugin.Register(&Plugin{}) }

// Plugin implements plugin.Plugin.
type Plugin struct{}

func (*Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          ID,
		Name:        "Argo CD",
		Version:     version,
		Description: "Argo CD applications, projects and ApplicationSets: status, sync, rollback, refresh and delete.",
		Category:    "gitops",
	}
}

// Detect reports the plugin available when the cluster serves Argo CD
// Applications. It asks discovery directly rather than the runtime's
// cached copy, so installing Argo CD is noticed once the cached result
// expires.
func (*Plugin) Detect(_ context.Context, rt *kube.Runtime) plugin.Status {
	list, err := rt.CS.Discovery().ServerResourcesForGroupVersion(groupVersion)
	switch {
	case apierrors.IsNotFound(err):
		return plugin.Unavailable(reasonNotInstalled, reason.Params{"groupVersion": groupVersion})
	case err != nil:
		return plugin.Unavailable(reasonDiscovery, reason.Params{"detail": err.Error()})
	}
	for _, r := range list.APIResources {
		if r.Name == applicationsGVR.Resource {
			return plugin.Status{Available: true}
		}
	}
	return plugin.Unavailable(reasonNoApplications, reason.Params{"groupVersion": groupVersion})
}

// Routes registers the plugin API. Every mutation is a write route, so a
// viewer or a read-only cluster is refused by the router before any
// Kubernetes call. Argo CD's own dry-run sync still patches the
// Application, so the sync route is not DryRunnable.
func (*Plugin) Routes(r plugin.Router) {
	r.Handle(http.MethodGet, "/applications", plugin.AccessRead, listApplications)
	r.Handle(http.MethodGet, "/applications/{ns}/{name}", plugin.AccessRead, getApplication)
	r.Handle(http.MethodPost, "/applications/{ns}/{name}/sync", plugin.AccessWrite, syncApplication)
	r.Handle(http.MethodPost, "/applications/{ns}/{name}/refresh", plugin.AccessWrite, refreshApplication)
	r.Handle(http.MethodPost, "/applications/{ns}/{name}/rollback", plugin.AccessWrite, rollbackApplication)
	r.Handle(http.MethodDelete, "/applications/{ns}/{name}", plugin.AccessWrite, deleteApplication)
	r.Handle(http.MethodGet, "/projects", plugin.AccessRead, listProjects)
	r.Handle(http.MethodGet, "/applicationsets", plugin.AccessRead, listApplicationSets)
}
