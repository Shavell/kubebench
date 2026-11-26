/** Domain types shaped after the Kubernetes API, trimmed to what the UI shows. */

/** Parameters of a server reason code (design §3.3), e.g. `{"cluster": "prod"}`. */
export type ReasonParams = Record<string, unknown>

/** A server reason code with its parameters, e.g. one of a response's `warningReasons`. */
export interface ReasonRef {
  reason: string
  params?: ReasonParams
}

export interface K8sNode {
  name: string
  roles: string[]
  status: 'Ready' | 'NotReady'
  version: string
  os: string
  kernelVersion: string
  containerRuntime: string
  cpuCapacity: number
  memoryCapacityGi: number
  age: string
  internalIP: string
  labels: Record<string, string>
  annotations: Record<string, string>
  /** The node is cordoned (spec.unschedulable); absent = schedulable. */
  unschedulable?: boolean
  /** spec.taints; absent = none. */
  taints?: NodeTaint[]
}

export type MetricKind = 'cpu' | 'memory' | 'network' | 'fs'

/** Raw Kubernetes object (unstructured) for the generic resource browser. */
export interface KubeObject {
  apiVersion?: string
  kind?: string
  metadata?: {
    name?: string
    namespace?: string
    creationTimestamp?: string
    labels?: Record<string, string>
    annotations?: Record<string, string>
    [k: string]: unknown
  }
  /** Synthetic age for mock objects that carry no creationTimestamp. */
  age?: string
  [k: string]: unknown
}

/** Global KubeBench role (users.role). */
export type GlobalRole = 'admin' | 'user'
/** Per-cluster assignment role (user_clusters.role). */
export type AssignmentRole = 'viewer' | 'operator'
/** Effective role of a user on one cluster (admins are admin everywhere). */
export type ClusterRole = 'admin' | AssignmentRole
export type ClusterEnvironment = '' | 'development' | 'staging' | 'production'

export interface ClusterAssignment {
  clusterId: number
  cluster: string
  role: AssignmentRole
}

export interface AppUser {
  id: number
  username: string
  role: GlobalRole
  mustChangePassword: boolean
  /** Where the account comes from (design §3.6); external users have no local password. */
  source: AuthSource
  /** Name and email reported by the identity provider; empty for local users. */
  displayName: string
  email: string
  /** Assigned cluster names (kept for v0.4 compatibility). */
  clusters: string[]
  assignments: ClusterAssignment[]
}

/** Effective permissions on the active cluster, as reported by /api/info. */
export interface SessionPermissions {
  role: ClusterRole
  canRead: boolean
  canWrite: boolean
  canExec: boolean
  isAdmin: boolean
  /** The active cluster is flagged read-only (blocks write/exec for everyone). */
  readOnly: boolean
}

export interface ActiveCluster {
  id: number
  name: string
  environment: ClusterEnvironment
  readOnly: boolean
  /** Provider, version and support window (design §7.12); null until the runtime is built. */
  platform?: ClusterPlatform | null
}

/** What the backend can do on the active cluster (RBAC + capability probes). */
export interface LiveCapabilities {
  crds: boolean
  helm: boolean
  exec: boolean
  /** Catalog keys of the optional (OpenShift) resource types the cluster serves (design §7.15). */
  optionalResources: string[]
}

export interface ServerFeatures {
  /** Clusters may be added by a server-side kubeconfig path. */
  pathKubeconfig: boolean
  /** The server has a plugins directory for external (runtime-loaded) UI plugins. */
  externalPlugins: boolean
  /** Pasted EKS, GKE and AKS kubeconfigs may sign in with the server's own cloud identity. */
  ambientCloudIdentity: boolean
}

