import type { ReactNode } from 'react'
import type { KubeObject } from '@/data/types'
import { Badge, toneForStatus } from '@/shared/Badge'

/** Sidebar grouping for the resource tree. */
export type ResourceGroup =
  | 'Cluster'
  | 'Workloads'
  | 'Config'
  | 'Network'
  | 'Storage'
  | 'Access Control'

export interface ResourceColumn {
  header: string
  render: (o: KubeObject) => ReactNode
  sortValue?: (o: KubeObject) => string | number
  width?: string
}

export interface ResourceDescriptor {
  /** Backend catalog key. */
  key: string
  /** Kubernetes kind (YAML + detail). */
  kind: string
  label: string
  icon: string
  group: ResourceGroup
  namespaced: boolean
  columns: ResourceColumn[]
}

// --- field accessors -------------------------------------------------------

export const objName = (o: KubeObject) => o.metadata?.name ?? '—'
export const objNamespace = (o: KubeObject) => o.metadata?.namespace ?? ''

/** "5s"|"26m"|"92d" from a synthetic age (mock) or creationTimestamp (live). */
export function objAge(o: KubeObject): string {
  if (o.age) return o.age
  const ts = o.metadata?.creationTimestamp
  if (!ts) return '—'
  const ms = Date.now() - new Date(ts).getTime()
  const m = Math.floor(ms / 60000)
  if (m < 1) return `${Math.max(0, Math.floor(ms / 1000))}s`
  if (m < 60) return `${m}m`
  if (m < 48 * 60) return `${Math.floor(m / 60)}h`
  return `${Math.floor(m / (24 * 60))}d`
}

export function ageMinutes(age: string): number {
  let total = 0
  for (const [, num, unit] of age.matchAll(/(\d+)(mo|[smhdy])/g)) {
    const f: Record<string, number> = { s: 1 / 60, m: 1, h: 60, d: 1440, mo: 43200, y: 525600 }
    total += Number(num) * (f[unit] ?? 0)
  }
  return total
}

function get(o: KubeObject, ...path: string[]): unknown {
  let cur: unknown = o
  for (const p of path) {
    if (cur && typeof cur === 'object') cur = (cur as Record<string, unknown>)[p]
    else return undefined
  }
  return cur
}

const str = (o: KubeObject, ...path: string[]): string => {
  const v = get(o, ...path)
  return v == null ? '—' : String(v)
}
const num = (o: KubeObject, ...path: string[]): number => {
  const v = get(o, ...path)
  return typeof v === 'number' ? v : 0
}

// --- common columns --------------------------------------------------------

const nameCol: ResourceColumn = {
  header: 'Name',
  render: objName,
  sortValue: objName,
}
const nsCol: ResourceColumn = {
  header: 'Namespace',
  render: objNamespace,
  sortValue: objNamespace,
}
const ageCol: ResourceColumn = {
  header: 'Age',
  render: objAge,
  sortValue: (o) => ageMinutes(objAge(o)),
  width: '80px',
}

const keysBadge = (o: KubeObject, field: string): ReactNode => {
  const data = get(o, field) as Record<string, unknown> | undefined
  const keys = data ? Object.keys(data) : []
  return keys.length ? keys.join(', ') : '—'
}

function withCommon(namespaced: boolean, mid: ResourceColumn[]): ResourceColumn[] {
  return namespaced ? [nameCol, nsCol, ...mid, ageCol] : [nameCol, ...mid, ageCol]
}

// --- catalog ---------------------------------------------------------------

