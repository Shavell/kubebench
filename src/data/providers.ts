import type { QueryParams } from './apiClient'
import type {
  LogStream,
  LogStreamHandlers,
  LogStreamRequest,
  TerminalConnection,
  TerminalHandlers,
  TerminalTarget,
  WatchHandlers,
  WatchStream,
} from './streams'
import type {
  AppUser,
  AssignmentRole,
  AuditPage,
  AuditQuery,
  ClusterAllocation,
  ClusterEntry,
  ClusterStats,
  ClusterUpdate,
  GlobalRole,
  K8sEvent,
  CrdDefinition,
  CustomResource,
  Deployment,
  K8sNode,
  KubeObject,
  MetricKind,
  MetricPoint,
  MetricsSourceInfo,
  Namespace,
  PluginCatalog,
  Pod,
  PodLogOptions,
  ServerInfo,
  Service,
  ApplyOptions,
  ApplyResult,
  DeleteOptions,
  MetadataPatch,
  ObjectRef,
  CronJobRun,
  DebugContainer,
  DrainOptions,
  DrainResult,
  NodeTaint,
  PodDeleteResult,
  PodKey,
  RestartableKind,
  RolloutHistory,
  ScalableKind,
} from './types'

/**
 * The contract between the UI and any data source. Every capability has a
 * deterministic mock implementation (src/data/mock) and a live one that
 * calls the Go backend (src/data/api). Live mode never falls back to mock
 * data: an unavailable capability rejects with an ApiError instead.
 */

export interface ClusterProvider {
  listNodes(): Promise<K8sNode[]>
  listNamespaces(): Promise<Namespace[]>
  listPods(namespace?: string): Promise<Pod[]>
  listDeployments(namespace?: string): Promise<Deployment[]>
  listServices(namespace?: string): Promise<Service[]>
  getResourceYaml(kind: string, namespace: string | null, name: string): Promise<string>
  listEvents(kind: string, namespace: string | null, name: string): Promise<K8sEvent[]>
  getPodLogs(namespace: string, pod: string, container: string, options?: PodLogOptions): Promise<string[]>
  /** Generic list for any catalog resource key (configmaps, secrets, …). */
  listResource(key: string, namespace?: string): Promise<KubeObject[]>
  getResource(key: string, namespace: string | null, name: string): Promise<KubeObject>
  /** Runs one command in a container and returns its output lines. */
  execCommand(namespace: string, pod: string, container: string, command: string): Promise<string[]>
  /** Follows container logs as they are written (design §7.1, Server-Sent Events). */
  streamLogs(request: LogStreamRequest, handlers: LogStreamHandlers): LogStream
  /** Opens an interactive terminal (TTY) in a container (design §7.1, WebSocket). */
  openTerminal(target: TerminalTarget, handlers: TerminalHandlers): TerminalConnection
  /**
   * Read-only Kubernetes API passthrough (GET /api/k8s/<path>), e.g.
   * `k8sGet('/api/v1/namespaces')`. Secrets are not readable this way.
   */
  k8sGet<T = unknown>(path: string, params?: QueryParams): Promise<T>
  /** Workload mutations (in mock mode they change the in-memory dataset). */
  deletePod(namespace: string, name: string): Promise<void>
  scaleDeployment(namespace: string, name: string, replicas: number): Promise<void>
  restartDeployment(namespace: string, name: string): Promise<void>
  // Resource editing (design §7.2):
  /** YAML of any object; `neat` drops what the API server maintains. Secret values come redacted. */
  getYaml(ref: ObjectRef, options?: { neat?: boolean }): Promise<string>
  /** Server-side apply of YAML/JSON manifests (any number of documents): one result per object. */
  applyYaml(manifests: string, options?: ApplyOptions): Promise<ApplyResult[]>
  /**
   * Edits one existing object: sends the changes from `original` (the YAML
   * editing started from) to `edited` as a merge patch, so removed fields are
   * removed and placeholders left unchanged keep the Secret's values. One result.
   */
  editYaml(ref: ObjectRef, original: string, edited: string, options?: { dryRun?: boolean }): Promise<ApplyResult[]>
  deleteResource(ref: ObjectRef, options?: DeleteOptions): Promise<void>
  patchMetadata(ref: ObjectRef, patch: MetadataPatch): Promise<void>
  /**
   * A Secret with its values (`data`, base64). Needs the exec permission and
   * is audited; callers show the values once and never keep them.
   */
  revealSecret(namespace: string, name: string): Promise<KubeObject>
}

export interface CrdProvider {
  listCrds(): Promise<CrdDefinition[]>
  listCustomResources(crdName: string): Promise<CustomResource[]>
}

export interface MetricsProvider {
  listSources(): Promise<MetricsSourceInfo[]>
  clusterOverview(): Promise<ClusterStats>
  clusterAllocation(): Promise<ClusterAllocation>
  nodeSeries(node: string, metric: MetricKind, minutes: number): Promise<MetricPoint[]>
  clusterSeries(metric: MetricKind, minutes: number): Promise<MetricPoint[]>
  podSeries(
    namespace: string,
    pod: string,
    metric: MetricKind,
    minutes: number,
  ): Promise<MetricPoint[]>
}