/** The session as described by GET /api/info (design §4.1), normalised. */
export interface ServerInfo {
  version: string
  commit: string
  user: string
  role: GlobalRole
  /** Every cluster request is refused until the user picks a new password. */
  mustChangePassword: boolean
  /** Where the signed-in account comes from (design §3.6). */
  authSource: AuthSource
  /** Name of the active cluster; empty when there is none. */
  context: string
  /** API server URL of the active cluster, when its runtime is up. */
  server: string
  cluster: ActiveCluster | null
  permissions: SessionPermissions
  capabilities: LiveCapabilities
  features: ServerFeatures
  /** Why no cluster runtime is available (no assignment, bad kubeconfig, …); empty otherwise. */
  clusterError: string
  /** Reason code and parameters of clusterError, for its translation (§3.3). */
  clusterErrorReason?: string
  clusterErrorParams?: ReasonParams
}

/** Options of GET /api/logs (design §4.4). */
export interface PodLogOptions {
  /** Number of lines from the end (server default 300, at most 5000). */
  tail?: number
  /** Logs of the previous, terminated instance of the container. */
  previous?: boolean
  /** Only lines written in the last N seconds. */
  sinceSeconds?: number
}

/** One row of the audit log (GET /api/audit). */
export interface AuditEntry {
  id: number
  /** RFC 3339 timestamp. */
  ts: string
  user: string
  cluster: string
  action: string
  target: string
  status: number
  detail: string
}

export interface AuditPage {
  items: AuditEntry[]
  /** Pass as `before` to load the next (older) page; null when exhausted. */
  nextBefore: number | null
}

export interface AuditQuery {
  limit?: number
  before?: number
  user?: string
  cluster?: string
  action?: string
}

export interface PluginAvailability {
  available: boolean
  /** The server's English explanation; reasonCode and reasonParams translate it (§3.3). */
  reason?: string
  reasonCode?: string
  reasonParams?: ReasonParams
}

/** A plugin known to the server: a backend plugin or an installed external UI plugin. */
export interface ServerPlugin {
  id: string
  name: string
  version: string
  description: string
  category: string
  builtin: boolean
  external: boolean
  enabled: boolean
  status: PluginAvailability
  /** Script URL of an external plugin (`/plugins/<id>/<entry>`); empty otherwise. */
  entry: string
}

/** GET /api/plugins: server plugins plus the admin's enable/disable choices for any plugin id. */
export interface PluginCatalog {
  plugins: ServerPlugin[]
  settings: Record<string, { enabled: boolean }>
}

export interface MetricsSourceInfo {
  id: string
  name: string
  available: boolean
  detail?: string
  /** Why an unavailable source has no data. */
  unavailable?: UnavailableReason
}

export interface ClusterEntry {
  id?: number
  name: string
  /** How the credentials are stored: kubeconfig, incluster or address (bookmark). */
  source?: string
  kubeconfigPath?: string
  address?: string
  active: boolean
  /** Context/server resolved from the kubeconfig, when readable. */
  context?: string
  server?: string
  error?: string
  readOnly?: boolean
  environment?: ClusterEnvironment
  /** The caller's effective role on this cluster. */
  role?: ClusterRole
  /** Provider, version and support window (design §7.12). */
  platform?: ClusterPlatform | null
  /** Admins only: the cloud whose identity signs in to it in process. */
  cloudAuth?: CloudCredentialProvider
  /** Admins only: the provider of stored per-cluster credentials (absent = the server's identity). */
  cloudCredentials?: CloudCredentialProvider
}

/** Fields an admin can change on a cluster (PATCH /api/clusters/{id}). */
export interface ClusterUpdate {
  name?: string
  readOnly?: boolean
  environment?: ClusterEnvironment
  /** Replaces the per-cluster cloud credentials; null removes them (design §7.12). */
  credentials?: CloudCredentials | null
}

export interface Namespace {
  name: string
  status: 'Active' | 'Terminating'
  age: string
}

export type PodPhase =
  | 'Running'
  | 'Pending'
  | 'Succeeded'
  | 'Failed'
  | 'CrashLoopBackOff'
  | 'ContainerCreating'

