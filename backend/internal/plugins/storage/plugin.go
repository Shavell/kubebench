// Package storage is the built-in "storage" backend plugin
// (docs/design-v0.5.md §7.6): an overview of the cluster's storage, CSI
// drivers and volume snapshots, and creators for PersistentVolumeClaims,
// PersistentVolumes and StorageClasses that validate their input before
// anything reaches the cluster, plus PVC expansion.
//
// Create requests take a small form-shaped body rather than a manifest:
// the server builds the object, so it can check names, quantities, access
// modes and references (the PV to bind, the PVC or snapshot to clone) and
// answer with messages that name the field at fault. Each create route
// honours dryRun=true with a real server-side dry run.
package storage

import (
	"context"
	"net/http"

	"kubebench/internal/kube"
	"kubebench/internal/plugin"
)

// ID is the plugin id; its routes live under /api/plugins/storage/.
const ID = "storage"

func init() { plugin.Register(&Plugin{}) }

// Plugin implements plugin.Plugin.
type Plugin struct{}

func (*Plugin) Manifest() plugin.Manifest {
	return plugin.Manifest{
		ID:          ID,
		Name:        "Storage",
		Version:     "0.5.0",
		Description: "PersistentVolumes, claims and StorageClasses: overview, creators and expansion",
		Category:    "storage",
	}
}

func (*Plugin) Routes(r plugin.Router) {
	r.Handle(http.MethodGet, "/overview", plugin.AccessRead, overview)
	r.Handle(http.MethodGet, "/pvcs", plugin.AccessRead, listPVCs)
	r.Handle(http.MethodGet, "/pvs", plugin.AccessRead, listPVs)
	r.Handle(http.MethodGet, "/csidrivers", plugin.AccessRead, listCSIDrivers)
	r.Handle(http.MethodGet, "/snapshots", plugin.AccessRead, listSnapshots)
	r.Handle(http.MethodPost, "/pvcs", plugin.AccessWrite|plugin.DryRunnable, createPVC)
	r.Handle(http.MethodPost, "/pvs", plugin.AccessWrite|plugin.DryRunnable, createPV)
	r.Handle(http.MethodPost, "/storageclasses", plugin.AccessWrite|plugin.DryRunnable, createStorageClass)
	r.Handle(http.MethodPatch, "/pvcs/{ns}/{name}/resize", plugin.AccessWrite, resizePVC)
}

// Detect: every cluster serves the core storage API, so the plugin is
// always available. Snapshots, which need CRDs, are reported per request.
func (*Plugin) Detect(context.Context, *kube.Runtime) plugin.Status {
	return plugin.Status{Available: true}
}
