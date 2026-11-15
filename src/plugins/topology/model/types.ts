import type { KubeObject } from '@/data/types'

/** The kinds the topology shows (design §7.9). */
export type TopologyKind =
  | 'Ingress'
  | 'Service'
  | 'Deployment'
  | 'ReplicaSet'
  | 'StatefulSet'
  | 'DaemonSet'
  | 'Pod'
  | 'PersistentVolumeClaim'
  | 'PersistentVolume'
  | 'ConfigMap'
  | 'Secret'

/** How a node is coloured: healthy, degraded, failing, informational, or referenced but absent. */
export type NodeTone = 'ok' | 'warning' | 'error' | 'neutral' | 'missing'

/** A node's status line: an i18n key of this plugin with its values. */
export interface NodeStatus {
  tone: NodeTone
  key: string
  values?: Record<string, string | number>
}

export interface TopologyNode {
  /** `<kind>/<namespace>/<name>` (namespace empty for PersistentVolumes). */
  id: string
  kind: TopologyKind
  namespace: string
  name: string
  status: NodeStatus
  /** Referenced by another object but not found in its list. */
  missing: boolean
}

/**
 * Edges point from the object that refers to the other one: an Ingress
 * routes to a Service, a Service selects Pods, an owner owns its children,
 * a Pod mounts a PVC, a PVC binds a PV, a Pod or Ingress references a
 * ConfigMap or Secret.
 */
export type EdgeKind = 'routes' | 'selects' | 'owns' | 'mounts' | 'binds' | 'references'

export interface TopologyEdge {
  /** `<from>><to>`; one edge per pair. */
  id: string
  from: string
  to: string
  kind: EdgeKind
}

export interface TopologyGraph {
  nodes: TopologyNode[]
  edges: TopologyEdge[]
}

/**
 * The lists of one namespace the graph is built from. A list that is
 * `undefined` has not loaded (yet, or failed): nothing it would contain is
 * reported missing, and statuses that depend on it read as unknown.
 */
export interface TopologySnapshot {
  ingresses?: KubeObject[]
  services?: KubeObject[]
  deployments?: KubeObject[]
  replicaSets?: KubeObject[]
  statefulSets?: KubeObject[]
  daemonSets?: KubeObject[]
  pods?: KubeObject[]
  pvcs?: KubeObject[]
  /** Cluster-wide; only the PVs bound to the namespace's claims are shown. */
  pvs?: KubeObject[]
  configMaps?: KubeObject[]
  secrets?: KubeObject[]
}

export interface BuildOptions {
  /** Show the ConfigMaps and Secrets Pods and Ingresses reference. */
  showConfig: boolean
  /** Show ReplicaSets scaled to zero without Pods (old Deployment revisions). */
  showInactiveReplicaSets: boolean
}

export const nodeId = (kind: TopologyKind, namespace: string, name: string) => `${kind}/${namespace}/${name}`
