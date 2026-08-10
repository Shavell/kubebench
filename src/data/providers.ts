import type {
  ClusterStats,
  CrdDefinition,
  CustomResource,
  Deployment,
  HelmRelease,
  HelmRepo,
  K8sNode,
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
}

export interface CrdProvider {
  listCrds(): Promise<CrdDefinition[]>
  listCustomResources(crdName: string): Promise<CustomResource[]>
}

export interface HelmProvider {
  listReleases(): Promise<HelmRelease[]>
  listRepositories(): Promise<HelmRepo[]>
  getReleaseValuesYaml(name: string): Promise<string>
}

export interface MetricsProvider {
  clusterOverview(): Promise<ClusterStats>
  nodeSeries(node: string, metric: 'cpu' | 'memory', minutes: number): Promise<MetricPoint[]>
  clusterSeries(metric: 'cpu' | 'memory', minutes: number): Promise<MetricPoint[]>
}

export interface DataProviders {
  cluster: ClusterProvider
  crd: CrdProvider
  helm: HelmProvider
  metrics: MetricsProvider
}