export interface ResourceAmounts {
  cpu: string
  memory: string
}

export interface ContainerSpec {
  name: string
  image: string
  command?: string[]
  args?: string[]
  env: { name: string; value: string }[]
  requests: ResourceAmounts
  limits: ResourceAmounts
  ports?: string
}

export interface Pod {
  name: string
  namespace: string
  status: PodPhase
  readyContainers: number
  totalContainers: number
  restarts: number
  age: string
  node: string
  podIP: string
  controlledBy?: string
  labels: Record<string, string>
  containers: ContainerSpec[]
}

export interface K8sEvent {
  type: 'Normal' | 'Warning'
  reason: string
  message: string
  age: string
  count: number
  source: string
}

export interface Deployment {
  name: string
  namespace: string
  readyReplicas: number
  replicas: number
  upToDate: number
  available: number
  age: string
  labels: Record<string, string>
  /** Rollouts are paused (kubectl rollout pause); absent = not paused. */
  paused?: boolean
}

export interface Service {
  name: string
  namespace: string
  type: 'ClusterIP' | 'NodePort' | 'LoadBalancer' | 'ExternalName'
  clusterIP: string
  externalIP?: string
  ports: string
  age: string
}

export interface CrdDefinition {
  /** Full name, e.g. "certificates.cert-manager.io". */
  name: string
  group: string
  kind: string
  versions: string[]
  scope: 'Namespaced' | 'Cluster'
  shortNames: string[]
  age: string
  /**
   * additionalPrinterColumns of the listed (first served) version, in order:
   * the keys of `CustomResource.extra`. Name, Namespace and Age are left out.
   */
  printerColumns: PrinterColumn[]
}

/** One CRD printer column. Like `kubectl get`, priority 0 shows by default, higher ones with -o wide. */
export interface PrinterColumn {
  name: string
  /** integer, number, string, boolean or date (the value is then an age). */
  type: string
  description?: string
  priority: number
}

export interface CustomResource {
  name: string
  namespace?: string
  crdName: string
  status: string
  age: string
  /**
   * Printer column values by column name, formatted like `kubectl get` (a date as
   * an age), e.g. { Secret: "shop-tls" }. A column without a value is missing.
   */
  extra: Record<string, string>
}

export interface MetricPoint {
  time: string
  value: number
}

export interface ResourceAllocation {
  unit: string
  allocatable: number
  requests: number
  limits: number
  /** Measured use; null when no metrics source has a value (never a made-up 0). */
  usage: number | null
}

export interface ClusterAllocation {
  cpu: ResourceAllocation
  memory: ResourceAllocation
  /** Why `usage` is null. */
  usageUnavailable?: UnavailableReason
}

/** Why a value is unknown: the server's reason.Message (design §3.3). */
export interface UnavailableReason {
  error: string
  reason?: string
  params?: ReasonParams
}

export interface ClusterStats {
  nodeCount: number
  readyNodes: number
  podCount: number
  runningPods: number
  namespaceCount: number
  /** Null when no metrics source measured the use; see usageUnavailable. */
  cpuUsagePercent: number | null
  memoryUsagePercent: number | null
  cpuCapacityCores: number
  memoryCapacityGi: number
  /** Why the usage percentages are null. */
  usageUnavailable?: UnavailableReason
}

// --- Resource editing (design §7.2) ---------------------------------------

/** One object of any kind, for the generic YAML, apply, delete and metadata endpoints. */
export interface ObjectRef {
  /** e.g. "apps/v1", "v1", "cert-manager.io/v1". */
  apiVersion: string
  kind: string
  /** Empty or null for cluster-scoped kinds. */
  namespace?: string | null
  name: string
}

/** What applying a document did, or in a dry run would do. */
export type ApplyAction = 'created' | 'configured' | 'unchanged'

