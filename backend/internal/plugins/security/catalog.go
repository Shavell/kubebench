package security

import (
	corev1 "k8s.io/api/core/v1"
)

// Targets: the kind of object a check evaluates.
const (
	targetWorkload       = "workload"       // pod templates (see workload)
	targetNamespace      = "namespace"      // Namespaces
	targetServiceAccount = "serviceaccount" // the namespaces' default ServiceAccounts
	targetRole           = "role"           // Roles and ClusterRoles
	targetBinding        = "binding"        // RoleBindings and ClusterRoleBindings
)

// Check is one rule of the catalog. Exactly one evaluator, the one of its
// Target, is set; it returns the evidence and whether the object fails.
type Check struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Severity    string   `json:"severity"`
	Category    string   `json:"category"`
	Frameworks  []string `json:"frameworks"`
	Target      string   `json:"target"`
	Remediation string   `json:"remediation"`

	// message is a finding's text; a %s stands for the evidence.
	message string
	// applies narrows the workloads a workload check looks at.
	applies   func(*workload) bool
	workload  func(*scan, *workload) ([]string, bool)
	namespace func(*scan, *corev1.Namespace) ([]string, bool)
	account   func(*scan, *corev1.ServiceAccount) ([]string, bool)
	role      func(*scan, *roleInfo) ([]string, bool)
	binding   func(*scan, *bindingInfo) ([]string, bool)
}

// Framework labels. CIS numbers follow the CIS Kubernetes Benchmark
// (v1.8+, section 5 "Policies"); PSS are the Pod Security Standards levels.
const (
	pssBaseline   = "PSS baseline"
	pssRestricted = "PSS restricted"
	nsaHardening  = "NSA hardening"
)

