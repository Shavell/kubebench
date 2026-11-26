package api

import "kubebench/internal/svcproxy"

// routes is the API's route table (docs/design-v0.5.md §3.2, §4). The
// access level decides session, role, read-only and audit handling.
func (s *Server) routes() {
	// Routes that work on a cluster run under the request timeout
	// (KUBEBENCH_REQUEST_TIMEOUT) unless marked longRunning: streams and
	// terminals, and operations that bound their own duration.

	// No session.
	s.handle("GET /healthz", accessPublic, s.handleHealthz)
	s.handle("GET /api/version", accessPublic, s.handleVersion)
	s.handle("POST /api/auth/login", accessPublic, s.handleLogin)
	// External identities (§3.6): sign-in methods and the OIDC flow.
	s.handle("GET /api/auth/methods", accessPublic, s.handleAuthMethods)
	s.handle("GET "+oidcLoginPath, accessPublic, s.handleOIDCLogin)
	s.handle("GET "+oidcCallbackPath, accessPublic, s.handleOIDCCallback)

	// Any session, including one that must change its password first.
	s.handle("POST /api/auth/logout", accessAuthed, s.handleLogout)
	s.handle("GET /api/auth/me", accessAuthed, s.handleMe)
	s.handle("POST /api/auth/password", accessAuthed, s.handleChangePassword)
	s.handle("GET /api/info", accessAuthed, s.handleInfo)
	s.handle("GET /api/clusters", accessAuthed, s.handleListClusters)
	s.handle("POST /api/clusters/activate", accessAuthed, s.handleActivateCluster)
	s.handle("GET /api/plugins", accessAuthed, s.handleListPlugins)
	s.handle("GET /plugins/{id}/{file...}", accessAuthed, s.handlePluginFile)
	s.handle("PATCH /api/auth/me", accessAuthed, s.handleUpdateMe)
	s.handle("GET /api/updates", accessAuthed, s.handleUpdates)

	// Global admins: users, the cluster inventory, plugins, the audit log.
	s.handle("GET /api/users", accessAdmin, s.handleListUsers)
	s.handle("POST /api/users", accessAdmin, s.handleCreateUser)
	s.handle("PATCH /api/users/{id}", accessAdmin, s.handleUpdateUser)
	s.handle("DELETE /api/users/{id}", accessAdmin, s.handleDeleteUser)
	s.handle("PUT /api/users/{id}/clusters", accessAdmin, s.handleSetAssignments)
	s.handle("POST /api/clusters", accessAdmin, s.handleAddCluster)
	s.handle("PATCH /api/clusters/{id}", accessAdmin, s.handleUpdateCluster)
	s.handle("DELETE /api/clusters", accessAdmin, s.handleDeleteCluster)
	s.handle("PUT /api/plugins/{id}", accessAdmin, s.handleSetPluginEnabled)
	s.handle("GET /api/audit", accessAdmin, s.handleListAudit)
	s.handle("PUT /api/settings/updates", accessAdmin, s.handleSetUpdateMode)
	s.handle("POST /api/updates/check", accessAdmin, s.handleCheckUpdates)
	// Settings > Authentication (§3.6).
	s.handle("GET /api/settings/auth", accessAdmin, s.handleAuthSettings)
	s.handle("GET /api/settings/auth/mappings", accessAdmin, s.handleListMappings)
	s.handle("POST /api/settings/auth/mappings", accessAdmin, s.handleCreateMapping)
	s.handle("PUT /api/settings/auth/mappings/{id}", accessAdmin, s.handleUpdateMapping)
	s.handle("DELETE /api/settings/auth/mappings/{id}", accessAdmin, s.handleDeleteMapping)
	s.handle("POST /api/settings/auth/preview", accessAdmin, s.handlePreviewSignIn)

	// The active cluster: reads.
	s.handle("GET /api/nodes", accessRead, s.handleNodes)
	s.handle("GET /api/namespaces", accessRead, s.handleNamespaces)
	s.handle("GET /api/pods", accessRead, s.handlePods)
	s.handle("GET /api/deployments", accessRead, s.handleDeployments)
	s.handle("GET /api/services", accessRead, s.handleServices)
	s.handle("GET /api/yaml", accessRead, s.handleYAML)
	s.handle("GET /api/events", accessRead, s.handleEvents)
	s.handle("GET /api/logs", accessRead, s.handleLogs)
	s.handle("GET /api/logs/stream", accessRead|longRunning, s.handleLogStream)
	s.handle("GET /api/resource", accessRead, s.handleResourceList)
	s.handle("GET /api/resource/get", accessRead, s.handleResourceGet)
	s.handle("GET /api/k8s/{path...}", accessRead|longRunning, s.handlePassthrough)
	s.handle("GET /api/metrics/sources", accessRead, s.handleMetricSources)
	s.handle("GET /api/metrics/overview", accessRead, s.handleOverview)
	s.handle("GET /api/metrics/allocation", accessRead, s.handleAllocation)
	s.handle("GET /api/metrics/cluster", accessRead, s.handleClusterSeries)
	s.handle("GET /api/metrics/node", accessRead, s.handleNodeSeries)
	s.handle("GET /api/metrics/pod", accessRead, s.handlePodSeries)
	s.handle("GET /api/crds", accessRead, s.handleCrds)
	s.handle("GET /api/crds/instances", accessRead, s.handleCrdInstances)
	// Live updates (§7.10): change notifications and the informers behind them.
	s.handle("GET /api/watch", accessRead|longRunning, s.handleWatch)
	s.handle("GET /api/watch/status", accessRead, s.handleWatchStatus)

	// The active cluster: changes (audited).
	s.handle("DELETE /api/pods", accessWrite, s.handleDeletePod)
	s.handle("POST /api/deployments/scale", accessWrite, s.handleScale)
	s.handle("POST /api/deployments/restart", accessWrite, s.handleRestart)
	s.handle("POST /api/exec", accessExec|longRunning, s.handleExec)
	s.handle("GET /api/exec/ws", accessExec|longRunning, s.handleTerminal)
	s.handle("POST /api/resource/apply", accessWrite|allowDryRun, s.handleApply)
	s.handle("POST /api/resource/edit", accessWrite|allowDryRun, s.handleEdit)
	s.handle("DELETE /api/resource", accessWrite, s.handleDeleteResource)
	s.handle("PATCH /api/resource/metadata", accessWrite, s.handlePatchMetadata)
	// Changes nothing but shows the values every other route redacts:
	// exec access, audited like every exec request.
	s.handle("POST /api/secrets/reveal", accessExec, s.handleRevealSecret)

	// Workload and node operations (§7.3); every change is audited.
	s.handle("GET /api/deployments/history", accessRead, s.handleDeploymentHistory)
	s.handle("POST /api/deployments/rollback", accessWrite, s.handleRollback)
	s.handle("POST /api/deployments/pause", accessWrite, s.handlePause)
	s.handle("POST /api/workloads/restart", accessWrite, s.handleRestartWorkload)
	s.handle("POST /api/workloads/scale", accessWrite, s.handleScaleWorkload)
	s.handle("POST /api/nodes/cordon", accessWrite, s.handleCordon)
	s.handle("POST /api/nodes/drain", accessWrite|longRunning, s.handleDrain)
	s.handle("PUT /api/nodes/taints", accessWrite, s.handleSetTaints)
	s.handle("POST /api/pods/bulk-delete", accessWrite, s.handleBulkDeletePods)
	s.handle("POST /api/pods/debug", accessExec|longRunning, s.handleDebugPod)
	s.handle("POST /api/cronjobs/trigger", accessWrite, s.handleTriggerCronJob)
	s.handle("POST /api/cronjobs/suspend", accessWrite, s.handleSuspendCronJob)
	s.handle("POST /api/namespaces", accessWrite, s.handleCreateNamespace)
	s.handle("DELETE /api/namespaces", accessWrite, s.handleDeleteNamespace)

	// Service browser (§7.14): opening a grant is audited once; proxied
	// requests authorize with the grant's token in their URL.
	s.handle("POST /api/proxy/services/{ns}/{name}/{port}", accessBrowse, s.handleOpenProxy(svcproxy.KindService))
	s.handle("POST /api/proxy/pods/{ns}/{name}/{port}", accessBrowse, s.handleOpenProxy(svcproxy.KindPod))
	s.handle("POST /api/proxy/ingresses/{ns}/{name}", accessBrowse, s.handleOpenIngressProxy)
	s.handle("POST /api/proxy/routes/{ns}/{name}", accessBrowse, s.handleOpenRouteProxy) // OpenShift (§7.15)
	s.handle("GET /api/proxy/sessions/{token}", accessAuthed, s.handleProxySession)
	s.handle("DELETE /api/proxy/sessions/{token}", accessAuthed, s.handleCloseProxySession)
	s.handle("GET /proxy/{token}", accessPublic, s.handleProxyRoot)
	for _, method := range []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"} {
		s.handle(method+" /proxy/{token}/{path...}", accessPublic, s.handleProxy)
	}

	s.mountPlugins()
	s.mux.HandleFunc("/api/", s.handleUnknownAPI)
}