/** The outcome of one object of POST /api/resource/apply. */
export interface ApplyResult {
  apiVersion: string
  kind: string
  /** The namespace it went to (defaulted when the manifest named none); absent for cluster-scoped kinds. */
  namespace?: string
  name: string
  /** Absent when the object failed (see `error`). */
  action?: ApplyAction
  error?: string
  /**
   * §3.3 code of `error`: `conflict` means fields the manifest changes are
   * owned by another field manager (a forced apply takes them over);
   * `not_found` a kind the cluster does not serve.
   */
  code?: string
  /** Reason code and parameters of `error` when KubeBench explains it itself (§3.3). */
  reason?: string
  params?: ReasonParams
  /** E.g. that the namespace of a cluster-scoped object was ignored. */
  warnings?: string[]
  /** The reasons of `warnings`, index by index. */
  warningReasons?: ReasonRef[]
}

export interface ApplyOptions {
  /** For namespaced objects that name no namespace; the server uses "default" without it. */
  namespace?: string
  /** Validate everything on the server (admission included) without persisting anything. */
  dryRun?: boolean
  /** Take over fields owned by other field managers instead of reporting a conflict. */
  force?: boolean
}

/** What happens to the objects a deleted one owns (e.g. a Deployment's ReplicaSets and Pods). */
export type DeletePropagation = 'Background' | 'Foreground' | 'Orphan'

export interface DeleteOptions {
  /** Default: the resource's own default (Background for built-in workloads). */
  propagation?: DeletePropagation
  /** Overrides the termination grace period; 0 deletes at once. */
  gracePeriodSeconds?: number
}

/** Labels and annotations to set; a null value removes the key, keys not named stay. */
export interface MetadataPatch {
  labels?: Record<string, string | null>
  annotations?: Record<string, string | null>
}

// ---- Workload and node operations (design §7.3) ---------------------------

export type TaintEffect = 'NoSchedule' | 'PreferNoSchedule' | 'NoExecute'

/** A node taint (spec.taints). */
export interface NodeTaint {
  key: string
  value: string
  effect: TaintEffect
}

/** Options of a node drain, after the kubectl drain flags. */
export interface DrainOptions {
  /** Leave DaemonSet Pods on the node; without it their presence stops the drain. */
  ignoreDaemonSets: boolean
  /** Also evict Pods with emptyDir volumes, whose data is lost. */
  deleteEmptyDirData: boolean
  /** Also evict Pods no controller manages; nothing recreates them. */
  force: boolean
  /** Seconds each Pod gets to stop; -1 = the Pod's own terminationGracePeriodSeconds. */
  gracePeriod: number
  /** Upper bound for the evictions, 1 to 600 seconds. */
  timeoutSeconds: number
}

/** What a drain did (POST /api/nodes/drain). Pods are "namespace/name". */
export interface DrainResult {
  node: string
  /** The node is unschedulable: every drain cordons it first. */
  cordoned: boolean
  /** Every Pod that had to leave the node is gone. */
  completed: boolean
  evicted: string[]
  /** Pods that should have left but are still there; `reason` translates KubeBench's own `error` (§3.3). */
  failed: { pod: string; error: string; reason?: string }[]
  /** The failed Pods a PodDisruptionBudget kept on the node. */
  blockedByPDB: string[]
  /** Pods a drain leaves alone. */
  skipped: { pod: string; reason: 'daemonSet' | 'mirror' }[]
  /** Problems that stopped the drain before any eviction. */
  errors: string[]
}

/** One revision of a Deployment: one of its ReplicaSets. */
export interface RolloutRevision {
  revision: number
  replicaSet: string
  images: string[]
  changeCause: string
  /** RFC 3339 creation time of the ReplicaSet. */
  created: string
  replicas: number
  readyReplicas: number
  current: boolean
}

/** GET /api/deployments/history. */
export interface RolloutHistory {
  currentRevision: number
  paused: boolean
  /** Newest first. */
  revisions: RolloutRevision[]
}

export type RestartableKind = 'Deployment' | 'StatefulSet' | 'DaemonSet'
export type ScalableKind = 'Deployment' | 'StatefulSet' | 'ReplicaSet'