export const RESOURCE_CATALOG: ResourceDescriptor[] = [
  // Config
  {
    key: 'configmaps', kind: 'ConfigMap', label: 'ConfigMaps', icon: 'symbol-key', group: 'Config', namespaced: true,
    columns: withCommon(true, [{ header: 'Keys', render: (o) => keysBadge(o, 'data') }]),
  },
  {
    key: 'secrets', kind: 'Secret', label: 'Secrets', icon: 'lock', group: 'Config', namespaced: true,
    columns: withCommon(true, [
      { header: 'Type', render: (o) => str(o, 'type'), sortValue: (o) => str(o, 'type') },
      { header: 'Keys', render: (o) => keysBadge(o, 'data') },
    ]),
  },
  // Workloads
  {
    key: 'statefulsets', kind: 'StatefulSet', label: 'StatefulSets', icon: 'database', group: 'Workloads', namespaced: true,
    columns: withCommon(true, [
      { header: 'Ready', render: (o) => `${num(o, 'status', 'readyReplicas')}/${num(o, 'spec', 'replicas')}` },
    ]),
  },
  {
    key: 'daemonsets', kind: 'DaemonSet', label: 'DaemonSets', icon: 'server-process', group: 'Workloads', namespaced: true,
    columns: withCommon(true, [
      { header: 'Ready', render: (o) => `${num(o, 'status', 'numberReady')}/${num(o, 'status', 'desiredNumberScheduled')}` },
    ]),
  },
  {
    key: 'replicasets', kind: 'ReplicaSet', label: 'ReplicaSets', icon: 'copy', group: 'Workloads', namespaced: true,
    columns: withCommon(true, [
      { header: 'Ready', render: (o) => `${num(o, 'status', 'readyReplicas')}/${num(o, 'status', 'replicas')}` },
    ]),
  },
  {
    key: 'jobs', kind: 'Job', label: 'Jobs', icon: 'checklist', group: 'Workloads', namespaced: true,
    columns: withCommon(true, [
      { header: 'Completions', render: (o) => `${num(o, 'status', 'succeeded')}/${num(o, 'spec', 'completions')}` },
    ]),
  },
  {
    key: 'cronjobs', kind: 'CronJob', label: 'CronJobs', icon: 'clock', group: 'Workloads', namespaced: true,
    columns: withCommon(true, [
      { header: 'Schedule', render: (o) => str(o, 'spec', 'schedule') },
      { header: 'Suspend', render: (o) => String(get(o, 'spec', 'suspend') ?? false) },
    ]),
  },
  // Network
  {
    key: 'ingresses', kind: 'Ingress', label: 'Ingresses', icon: 'globe', group: 'Network', namespaced: true,
    columns: withCommon(true, [
      { header: 'Class', render: (o) => str(o, 'spec', 'ingressClassName') },
      {
        header: 'Hosts',
        render: (o) => {
          const rules = (get(o, 'spec', 'rules') as { host?: string }[] | undefined) ?? []
          return rules.map((r) => r.host).filter(Boolean).join(', ') || '—'
        },
      },
    ]),
  },
  {
    key: 'endpoints', kind: 'Endpoints', label: 'Endpoints', icon: 'plug', group: 'Network', namespaced: true,
    columns: withCommon(true, [
      {
        header: 'Addresses',
        render: (o) => {
          const subsets = (get(o, 'subsets') as { addresses?: { ip?: string }[] }[] | undefined) ?? []
          const ips = subsets.flatMap((s) => (s.addresses ?? []).map((a) => a.ip))
          return ips.length ? `${ips.length} address(es)` : '—'
        },
      },
    ]),
  },
  {
    key: 'networkpolicies', kind: 'NetworkPolicy', label: 'Network Policies', icon: 'shield', group: 'Network', namespaced: true,
    columns: withCommon(true, [
      {
        header: 'Policy Types',
        render: (o) => ((get(o, 'spec', 'policyTypes') as string[] | undefined) ?? []).join(', ') || '—',
      },
    ]),
  },
  // Storage
  {
    key: 'persistentvolumeclaims', kind: 'PersistentVolumeClaim', label: 'Persistent Volume Claims', icon: 'save', group: 'Storage', namespaced: true,
    columns: withCommon(true, [
      { header: 'Status', render: (o) => <Badge tone={toneForStatus(str(o, 'status', 'phase'))}>{str(o, 'status', 'phase')}</Badge> },
      { header: 'Capacity', render: (o) => str(o, 'status', 'capacity', 'storage') },
      { header: 'Storage Class', render: (o) => str(o, 'spec', 'storageClassName') },
    ]),
  },
  {
    key: 'persistentvolumes', kind: 'PersistentVolume', label: 'Persistent Volumes', icon: 'save-all', group: 'Storage', namespaced: false,
    columns: withCommon(false, [
      { header: 'Status', render: (o) => <Badge tone={toneForStatus(str(o, 'status', 'phase'))}>{str(o, 'status', 'phase')}</Badge> },
      { header: 'Capacity', render: (o) => str(o, 'spec', 'capacity', 'storage') },
      { header: 'Storage Class', render: (o) => str(o, 'spec', 'storageClassName') },
    ]),
  },
  {
    key: 'storageclasses', kind: 'StorageClass', label: 'Storage Classes', icon: 'layers', group: 'Storage', namespaced: false,
    columns: withCommon(false, [
      { header: 'Provisioner', render: (o) => str(o, 'provisioner') },
      { header: 'Reclaim', render: (o) => str(o, 'reclaimPolicy') },
      { header: 'Binding', render: (o) => str(o, 'volumeBindingMode') },
    ]),
  },
  // Access Control
  {
    key: 'serviceaccounts', kind: 'ServiceAccount', label: 'Service Accounts', icon: 'account', group: 'Access Control', namespaced: true,
    columns: withCommon(true, []),
  },
  {
    key: 'roles', kind: 'Role', label: 'Roles', icon: 'shield', group: 'Access Control', namespaced: true,
    columns: withCommon(true, []),
  },
  {
    key: 'rolebindings', kind: 'RoleBinding', label: 'Role Bindings', icon: 'references', group: 'Access Control', namespaced: true,
    columns: withCommon(true, [{ header: 'Role', render: (o) => str(o, 'roleRef', 'name') }]),
  },
  {
    key: 'clusterroles', kind: 'ClusterRole', label: 'Cluster Roles', icon: 'shield', group: 'Access Control', namespaced: false,
    columns: withCommon(false, []),
  },
  {
    key: 'clusterrolebindings', kind: 'ClusterRoleBinding', label: 'Cluster Role Bindings', icon: 'references', group: 'Access Control', namespaced: false,
    columns: withCommon(false, [{ header: 'Role', render: (o) => str(o, 'roleRef', 'name') }]),
  },
  // Cluster
  {
    key: 'events', kind: 'Event', label: 'Events', icon: 'bell', group: 'Cluster', namespaced: true,
    columns: [
      { header: 'Type', render: (o) => <Badge tone={str(o, 'type') === 'Warning' ? 'warning' : 'neutral'}>{str(o, 'type')}</Badge>, sortValue: (o) => str(o, 'type') },
      { header: 'Reason', render: (o) => str(o, 'reason') },
      { header: 'Namespace', render: objNamespace, sortValue: objNamespace },
      { header: 'Message', render: (o) => str(o, 'message'), width: '40%' },
      { header: 'Count', render: (o) => str(o, 'count') },
      ageCol,
    ],
  },
]

export const catalogByKey = new Map(RESOURCE_CATALOG.map((d) => [d.key, d]))
