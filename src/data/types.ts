/** Domain types shaped after the Kubernetes API, trimmed to what the UI shows. */

export interface K8sNode {
  name: string
  roles: string[]
  status: 'Ready' | 'NotReady'
  version: string
  os: string
  cpuCapacity: number
  memoryCapacityGi: number
  age: string
  internalIP: string
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
  labels: Record<string, string>
  containers: { name: string; image: string }[]
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
