import type {
  ClusterEntry,
  ClusterStats,
  K8sEvent,
  CrdDefinition,
  CustomResource,
  Deployment,
  HelmRelease,
  HelmRepo,
  K8sNode,
  MetricKind,
  MetricPoint,
  Namespace,
  Pod,
  Service,
} from './types'

/**
 * The contract between the UI and any data source. Today these are
 * implemented by mocks (src/data/mock); the real implementation will
 * call the Kubernetes API / Helm / Prometheus without touching the UI.
 * Everything is async so the UI already handles the eventual API shape.
 */

export interface ClusterProvider {
  listNodes(): Promise<K8sNode[]>
  listNamespaces(): Promise<Namespace[]>
  listPods(namespace?: string): Promise<Pod[]>
  listDeployments(namespace?: string): Promise<Deployment[]>
  listServices(namespace?: string): Promise<Service[]>
  getResourceYaml(kind: string, namespace: string | null, name: string): Promise<string>
  listEvents(kind: string, namespace: string | null, name: string): Promise<K8sEvent[]>
  getPodLogs(namespace: string, pod: string, container: string): Promise<string[]>
  /** Mutations are visual-only: they mutate the mock dataset in memory. */
  deletePod(namespace: string, name: string): Promise<void>
  scaleDeployment(namespace: string, name: string, replicas: number): Promise<void>
  restartDeployment(namespace: string, name: string): Promise<void>
}

export interface CrdProvider {
  listCrds(): Promise<CrdDefinition[]>
  listCustomResources(crdName: string): Promise<CustomResource[]>
}

export interface HelmProvider {
  listReleases(): Promise<HelmRelease[]>
  listRepositories(): Promise<HelmRepo[]>
  getReleaseValuesYaml(name: string): Promise<string>
  /** Creates a new revision that restores the given one, like `helm rollback`. */
  rollbackRelease(name: string, revision: number): Promise<void>
  uninstallRelease(name: string): Promise<void>
}

export interface MetricsProvider {
  clusterOverview(): Promise<ClusterStats>
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
  addCluster(entry: { name: string; kubeconfigPath?: string; address?: string }): Promise<void>
  removeCluster(name: string): Promise<void>
  /** Switches the backend to another cluster; the app reloads afterwards. */
  activateCluster(name: string): Promise<void>
}

export interface DataProviders {
  cluster: ClusterProvider
  crd: CrdProvider
  helm: HelmProvider
  metrics: MetricsProvider
  inventory: InventoryProvider
}