export interface InventoryProvider {
  listClusters(): Promise<ClusterEntry[]>
  addCluster(entry: {
    name: string
    kubeconfigPath?: string
    content?: string
    address?: string
    context?: string
  }): Promise<void>
  removeCluster(name: string): Promise<void>
  /** Switches the backend to another cluster; the app reloads afterwards. */
  activateCluster(name: string): Promise<void>
  /** Admin: rename a cluster, set its environment or read-only flag. */
  updateCluster(id: number, update: ClusterUpdate): Promise<void>
}

export interface UsersProvider {
  listUsers(): Promise<AppUser[]>
  createUser(username: string, password: string, role: GlobalRole): Promise<void>
  /** Changes the global role and/or resets the password (forces a change at next sign-in). */
  updateUser(id: number, update: { role?: GlobalRole; password?: string }): Promise<void>
  deleteUser(id: number): Promise<void>
  /** Replaces the user's cluster assignments. */
  assignClusters(id: number, assignments: { clusterId: number; role: AssignmentRole }[]): Promise<void>
}

/** The signed-in user's own session. */
export interface SessionProvider {
  /** The session facts: user, active cluster, permissions, capabilities (GET /api/info). */
  info(): Promise<ServerInfo>
  /** Clears `mustChangePassword`; the server revokes the user's other sessions. */
  changePassword(currentPassword: string, newPassword: string): Promise<void>
  logout(): Promise<void>
}

export interface AuditProvider {
  /** Newest first; page with `before` = the previous page's `nextBefore`. */
  listAudit(query: AuditQuery): Promise<AuditPage>
}

export interface PluginsProvider {
  listPlugins(): Promise<PluginCatalog>
  /** Admin: enable or disable any plugin id (built-in, backend or external). */
  setPluginEnabled(id: string, enabled: boolean): Promise<void>
}

export interface DataProviders {
  cluster: ClusterProvider
  crd: CrdProvider
  metrics: MetricsProvider
  inventory: InventoryProvider
  users: UsersProvider
  session: SessionProvider
  audit: AuditProvider
  plugins: PluginsProvider
  workloadOps: WorkloadOpsProvider
  live: LiveProvider
}

/**
 * Node, rollout, workload, Pod, CronJob and namespace operations (design
 * §7.3). In mock mode they change the in-memory dataset.
 */
export interface WorkloadOpsProvider {
  /** Cordons (unschedulable) or uncordons a node. */
  cordonNode(name: string, unschedulable: boolean): Promise<void>
  /** Cordons the node and evicts its Pods; resolves when the drain ended, with every Pod's outcome. */
  drainNode(name: string, options: DrainOptions): Promise<DrainResult>
  /** Replaces the node's taints and returns the stored list. */
  setNodeTaints(name: string, taints: NodeTaint[]): Promise<NodeTaint[]>
  deploymentHistory(namespace: string, name: string): Promise<RolloutHistory>
  /** Refused (`conflict`) while the Deployment is paused or for the current revision. */
  rollbackDeployment(namespace: string, name: string, revision: number): Promise<void>
  pauseDeployment(namespace: string, name: string, paused: boolean): Promise<void>
  restartWorkload(kind: RestartableKind, namespace: string, name: string): Promise<void>
  scaleWorkload(kind: ScalableKind, namespace: string, name: string, replicas: number): Promise<void>
  /** Deletes at once (grace period 0), without waiting for the containers to stop. */
  forceDeletePod(namespace: string, name: string): Promise<void>
  /** Deletes 1 to MAX_BULK_DELETE Pods (else `bad_request`); one result per Pod, in order. */
  deletePods(pods: PodKey[], force: boolean): Promise<PodDeleteResult[]>
  /**
   * Adds an interactive ephemeral container and resolves once it runs (live:
   * up to a minute, rejected with `conflict` when it cannot start); attach to
   * it with the pod-exec tab kind.
   */
  addDebugContainer(namespace: string, pod: string, options: { image?: string; target?: string }): Promise<DebugContainer>
  /** Creates a Job from the CronJob now. */
  triggerCronJob(namespace: string, name: string): Promise<CronJobRun>
  suspendCronJob(namespace: string, name: string, suspend: boolean): Promise<void>
  createNamespace(name: string, labels: Record<string, string>): Promise<void>
  /**
   * System namespaces and the one KubeBench runs in are refused with
   * `forbidden` unless `force` is set.
   */
  deleteNamespace(name: string, force: boolean): Promise<void>
}

/** Live updates (design §7.10): which resources of the active cluster change. */
export interface LiveProvider {
  /** Follows changes of `topics` (1 to 32 resources) until closed. */
  watch(topics: readonly string[], handlers: WatchHandlers): WatchStream
}
