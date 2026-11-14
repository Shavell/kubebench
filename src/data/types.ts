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

export type HelmReleaseStatus = 'deployed' | 'failed' | 'pending-upgrade' | 'superseded'

export interface HelmRelease {
  name: string
  namespace: string
  chart: string
  chartVersion: string
  appVersion: string
  revision: number
  status: HelmReleaseStatus
  updated: string
  history: { revision: number; status: HelmReleaseStatus; updated: string; description: string }[]
}

export interface HelmRepo {
  name: string
  url: string
  chartCount: number
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
