package api

import "kubebench/internal/reason"

// Reasons of the core API's own errors (docs/design-v0.5.md §3.3). The
// English texts are the "error" fallback; the UI translates the codes
// (src/core/reasons/locales). Shared validation codes are in package
// reason.
var (
	// Routing and requests.
	reasonNotFound         = reason.Define("api.not_found", "not found")
	reasonNoEndpoint       = reason.Define("api.no_endpoint", "no API endpoint {{path}}")
	reasonMethodNotAllowed = reason.Define("api.method_not_allowed", "{{method}} is not allowed on {{path}}")
	reasonRequiredParams   = reason.Define("api.required_params", "these parameters are required: {{fields}}")
	reasonCrossOrigin      = reason.Define("api.cross_origin", "cross-origin request rejected")
	reasonNoOrigin         = reason.Define("api.websocket_no_origin", "WebSocket request without an Origin rejected")
	reasonCrossSite        = reason.Define("api.cross_site", "cross-site request rejected")
	reasonShuttingDown     = reason.Define("api.shutting_down", "the server is shutting down")
	reasonInvalidNumber    = reason.Define("api.invalid_number", "\"{{value}}\" is not a valid number (at least {{min}})")
	reasonAuditTarget      = reason.Define("api.audit_target_too_long", "the target is too long to be recorded in the audit log (at most {{max}} characters)")
	reasonAuditDetail      = reason.Define("api.audit_detail_too_long", "the request parameters are too long to be recorded in the audit log (at most {{max}} characters)")

	// Sign-in and sessions.
	reasonSignIn             = reason.Define("auth.sign_in", "sign in to continue")
	reasonSessionExpired     = reason.Define("auth.session_expired", "your session has expired; sign in again")
	reasonSessionEnded       = reason.Define("auth.session_ended", "your session has ended; sign in again")
	reasonPasswordChange     = reason.Define("auth.password_change_required", "change your password to continue")
	reasonAdminRequired      = reason.Define("auth.admin_required", "this action needs an administrator")
	reasonInvalidCredentials = reason.Define("auth.invalid_credentials", "invalid username or password")
	reasonTooManyAttempts    = reason.Define("auth.too_many_attempts", "too many failed attempts; try again in {{minutes}} minutes")
	reasonWrongPassword      = reason.Define("auth.wrong_password", "the current password is incorrect")
	reasonSamePassword       = reason.Define("auth.same_password", "the new password must differ from the current one")

	// External identities (§3.6). A refused external sign-in names its
	// failure class (identity.Class*) as auth.signin.<class>; the cause
	// stays in the server log, as it may describe the provider.
	reasonUnknownSource       = reason.Define("auth.unknown_source", "unknown sign-in source \"{{source}}\"")
	reasonOIDCOff             = reason.Define("auth.oidc_not_configured", "OIDC sign-in is not configured")
	reasonExternalPassword    = reason.Define("auth.external_password", "this account signs in through an identity provider; its password is managed there")
	reasonPreviewSource       = reason.Define("auth.preview_source", "source must be oidc or ldap")
	reasonMappingID           = reason.Define("auth.invalid_mapping_id", "invalid mapping id \"{{id}}\"")
	reasonSignInRefused       = reason.Define("auth.signin.refused", "the sign-in was refused")
	reasonSignInCredentials   = reason.Define("auth.signin.invalid_credentials", "invalid username or password")
	reasonSignInDisabled      = reason.Define("auth.signin.account_disabled", "this account is disabled")
	reasonSignInDomain        = reason.Define("auth.signin.domain_not_allowed", "this email domain may not sign in")
	reasonSignInNoGroup       = reason.Define("auth.signin.no_matching_group", "no group of this account grants access to KubeBench")
	reasonSignInConflict      = reason.Define("auth.signin.username_conflict", "the username belongs to another KubeBench account")
	reasonSignInUsername      = reason.Define("auth.signin.invalid_username", "the account's username cannot be used in KubeBench")
	reasonSignInRequest       = reason.Define("auth.signin.invalid_request", "the sign-in could not be verified; try again")
	reasonSignInProviderError = reason.Define("auth.signin.provider_error", "the identity provider could not be reached")

	// Roles and the active cluster.
	reasonReadOnly         = reason.Define("cluster.read_only", "cluster \"{{cluster}}\" is read-only; an administrator can clear the flag in the Clusters view")
	reasonRoleNoChange     = reason.Define("role.cannot_change", "your role on cluster \"{{cluster}}\" is {{role}}, which cannot change resources")
	reasonRoleNoContribute = reason.Define("role.cannot_contribute", "your role on cluster \"{{cluster}}\" is {{role}}, which cannot change what every user of the cluster sees")
	reasonRoleNoBrowse     = reason.Define("role.cannot_browse", "your role on cluster \"{{cluster}}\" is {{role}}, which cannot open services in the browser")
	reasonNoActive         = reason.Define("cluster.none_active", "no cluster is active; choose one in the Clusters view")
	reasonActiveRemoved    = reason.Define("cluster.active_removed", "the active cluster was removed; choose another one")
	reasonAccessRevoked    = reason.Define("cluster.access_revoked", "you no longer have access to cluster \"{{cluster}}\"; choose another one")
	reasonAccessLost       = reason.Define("cluster.access_lost", "you no longer have access to cluster \"{{cluster}}\"")
	reasonClusterDown      = reason.Define("cluster.unavailable", "cluster \"{{cluster}}\" is unavailable: {{detail}}")
	reasonClusterRemoved   = reason.Define("cluster.removed", "cluster \"{{cluster}}\" was removed")
	reasonClusterGone      = reason.Define("cluster.gone", "the cluster was removed")
	reasonNoneAdded        = reason.Define("cluster.none_added", "no cluster has been added yet; add one in the Clusters view")
	reasonNoneAssigned     = reason.Define("cluster.none_assigned", "no cluster is assigned to you; ask an administrator")
	reasonRebuilding       = reason.Define("cluster.rebuilding", "the connection to cluster \"{{cluster}}\" is being rebuilt; try again")

	// Users.
	reasonUsername         = reason.Define("users.invalid_username", "a username is 1-64 letters, digits, '.', '_', '@' or '-'")
	reasonUserNothing      = reason.Define("users.nothing_to_update", "nothing to update: send role and/or password")
	reasonDeleteSelf       = reason.Define("users.delete_self", "you cannot delete your own account")
	reasonNoAssignments    = reason.Define("users.assignments_required", "send assignments (or the v0.4 clusterIds)")
	reasonClusterRole      = reason.Define("users.invalid_cluster_role", "invalid cluster role \"{{role}}\": use viewer or operator")
	reasonAssignedMissing  = reason.Define("users.assignment_missing", "a user or cluster of the assignments does not exist")
	reasonUserID           = reason.Define("users.invalid_id", "invalid user id \"{{id}}\"")
	reasonUserNotFound     = reason.Define("users.not_found", "user {{id}} not found")
	reasonGlobalRole       = reason.Define("users.invalid_role", "invalid role \"{{role}}\": use admin or user")
	reasonLastAdminDemote  = reason.Define("users.last_admin_demote", "cannot demote the last local admin: promote another local user first (the local admin is the break-glass account)")
	reasonLastAdminDelete  = reason.Define("users.last_admin_delete", "cannot delete the last local admin: promote another local user first (the local admin is the break-glass account)")
	reasonLastAdmin        = reason.Define("users.last_admin", "at least one admin must remain")
	reasonSeenVersionLimit = reason.Define("users.seen_version_length", "lastSeenVersion must be 1 to {{max}} characters")

	// Clusters and cloud credentials.
	reasonPathDisabled     = reason.Define("clusters.path_disabled", "adding clusters by server-side path is disabled (KUBEBENCH_ALLOW_PATH_KUBECONFIG); paste the kubeconfig instead")
	reasonPathWritten      = reason.Define("clusters.path_written", "{{path}} is inside {{dir}}, which KubeBench writes with content from requests; add a kubeconfig of the server's own instead")
	reasonReadKubeconfig   = reason.Define("clusters.read_kubeconfig", "cannot read the kubeconfig {{path}}: {{detail}}")
	reasonSourceRequired   = reason.Define("clusters.source_required", "kubeconfig content, path or API address is required")
	reasonBadKubeconfig    = reason.Define("clusters.invalid_kubeconfig", "invalid kubeconfig: {{detail}}")
	reasonUnknownContext   = reason.Define("clusters.unknown_context", "context \"{{context}}\" is not in the kubeconfig")
	reasonServerSideRefs   = reason.Define("clusters.server_side_references", "the kubeconfig uses {{references}}, which this server does not allow in pasted kubeconfigs (KUBEBENCH_ALLOW_KUBECONFIG_EXEC); embed the credentials as *-data fields or a token")
	reasonClusterName      = reason.Define("clusters.invalid_name", "a cluster name is 1-63 characters")
	reasonClusterNotFound  = reason.Define("clusters.not_found", "cluster \"{{name}}\" not found")
	reasonClusterID        = reason.Define("clusters.invalid_id", "invalid cluster id \"{{id}}\"")
	reasonClusterIDMissing = reason.Define("clusters.id_not_found", "cluster {{id}} not found")
	reasonClusterNothing   = reason.Define("clusters.nothing_to_update", "nothing to update: send name, readOnly, environment, credentials and/or allowedNamespaces")
	reasonNamespaceCount   = reason.Define("clusters.too_many_namespaces", "at most {{max}} allowed namespaces")
	reasonEnvironment      = reason.Define("clusters.invalid_environment", "invalid environment \"{{environment}}\": use development, staging, production or empty")
	reasonAddressOnly      = reason.Define("clusters.address_only", "address-only entries cannot be activated")
	reasonNoAccess         = reason.Define("clusters.no_access", "you have no access to cluster \"{{name}}\"")
	reasonConnectFailed    = reason.Define("clusters.connect_failed", "cannot connect to cluster \"{{name}}\": {{detail}}")
	reasonCredentialsJSON  = reason.Define("clusters.invalid_credentials", "invalid credentials: {{detail}}")
	reasonCloudKubeconfig  = reason.Define("clusters.credentials_kubeconfig_only", "cloud credentials apply to kubeconfig clusters only")
	reasonAmbientIdentity  = reason.Define("clusters.ambient_identity", "the kubeconfig signs in to {{provider}} with the KubeBench server's own cloud identity, which this server does not allow (KUBEBENCH_ALLOW_AMBIENT_CLOUD_IDENTITY); give the cluster its own credentials (an access key, a service-account key or a service principal)")

	// Plugins and updates.
	reasonPluginID          = reason.Define("plugins.invalid_id", "invalid plugin id \"{{id}}\"")
	reasonPluginDisabled    = reason.Define("plugins.disabled", "the {{id}} plugin is disabled")
	reasonPluginUnavailable = reason.Define("plugins.unavailable", "the plugin is not available on this cluster")
	reasonNoPluginFile      = reason.Define("plugins.no_file", "no plugin file {{path}}")
	reasonUpdatesOff        = reason.Define("updates.off", "update checks are off; choose another update mode first")

	// Resources, editing and operations.
	reasonUnknownResource  = reason.Define("resources.unknown", "unknown resource \"{{key}}\"")
	reasonKeyAndKind       = reason.Define("resources.key_and_kind", "name the resource by key or by apiVersion and kind, not both")
	reasonObjectRequired   = reason.Define("resources.object_required", "apiVersion, kind (or key) and name are required")
	reasonManifestTooLarge = reason.Define("editing.too_large", "the manifests exceed {{size}}")
	reasonManifestRead     = reason.Define("editing.read_failed", "reading the manifests: {{detail}}")
	reasonNoObjects        = reason.Define("editing.no_objects", "the request contains no objects")
	reasonNamespaceDelete  = reason.Define("editing.namespace_delete", "Namespaces are deleted with DELETE /api/namespaces, which protects the system namespaces")
	reasonNothingToChange  = reason.Define("editing.nothing_to_change", "nothing to change: name labels or annotations")
	reasonRedactedMetadata = reason.Define("editing.annotation_placeholder", "a value is a redaction placeholder; it would replace the real value of the annotation")
	reasonTaintsRequired   = reason.Define("nodes.taints_required", "taints is required; send [] to remove every taint")
	reasonPodOrSelector    = reason.Define("logs.pod_or_selector", "give either pod or selector")
	reasonSelector         = reason.Define("logs.invalid_selector", "invalid selector: {{detail}}")
	reasonLogTail          = reason.Define("logs.invalid_tail", "tail must be a number of lines, or 0 for the whole log")
	reasonNoPodMatches     = reason.Define("logs.no_match", "no pod in namespace \"{{namespace}}\" matches \"{{selector}}\"")
	reasonShell            = reason.Define("terminal.invalid_shell", "command must be one of the shells {{shells}}")
	reasonWebSocket        = reason.Define("terminal.websocket_required", "this endpoint needs a WebSocket connection")
	reasonTooManyLogs      = reason.Define("streams.too_many_logs", "you have {{max}} log streams open, the most at once; close one first")
	reasonTooManyTerminals = reason.Define("streams.too_many_terminals", "you have {{max}} terminals open, the most at once; close one first")
	reasonTooManyWatches   = reason.Define("streams.too_many_watches", "you have {{max}} live-update streams open, the most at once; close one first")
	reasonStreamClosed     = reason.Define("streams.closed", "the session was closed: {{cause}}")
	reasonTerminalIdle     = reason.Define("terminal.idle", "the session was closed after {{duration}} without input")
	reasonWatchRebuilt     = reason.Define("watch.connection_rebuilt", "the cluster connection was rebuilt")
	reasonTopic            = reason.Define("watch.invalid_topic", "invalid topic \"{{topic}}\": give a resource like pods or applications.argoproj.io")
	reasonTooManyTopics    = reason.Define("watch.too_many_topics", "at most {{max}} topics per stream")

	// Metrics.
	reasonNoPrometheus = reason.Define("metrics.no_prometheus", "no Prometheus was found on this cluster")
	reasonPromQuery    = reason.Define("metrics.prometheus_failed", "the query to Prometheus {{location}} failed: {{detail}}")

	// Kubernetes read passthrough.
	reasonPassthroughSize     = reason.Define("passthrough.too_large", "the response exceeds 16 MiB; page through it with limit and continue")
	reasonPassthroughRedirect = reason.Define("passthrough.redirect", "the cluster API answered {{status}}; redirects are not followed")
	reasonPassthroughPath     = reason.Define("passthrough.invalid_path", "invalid path \"{{path}}\"")
	reasonPassthroughPrefix   = reason.Define("passthrough.prefix", "only /api, /apis and /version can be read through /api/k8s")
	reasonPassthroughWatch    = reason.Define("passthrough.no_watch", "watch requests are not proxied; use the live-updates stream")
	reasonPassthroughFollow   = reason.Define("passthrough.no_follow", "followed logs are not proxied; use the log stream")
	reasonPassthroughSecrets  = reason.Define("passthrough.no_secrets", "secrets cannot be read through /api/k8s; use /api/resource")
	reasonPassthroughSubres   = reason.Define("passthrough.subresource", "the {{subresource}} subresource cannot be used through /api/k8s")

	// Service browser.
	reasonProxyOriginHost = reason.Define("proxy.origin_unknown_host", "the service browser cannot tell KubeBench's host from KUBEBENCH_PROXY_ORIGIN ({{origin}})")
	reasonProxySameSite   = reason.Define("proxy.origin_same_site", "the service browser is off: KUBEBENCH_PROXY_ORIGIN ({{origin}}) is on the same site as KubeBench ({{app}}), so its pages could replace your KubeBench session cookie; an administrator must set it to a host on another domain, or unset it")
	reasonExternalName    = reason.Define("proxy.external_name", "Service {{namespace}}/{{name}} points outside the cluster (ExternalName); open {{externalName}} directly")
	reasonNoServicePort   = reason.Define("proxy.no_service_port", "Service {{namespace}}/{{name}} has no port {{port}}")
	reasonNoPodPort       = reason.Define("proxy.no_pod_port", "Pod {{namespace}}/{{name}} has no port named {{port}}")
	reasonNoIngressRoute  = reason.Define("proxy.no_ingress_route", "Ingress {{namespace}}/{{name}} has no rule for host \"{{host}}\" and path \"{{path}}\" that leads to a Service")
	reasonRouteNotService = reason.Define("proxy.route_not_service", "Route {{namespace}}/{{name}} does not lead to a Service (it names a {{kind}})")
	reasonRouteNoPort     = reason.Define("proxy.route_no_port", "Service {{service}} of Route {{namespace}}/{{name}} has no port matching \"{{port}}\"")
	reasonBrowserEnded    = reason.Define("proxy.session_ended", "the browser session has ended")
	reasonKubeBenchEnded  = reason.Define("proxy.kubebench_session_ended", "your KubeBench session has ended; sign in again")
)