// catalog is every check, in the order the catalog view lists them.
var catalog = []*Check{
	// Pod Security Standards, baseline.
	{
		ID: "privileged", Title: "Privileged containers", Severity: SeverityCritical, Category: CategoryBaseline,
		Frameworks: []string{"CIS 5.2.2", pssBaseline}, Target: targetWorkload,
		message:     "Containers run privileged, with full access to the node: %s",
		Remediation: "Remove securityContext.privileged (and Windows hostProcess) from the containers. Grant the specific capabilities the workload needs instead.",
		workload:    checkPrivileged,
	},
	{
		ID: "host-namespaces", Title: "Host namespaces shared", Severity: SeverityHigh, Category: CategoryBaseline,
		Frameworks: []string{"CIS 5.2.3", "CIS 5.2.4", "CIS 5.2.5", pssBaseline}, Target: targetWorkload,
		message:     "The pod shares the node's namespaces: %s",
		Remediation: "Set hostNetwork, hostPID and hostIPC to false (or leave them out). Expose network services through a Service instead of the host network.",
		workload:    checkHostNamespaces,
	},
	{
		ID: "host-path", Title: "hostPath volumes", Severity: SeverityHigh, Category: CategoryBaseline,
		Frameworks: []string{"CIS 5.2.12", pssBaseline}, Target: targetWorkload,
		message:     "The pod mounts directories of the node: %s",
		Remediation: "Replace hostPath volumes with PersistentVolumeClaims, emptyDir, ConfigMaps or Secrets. If node access is unavoidable, mount the narrowest path read-only.",
		workload:    checkHostPath,
	},
	{
		ID: "host-ports", Title: "Host ports", Severity: SeverityMedium, Category: CategoryBaseline,
		Frameworks: []string{"CIS 5.2.13", pssBaseline}, Target: targetWorkload,
		message:     "Containers bind ports of the node: %s",
		Remediation: "Remove hostPort from the container ports and publish them through a Service (NodePort or LoadBalancer) or an Ingress.",
		workload:    checkHostPorts,
	},
	{
		ID: "capabilities", Title: "Capabilities beyond the baseline", Severity: SeverityHigh, Category: CategoryBaseline,
		Frameworks: []string{"CIS 5.2.8", "CIS 5.2.9", pssBaseline}, Target: targetWorkload,
		message:     "Containers add Linux capabilities outside the baseline allowlist: %s",
		Remediation: "Remove the capabilities from securityContext.capabilities.add; the baseline allows only AUDIT_WRITE, CHOWN, DAC_OVERRIDE, FOWNER, FSETID, KILL, MKNOD, NET_BIND_SERVICE, SETFCAP, SETGID, SETPCAP, SETUID and SYS_CHROOT.",
		workload:    checkCapabilities,
	},
	{
		ID: "proc-mount", Title: "Unmasked /proc", Severity: SeverityMedium, Category: CategoryBaseline,
		Frameworks: []string{pssBaseline}, Target: targetWorkload,
		message:     "Containers use a procMount other than Default: %s",
		Remediation: "Remove securityContext.procMount or set it to Default.",
		workload:    checkProcMount,
	},
	{
		ID: "unsafe-sysctls", Title: "Unsafe sysctls", Severity: SeverityMedium, Category: CategoryBaseline,
		Frameworks: []string{pssBaseline}, Target: targetWorkload,
		message:     "The pod sets sysctls outside the safe set: %s",
		Remediation: "Remove the sysctls from securityContext.sysctls, or configure them on dedicated, tainted nodes instead.",
		workload:    checkSysctls,
	},
	{
		ID: "selinux-apparmor", Title: "SELinux or AppArmor overrides", Severity: SeverityMedium, Category: CategoryBaseline,
		Frameworks: []string{pssBaseline}, Target: targetWorkload,
		message:     "The pod weakens its SELinux or AppArmor confinement: %s",
		Remediation: "Leave seLinuxOptions.user and role unset, use a container_t-family type, and do not set an Unconfined AppArmor profile.",
		workload:    checkSELinuxAppArmor,
	},
	// Pod Security Standards, restricted.
	{
		ID: "privilege-escalation", Title: "Privilege escalation allowed", Severity: SeverityMedium, Category: CategoryRestricted,
		Frameworks: []string{"CIS 5.2.6", pssRestricted}, Target: targetWorkload,
		message:     "Containers do not set allowPrivilegeEscalation: false: %s",
		Remediation: "Set securityContext.allowPrivilegeEscalation: false on every container.",
		workload:    checkPrivilegeEscalation,
	},
	{
		ID: "run-as-non-root", Title: "Containers may run as root", Severity: SeverityMedium, Category: CategoryRestricted,
		Frameworks: []string{"CIS 5.2.7", pssRestricted}, Target: targetWorkload,
		message:     "Containers are not required to run as a non-root user: %s",
		Remediation: "Set securityContext.runAsNonRoot: true (on the pod or every container) and a non-zero runAsUser; build the image with a non-root USER.",
		workload:    checkRunAsNonRoot,
	},
	{
		ID: "seccomp", Title: "No seccomp profile", Severity: SeverityMedium, Category: CategoryRestricted,
		Frameworks: []string{"CIS 5.7.2", pssRestricted}, Target: targetWorkload,
		message:     "Containers run without a RuntimeDefault or Localhost seccomp profile: %s",
		Remediation: "Set securityContext.seccompProfile.type: RuntimeDefault on the pod (or Localhost with a reviewed profile).",
		workload:    checkSeccomp,
	},
	{
		ID: "drop-capabilities", Title: "Capabilities not dropped", Severity: SeverityLow, Category: CategoryRestricted,
		Frameworks: []string{"CIS 5.2.10", pssRestricted}, Target: targetWorkload,
		message:     "Containers do not drop ALL capabilities: %s",
		Remediation: "Set securityContext.capabilities.drop: [\"ALL\"] and add back only NET_BIND_SERVICE if the container binds a low port.",
		workload:    checkDropCapabilities,
	},
	{
		ID: "volume-types", Title: "Restricted volume types", Severity: SeverityLow, Category: CategoryRestricted,
		Frameworks: []string{pssRestricted}, Target: targetWorkload,
		message:     "The pod uses volume types the restricted level does not allow: %s",
		Remediation: "Use only configMap, csi, downwardAPI, emptyDir, ephemeral, persistentVolumeClaim, projected and secret volumes; reach other storage through a PersistentVolumeClaim.",
		workload:    checkVolumeTypes,
	},
	// Workload hygiene.
	{
		ID: "resource-requests", Title: "Missing resource requests", Severity: SeverityLow, Category: CategoryHygiene,
		Frameworks: []string{nsaHardening}, Target: targetWorkload,
		message:     "Containers do not request CPU or memory: %s",
		Remediation: "Set resources.requests.cpu and resources.requests.memory so the scheduler can place the pod and evictions treat it fairly.",
		workload:    checkRequests,
	},
	{
		ID: "resource-limits", Title: "Missing resource limits", Severity: SeverityMedium, Category: CategoryHygiene,
		Frameworks: []string{nsaHardening}, Target: targetWorkload,
		message:     "Containers have no CPU or memory limit: %s",
		Remediation: "Set resources.limits.memory (and cpu where appropriate), or give the namespace a LimitRange with defaults, so one workload cannot exhaust the node.",
		workload:    checkLimits,
	},
	{
		ID: "probes", Title: "Missing health probes", Severity: SeverityLow, Category: CategoryHygiene,
		Frameworks: []string{"Best practice"}, Target: targetWorkload,
		message:     "Containers have no liveness or readiness probe: %s",
		Remediation: "Add a readinessProbe so traffic only reaches ready pods, and a livenessProbe so a hung container is restarted.",
		applies:     (*workload).longRunning,
		workload:    checkProbes,
	},
	{
		ID: "image-tag", Title: "Mutable image tags", Severity: SeverityMedium, Category: CategoryHygiene,
		Frameworks: []string{"Best practice"}, Target: targetWorkload,
		message:     "Containers use the latest tag or no tag: %s",
		Remediation: "Pin images to an immutable version tag, or better to a digest (image@sha256:…), so every pod runs the image that was reviewed.",
		workload:    checkImageTag,
	},
	{
		ID: "read-only-root", Title: "Writable root filesystem", Severity: SeverityLow, Category: CategoryHygiene,
		Frameworks: []string{"CIS 5.7.3", nsaHardening}, Target: targetWorkload,
		message:     "Containers can write to their root filesystem: %s",
		Remediation: "Set securityContext.readOnlyRootFilesystem: true and mount an emptyDir where the application must write.",
		workload:    checkReadOnlyRoot,
	},
	// RBAC.
	{
		ID: "cluster-admin-binding", Title: "cluster-admin bindings", Severity: SeverityCritical, Category: CategoryRBAC,
		Frameworks: []string{"CIS 5.1.1"}, Target: targetBinding,
		message:     "The binding grants cluster-admin to: %s",
		Remediation: "Bind a role with only the permissions the subjects need; keep cluster-admin for break-glass accounts.",
		binding:     checkClusterAdminBinding,
	},
	{
		ID: "rbac-wildcards", Title: "Wildcards in roles", Severity: SeverityHigh, Category: CategoryRBAC,
		Frameworks: []string{"CIS 5.1.3"}, Target: targetRole,
		message:     "The role grants rules with wildcards: %s",
		Remediation: "Replace * in apiGroups, resources and verbs with the explicit groups, resources and verbs the subjects use.",
		role:        checkWildcards,
	},
	{
		ID: "secrets-access", Title: "Read access to Secrets", Severity: SeverityHigh, Category: CategoryRBAC,
		Frameworks: []string{"CIS 5.1.2"}, Target: targetRole,
		message:     "The role may read Secrets: %s",
		Remediation: "Remove get, list and watch on secrets, or limit the rule to the resourceNames the subjects need; list and watch reveal every Secret's values.",
		role:        checkSecretsAccess,
	},
	{
		ID: "pod-create", Title: "Pod creation and exec", Severity: SeverityMedium, Category: CategoryRBAC,
		Frameworks: []string{"CIS 5.1.4"}, Target: targetRole,
		message:     "The role may create pods or exec into them: %s",
		Remediation: "Grant create on pods (and pods/exec, pods/attach) only to the controllers and people who need it: a new pod can mount any Secret and ServiceAccount of its namespace.",
		role:        checkPodCreate,
	},
	{
		ID: "rbac-escalation", Title: "bind, escalate or impersonate", Severity: SeverityHigh, Category: CategoryRBAC,
		Frameworks: []string{"CIS 5.1.8"}, Target: targetRole,
		message:     "The role may escalate its privileges: %s",
		Remediation: "Remove the bind, escalate and impersonate verbs; they let the subjects grant themselves, or act with, permissions they do not hold.",
		role:        checkEscalation,
	},
	{
		ID: "default-service-account", Title: "Default ServiceAccount in use", Severity: SeverityLow, Category: CategoryRBAC,
		Frameworks: []string{"CIS 5.1.5"}, Target: targetServiceAccount,
		message:     "The namespace's default ServiceAccount should be inert: %s",
		Remediation: "Set automountServiceAccountToken: false on the default ServiceAccount, do not bind roles to it, and give each workload its own ServiceAccount.",
		account:     checkDefaultServiceAccount,
	},
	{
		ID: "service-account-token", Title: "Default ServiceAccount token mounted", Severity: SeverityMedium, Category: CategoryRBAC,
		Frameworks: []string{"CIS 5.1.5", "CIS 5.1.6"}, Target: targetWorkload,
		message:     "The pod runs as the default ServiceAccount with its API token mounted",
		Remediation: "Give the workload its own ServiceAccount, or set automountServiceAccountToken: false on the pod when it does not call the Kubernetes API.",
		workload:    checkServiceAccountToken,
	},
	// Network.
	{
		ID: "network-policy", Title: "Namespaces without NetworkPolicy", Severity: SeverityMedium, Category: CategoryNetwork,
		Frameworks: []string{"CIS 5.3.2"}, Target: targetNamespace,
		message:     "The namespace has no NetworkPolicy, so every pod accepts traffic from anywhere",
		Remediation: "Add a default-deny NetworkPolicy for ingress (and egress) and policies that allow the traffic the workloads need.",
		namespace:   checkNetworkPolicy,
	},
	// Secrets.
	{
		ID: "secret-env", Title: "Secrets in environment variables", Severity: SeverityMedium, Category: CategorySecrets,
		Frameworks: []string{"CIS 5.4.1"}, Target: targetWorkload,
		message:     "Containers read Secrets into environment variables: %s",
		Remediation: "Mount the Secret as a volume and read the files instead: environment variables leak into logs, crash dumps and child processes.",
		workload:    checkSecretEnv,
	},
	{
		ID: "hardcoded-secret", Title: "Credentials in plain environment values", Severity: SeverityHigh, Category: CategorySecrets,
		Frameworks: []string{"CIS 5.4.1"}, Target: targetWorkload,
		message:     "Environment variables that look like credentials have literal values in the manifest: %s",
		Remediation: "Move the values into a Secret (mounted as a file) or an external secret store; anyone who can read the workload can read them now.",
		workload:    checkHardcodedSecret,
	},
	// Namespaces.
	{
		ID: "psa-labels", Title: "No Pod Security admission level", Severity: SeverityLow, Category: CategoryNamespaces,
		Frameworks: []string{"CIS 5.2.1", pssBaseline}, Target: targetNamespace,
		message:     "Pod Security admission does not enforce a baseline or restricted level: %s",
		Remediation: "Label the namespace pod-security.kubernetes.io/enforce=baseline (or restricted); try it first with the warn and audit labels.",
		namespace:   checkPSALabels,
	},
	{
		ID: "default-namespace", Title: "Workloads in the default namespace", Severity: SeverityLow, Category: CategoryNamespaces,
		Frameworks: []string{"CIS 5.7.4"}, Target: targetWorkload,
		message:     "The workload runs in the default namespace",
		Remediation: "Move the workload to a namespace of its own, where quotas, policies and RBAC can be scoped to it.",
		workload:    checkDefaultNamespace,
	},
}

// checksByID indexes the catalog.
var checksByID = func() map[string]*Check {
	m := make(map[string]*Check, len(catalog))
	for _, c := range catalog {
		m[c.ID] = c
	}
	return m
}()