export interface PodKey {
  namespace: string
  name: string
}

/** The most Pods one bulk delete takes (the backend's kube.MaxBulkDelete). */
export const MAX_BULK_DELETE = 200

/** The outcome for one Pod of a bulk delete; `error` is absent on success. */
export interface PodDeleteResult extends PodKey {
  error?: string
}

/** The ephemeral container a debug request added. */
export interface DebugContainer {
  namespace: string
  pod: string
  container: string
}

/** The Job a CronJob trigger created. */
export interface CronJobRun {
  namespace: string
  job: string
}

// ---- Updates and What's new (design §7.11) ----

/** Which releases the update check reports: none, releases, or prereleases too. */
export type UpdateMode = 'off' | 'stable' | 'prerelease'

/** GET /api/updates: the latest check for the server's update mode. */
export interface UpdateStatus {
  mode: UpdateMode
  /** KUBEBENCH_UPDATE_MODE, the mode until an admin chooses one. */
  defaultMode: UpdateMode
  /** The build version, or "unknown" for development and snapshot builds. */
  current: string
  /** The newest release the mode admits; empty when none is published. */
  latest: string
  available: boolean
  prerelease: boolean
  /** RFC 3339, or null. */
  publishedAt: string | null
  /** The release page (the releases page when there is no release); may be empty. */
  url: string
  /** Release notes as published: Markdown source, never HTML. */
  notes: string
  /** Time of the last successful check (RFC 3339), or null. */
  checkedAt: string | null
  /** Why the last check failed; empty after a success. */
  error: string
  /** Reason code and parameters of error, for its translation (§3.3). */
  errorReason?: string
  errorParams?: ReasonParams
}

// Live updates (design §7.10, GET /api/watch).

/**
 * How a topic's changes reach the UI: `live` as they happen, `starting`
 * while the server's informer lists the resource, `polling` when the
 * server cannot watch it (`reason` says why) and the UI refreshes it on a
 * timer instead.
 */
export type WatchTopicMode = 'starting' | 'live' | 'polling'

/** `event: status` of a watch stream. */
export interface WatchTopicStatus {
  /** A resource: `pods`, `deployments`, `applications.argoproj.io`, … */
  topic: string
  mode: WatchTopicMode
  reason?: string
  /** Reason code and parameters of reason, for its translation (§3.3). */
  reasonCode?: string
  reasonParams?: ReasonParams
}

/** `event: change`: an object of `topic` changed in `namespace` ('' = cluster-scoped or anywhere). */
export interface WatchChange {
  topic: string
  namespace: string
}

/** Where a cluster runs (design §7.12): a managed service, a distribution, or plain Kubernetes. */
export type CloudProvider =
  | 'eks'
  | 'aks'
  | 'gke'
  | 'doks'
  | 'lke'
  | 'ovh'
  | 'openshift'
  | 'rke2'
  | 'k3s'
  | 'kubernetes'

/** The infrastructure under the nodes; empty when unknown. */
export type InfraCloud = '' | 'aws' | 'azure' | 'gcp' | 'digitalocean' | 'linode' | 'ovh'

export type VersionSupportStatus = 'supported' | 'ending-soon' | 'end-of-life' | 'unknown'

/** Whether the cluster's minor version is still supported by its provider. */
export interface VersionSupport {
  status: VersionSupportStatus
  /** End of standard support (YYYY-MM-DD), empty when the table has none. */
  endDate: string
  /** The lifecycle table used: eks, aks, gke, openshift or kubernetes (upstream); empty for none. */
  basis: string
  /** OpenShift only (design §7.15): the current life-cycle phase. */
  phase?: OpenShiftPhase
  /** OpenShift only: the end of full support (YYYY-MM-DD). */
  fullSupportEnd?: string
  /** OpenShift only: the end of Extended Update Support, for minors that have it. */
  eusEnd?: string
}

