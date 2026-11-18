package argocd

import "kubebench/internal/reason"

// Reasons of the Argo CD plugin's own refusals (docs/design-v0.5.md §3.3).
var (
	reasonNotInstalled     = reason.Define("argocd.not_installed", "Argo CD is not installed: the cluster does not serve {{groupVersion}}")
	reasonNoApplications   = reason.Define("argocd.no_applications", "Argo CD is not installed: the cluster does not serve {{groupVersion}} applications")
	reasonDiscovery        = reason.Define("argocd.discovery_failed", "cannot discover Argo CD: {{detail}}")
	reasonOperationRunning = reason.Define("argocd.operation_running", "another operation is already running on application {{namespace}}/{{name}}; wait for it to finish")
	reasonOrphanDeleting   = reason.Define("argocd.deleting_without_cascade", "application {{namespace}}/{{name}} is already being deleted without cascade")
	reasonMultiSource      = reason.Define("argocd.multi_source_revision", "a revision cannot be chosen for an application with several sources")
	reasonResourceName     = reason.Define("argocd.resource_kind_name", "every selected resource needs a kind and a name")
	reasonAutoSync         = reason.Define("argocd.rollback_auto_sync", "rollback is not possible while automated sync is enabled; disable it first")
	reasonNoHistoryEntry   = reason.Define("argocd.no_history_entry", "the application has no history entry {{id}}")
	reasonHistorySource    = reason.Define("argocd.history_without_source", "history entry {{id}} does not record its source (deployed by Argo CD 1.4 or older)")
	reasonSeveralValues    = reason.Define("argocd.several_json_values", "invalid request body: more than one JSON value")
)