// auditKinds names the audit target kind of routes whose path does not
// (see targetKind). Declared here rather than in the handlers so that
// denied requests, which never reach a handler, get the same target.
var auditKinds = map[string]string{
	"POST /api/exec":   "pod",
	"GET /api/exec/ws": "pod",
	// Helm plugin (§7.4): the release is named by the body.
	"POST /api/plugins/helm/install": "release",
	// Security plugin (§7.7): accepted risks are named by their finding key.
	"POST /api/plugins/security/accept":         "finding",
	"DELETE /api/plugins/security/accept/{key}": "finding",
	// Service browser (§7.14).
	"POST /api/proxy/services/{ns}/{name}/{port}": "service",
	"POST /api/proxy/pods/{ns}/{name}/{port}":     "pod",
	"POST /api/proxy/ingresses/{ns}/{name}":       "ingress",
	"POST /api/proxy/routes/{ns}/{name}":          "route",
	// Group mappings (§3.6).
	"POST /api/settings/auth/mappings":        "mapping",
	"PUT /api/settings/auth/mappings/{id}":    "mapping",
	"DELETE /api/settings/auth/mappings/{id}": "mapping",
	"POST /api/settings/auth/preview":         "user",
}

// auditActions names the audit action of routes that record a named action
// instead of "<METHOD> <route pattern>" (§3.4), e.g. a WebSocket session,
// whose row is written once, when the connection is upgraded.
var auditActions = map[string]string{
	"GET /api/exec/ws": "exec.session",
	// A browsing session, not each request it makes (§7.14).
	"POST /api/proxy/services/{ns}/{name}/{port}": "service.proxy",
	"POST /api/proxy/pods/{ns}/{name}/{port}":     "pod.proxy",
	"POST /api/proxy/ingresses/{ns}/{name}":       "ingress.proxy",
	"POST /api/proxy/routes/{ns}/{name}":          "route.proxy",
}