/** Phases of an OpenShift minor version's life cycle. */
export type OpenShiftPhase = 'full' | 'maintenance' | 'eus' | 'ended'

export interface ClusterPlatform {
  provider: CloudProvider
  cloud: InfraCloud
  /** The API server's gitVersion; empty until detected. */
  version: string
  /** The distribution's own version: OpenShift's (design §7.15); absent elsewhere. */
  productVersion?: string
  support: VersionSupport
  /** False when only the API server URL was looked at (the cluster is not connected yet). */
  detected: boolean
}

export type CloudCredentialProvider = 'aws' | 'gcp' | 'azure'

/** Per-cluster cloud credentials an admin supplies; write-only (never returned). */
export type CloudCredentials =
  | { provider: 'aws'; accessKeyId: string; secretAccessKey: string; sessionToken?: string }
  | { provider: 'aws'; profile: string }
  | { provider: 'gcp'; serviceAccountJson: string }
  | { provider: 'azure'; tenantId: string; clientId: string; clientSecret: string }

/** Where an account comes from (design §3.6): a local password or an identity provider. */
export type AuthSource = 'local' | 'oidc' | 'ldap'
/** An identity provider's source. */
export type ExternalAuthSource = Exclude<AuthSource, 'local'>

/** One way to sign in besides a local account (GET /api/auth/methods). */
export interface SignInMethod {
  source: ExternalAuthSource
  name: string
  /** `redirect`: the browser goes to `loginUrl` (OIDC); `password`: POST /api/auth/login with the source. */
  type: 'redirect' | 'password'
  loginUrl?: string
}

export interface SignInMethods {
  /** Local accounts always work (the local admin is the break-glass account). */
  local: boolean
  providers: SignInMethod[]
}

/** One configuration value of a provider; secrets only report whether they are set. */
export interface AuthProviderSetting {
  key: string
  value: string
  secretSet?: boolean
}

/** A configured identity provider (GET /api/settings/auth). */
export interface AuthProviderInfo {
  source: ExternalAuthSource
  name: string
  settings: AuthProviderSetting[]
  status: { ok: boolean; error: string }
  /** Insecure settings: 'plaintext' (no TLS), 'tlsVerifyDisabled'. */
  warnings: string[]
}

export interface AuthSettings {
  publicUrl: string
  publicUrlConfigured: boolean
  requireGroup: boolean
  providers: AuthProviderInfo[]
}

/** The role a group mapping grants: global admin, or a role on one cluster. */
export type MappingRole = 'admin' | AssignmentRole

/** A group mapping as an admin edits it. */
export interface GroupMappingInput {
  group: string
  /** '' applies to every provider. */
  source: ExternalAuthSource | ''
  role: MappingRole
  /** Cluster name; '' for admin. */
  cluster: string
}

export interface GroupMapping extends GroupMappingInput {
  id: number
  /** False when no cluster of that name is in the inventory (it grants nothing yet). */
  clusterExists: boolean
}

/** Why a sign-in would be or was refused (failure class). */
export type SignInFailure =
  | 'invalid_credentials'
  | 'account_disabled'
  | 'domain_not_allowed'
  | 'no_matching_group'
  | 'username_conflict'
  | 'invalid_username'
  | 'invalid_request'
  | 'provider_error'
  | 'rate_limited'

/** "Who would I be": the result of signing a user in, computed without a session. */
export interface SignInPreview {
  source: ExternalAuthSource
  username: string
  groups: string[]
  groupsFrom: 'input' | 'directory' | 'lastSignIn' | 'none'
  admin: boolean
  clusters: { cluster: string; role: AssignmentRole; exists: boolean }[]
  /** IDs of the mappings that applied. */
  matched: number[]
  allowed: boolean
  reason: SignInFailure | ''
  /** Source of the account that already has this username; '' for none. */
  existingSource: AuthSource | ''
  lastSignIn: string | null
}
