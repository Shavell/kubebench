package api

// routes is the API's route table (docs/design-v0.5.md §3.2, §4). The
// access level decides session, role, read-only and audit handling.
func (s *Server) routes() {
	// No session.
	s.handle("GET /healthz", accessPublic, s.handleHealthz)
	s.handle("GET /api/version", accessPublic, s.handleVersion)
	s.handle("POST /api/auth/login", accessPublic, s.handleLogin)

	// Any session, including one that must change its password first.
	s.handle("POST /api/auth/logout", accessAuthed, s.handleLogout)
	s.handle("GET /api/auth/me", accessAuthed, s.handleMe)
	s.handle("POST /api/auth/password", accessAuthed, s.handleChangePassword)
	s.handle("GET /api/info", accessAuthed, s.handleInfo)
	s.handle("GET /api/clusters", accessAuthed, s.handleListClusters)
	s.handle("POST /api/clusters/activate", accessAuthed, s.handleActivateCluster)
	s.handle("GET /api/plugins", accessAuthed, s.handleListPlugins)
	s.handle("GET /plugins/{id}/{file...}", accessAuthed, s.handlePluginFile)

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

	// The active cluster: reads.
	s.handle("GET /api/nodes", accessRead, s.handleNodes)
	s.handle("GET /api/namespaces", accessRead, s.handleNamespaces)
	s.handle("GET /api/pods", accessRead, s.handlePods)
	s.handle("GET /api/deployments", accessRead, s.handleDeployments)
	s.handle("GET /api/services", accessRead, s.handleServices)
	s.handle("GET /api/yaml", accessRead, s.handleYAML)
	s.handle("GET /api/events", accessRead, s.handleEvents)
	s.handle("GET /api/logs", accessRead, s.handleLogs)
	s.handle("GET /api/logs/stream", accessRead, s.handleLogStream)
	s.handle("GET /api/resource", accessRead, s.handleResourceList)
	s.handle("GET /api/resource/get", accessRead, s.handleResourceGet)
	s.handle("GET /api/k8s/{path...}", accessRead, s.handlePassthrough)
	s.handle("GET /api/metrics/sources", accessRead, s.handleMetricSources)
	s.handle("GET /api/metrics/overview", accessRead, s.handleOverview)
	s.handle("GET /api/metrics/allocation", accessRead, s.handleAllocation)
	s.handle("GET /api/metrics/cluster", accessRead, s.handleClusterSeries)
	s.handle("GET /api/metrics/node", accessRead, s.handleNodeSeries)
	s.handle("GET /api/metrics/pod", accessRead, s.handlePodSeries)
	s.handle("GET /api/crds", accessRead, s.handleCrds)
	s.handle("GET /api/crds/instances", accessRead, s.handleCrdInstances)
	// Live updates (§7.10): change notifications and the informers behind them.
	s.handle("GET /api/watch", accessRead, s.handleWatch)
	s.handle("GET /api/watch/status", accessRead, s.handleWatchStatus)

	// The active cluster: changes (audited).
	s.handle("DELETE /api/pods", accessWrite, s.handleDeletePod)
	s.handle("POST /api/deployments/scale", accessWrite, s.handleScale)
	s.handle("POST /api/deployments/restart", accessWrite, s.handleRestart)
	s.handle("POST /api/exec", accessExec, s.handleExec)
	s.handle("GET /api/exec/ws", accessExec, s.handleTerminal)
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
	s.handle("POST /api/nodes/drain", accessWrite, s.handleDrain)
	s.handle("PUT /api/nodes/taints", accessWrite, s.handleSetTaints)
	s.handle("POST /api/pods/bulk-delete", accessWrite, s.handleBulkDeletePods)
	s.handle("POST /api/pods/debug", accessExec, s.handleDebugPod)
	s.handle("POST /api/cronjobs/trigger", accessWrite, s.handleTriggerCronJob)
	s.handle("POST /api/cronjobs/suspend", accessWrite, s.handleSuspendCronJob)
	s.handle("POST /api/namespaces", accessWrite, s.handleCreateNamespace)
	s.handle("DELETE /api/namespaces", accessWrite, s.handleDeleteNamespace)

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
}

// auditActions names the audit action of routes that record a named action
// instead of "<METHOD> <route pattern>" (§3.4), e.g. a WebSocket session,
// whose row is written once, when the connection is upgraded.
var auditActions = map[string]string{
	"GET /api/exec/ws": "exec.session",
}
