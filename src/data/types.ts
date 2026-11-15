/** Domain types shaped after the Kubernetes API, trimmed to what the UI shows. */

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
}

/** What the backend can do on the active cluster (RBAC + capability probes). */
export interface LiveCapabilities {
  crds: boolean
  helm: boolean
  exec: boolean
}

export interface ServerFeatures {
  /** Clusters may be added by a server-side kubeconfig path. */
  pathKubeconfig: boolean
  /** The server has a plugins directory for external (runtime-loaded) UI plugins. */
  externalPlugins: boolean
}

/** The session as described by GET /api/info (design §4.1), normalised. */
export interface ServerInfo {
  version: string
  commit: string
  user: string
  role: GlobalRole
  /** Every cluster request is refused until the user picks a new password. */
  mustChangePassword: boolean
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
  reason?: string
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
}

/** Fields an admin can change on a cluster (PATCH /api/clusters/{id}). */
export interface ClusterUpdate {
  name?: string
  readOnly?: boolean
  environment?: ClusterEnvironment
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
}

export interface CustomResource {
  name: string
  namespace?: string
  crdName: string
  status: string
  age: string
  /** CRD-specific columns shown in the list, e.g. { Secret: "shop-tls" }. */
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
  usage: number
}

export interface ClusterAllocation {
  cpu: ResourceAllocation
  memory: ResourceAllocation
}

export interface ClusterStats {
  nodeCount: number
  readyNodes: number
  podCount: number
  runningPods: number
  namespaceCount: number
  cpuUsagePercent: number
  memoryUsagePercent: number
  cpuCapacityCores: number
  memoryCapacityGi: number
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
  /** E.g. that the namespace of a cluster-scoped object was ignored. */
  warnings?: string[]
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
  /** Pods that should have left but are still there. */
  failed: { pod: string; error: string }[]
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
}

/** `event: change`: an object of `topic` changed in `namespace` ('' = cluster-scoped or anywhere). */
export interface WatchChange {
  topic: string
  namespace: string
}
