import type { ReactNode } from 'react'
import { dataMode } from '@/data/mode'
import type { KubeObject } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { toneForStatus } from '@/shared/badgeTone'
import type { Column } from '@/shared/Table'
import { tw } from './i18n'
import { openShiftResources } from './openshiftCatalog'

/** Sidebar grouping for the resource tree; the label is the `groups.<id>` key. */
export type ResourceGroup = 'cluster' | 'workloads' | 'config' | 'network' | 'storage' | 'accessControl' | 'openshift'

/** A table column without its header: the header is the `columns.<key>` i18n key. */
export type ColumnSpec<T> = Omit<Column<T>, 'header'>

export type ResourceColumn = ColumnSpec<KubeObject>

export interface ResourceDescriptor {
  /** Backend catalog key; the display label is the `resources.<key>` i18n key. */
  key: string
  /** Kubernetes kind (YAML + detail). */
  kind: string
  icon: string
  group: ResourceGroup
  namespaced: boolean
  columns: ResourceColumn[]
  /** Extra rows of the detail page's summary, after the list columns. */
  details?: ResourceColumn[]
  /**
   * Served by some clusters only (OpenShift's types, design §7.15): listed
   * when the cluster reports it in capabilities.optionalResources.
   */
  optional?: boolean
  /** Live-update topic when the key alone is ambiguous (default: the key). */
  topic?: string
}

/** Whether the active cluster serves a catalog type (always, unless it is optional). */
export function isServed(d: ResourceDescriptor): boolean {
  return !d.optional || (dataMode.capabilities?.optionalResources ?? []).includes(d.key)
}

/** The live-update topic of a catalog type's objects. */
export const topicOf = (d: ResourceDescriptor | undefined, key: string): string => d?.topic ?? key

/** Translated, plural display name of a resource list (`resources.<key>`). */
export function resourceLabel(key: string): string {
  return tw(`resources.${key}`)
}

/** Translated label of a rich list tab ("Pods" → `resources.pods`). */
export function richLabel(resource: string): string {
  return resourceLabel(resource.toLowerCase())
}

/** Table columns with their headers translated from `columns.<key>`. */
export function withHeaders<T>(specs: ColumnSpec<T>[]): Column<T>[] {
  return specs.map((spec) => ({ ...spec, header: tw(`columns.${spec.key}`) }))
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
const yesNo = (value: unknown): string => tw(value ? 'common.yes' : 'common.no')

// --- common columns --------------------------------------------------------

const nameCol: ResourceColumn = {
  key: 'name',
  render: objName,
  sortValue: objName,
}
const nsCol: ResourceColumn = {
  key: 'namespace',
  render: objNamespace,
  sortValue: objNamespace,
}
const ageCol: ResourceColumn = {
  key: 'age',
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
    key: 'configmaps', kind: 'ConfigMap', icon: 'symbol-key', group: 'config', namespaced: true,
    columns: withCommon(true, [{ key: 'keys', render: (o) => keysBadge(o, 'data') }]),
  },
  {
    key: 'secrets', kind: 'Secret', icon: 'lock', group: 'config', namespaced: true,
    columns: withCommon(true, [
      { key: 'type', render: (o) => str(o, 'type'), sortValue: (o) => str(o, 'type') },
      { key: 'keys', render: (o) => keysBadge(o, 'data') },
    ]),
  },
  {
    key: 'resourcequotas', kind: 'ResourceQuota', icon: 'pie-chart', group: 'config', namespaced: true,
    columns: withCommon(true, [{ key: 'scopes', render: (o) => ((get(o, 'spec', 'scopes') as string[] | undefined) ?? []).join(', ') || '—' }]),
  },
  {
    key: 'limitranges', kind: 'LimitRange', icon: 'symbol-ruler', group: 'config', namespaced: true,
    columns: withCommon(true, []),
  },
  {
    key: 'horizontalpodautoscalers', kind: 'HorizontalPodAutoscaler', icon: 'graph', group: 'config', namespaced: true,
    columns: withCommon(true, [
      { key: 'reference', render: (o) => `${str(o, 'spec', 'scaleTargetRef', 'kind')}/${str(o, 'spec', 'scaleTargetRef', 'name')}` },
      { key: 'minMax', render: (o) => `${num(o, 'spec', 'minReplicas')} / ${num(o, 'spec', 'maxReplicas')}` },
      { key: 'replicas', render: (o) => str(o, 'status', 'currentReplicas') },
    ]),
  },
  {
    key: 'poddisruptionbudgets', kind: 'PodDisruptionBudget', icon: 'shield', group: 'config', namespaced: true,
    columns: withCommon(true, [
      { key: 'minAvailable', render: (o) => str(o, 'spec', 'minAvailable') },
      { key: 'maxUnavailable', render: (o) => str(o, 'spec', 'maxUnavailable') },
      { key: 'healthy', render: (o) => str(o, 'status', 'currentHealthy') },
    ]),
  },
  {
    key: 'priorityclasses', kind: 'PriorityClass', icon: 'list-ordered', group: 'config', namespaced: false,
    columns: withCommon(false, [
      { key: 'value', render: (o) => str(o, 'value'), sortValue: (o) => num(o, 'value') },
      { key: 'globalDefault', render: (o) => yesNo(get(o, 'globalDefault')) },
    ]),
  },
  {
    key: 'runtimeclasses', kind: 'RuntimeClass', icon: 'server-process', group: 'config', namespaced: false,
    columns: withCommon(false, [{ key: 'handler', render: (o) => str(o, 'handler') }]),
  },
  {
    key: 'leases', kind: 'Lease', icon: 'watch', group: 'config', namespaced: true,
    columns: withCommon(true, [{ key: 'holder', render: (o) => str(o, 'spec', 'holderIdentity') }]),
  },
  {
    key: 'mutatingwebhookconfigurations', kind: 'MutatingWebhookConfiguration', icon: 'symbol-event', group: 'config', namespaced: false,
    columns: withCommon(false, [{ key: 'webhooks', render: (o) => String(((get(o, 'webhooks') as unknown[]) ?? []).length) }]),
  },
  {
    key: 'validatingwebhookconfigurations', kind: 'ValidatingWebhookConfiguration', icon: 'symbol-event', group: 'config', namespaced: false,
    columns: withCommon(false, [{ key: 'webhooks', render: (o) => String(((get(o, 'webhooks') as unknown[]) ?? []).length) }]),
  },
  // Workloads
  {
    key: 'statefulsets', kind: 'StatefulSet', icon: 'database', group: 'workloads', namespaced: true,
    columns: withCommon(true, [
      { key: 'ready', render: (o) => `${num(o, 'status', 'readyReplicas')}/${num(o, 'spec', 'replicas')}` },
    ]),
  },
  {
    key: 'daemonsets', kind: 'DaemonSet', icon: 'server-process', group: 'workloads', namespaced: true,
    columns: withCommon(true, [
      { key: 'ready', render: (o) => `${num(o, 'status', 'numberReady')}/${num(o, 'status', 'desiredNumberScheduled')}` },
    ]),
  },
  {
    key: 'replicasets', kind: 'ReplicaSet', icon: 'copy', group: 'workloads', namespaced: true,
    columns: withCommon(true, [
      { key: 'ready', render: (o) => `${num(o, 'status', 'readyReplicas')}/${num(o, 'status', 'replicas')}` },
    ]),
  },
  {
    key: 'jobs', kind: 'Job', icon: 'checklist', group: 'workloads', namespaced: true,
    columns: withCommon(true, [
      { key: 'completions', render: (o) => `${num(o, 'status', 'succeeded')}/${num(o, 'spec', 'completions')}` },
    ]),
  },
  {
    key: 'cronjobs', kind: 'CronJob', icon: 'clock', group: 'workloads', namespaced: true,
    columns: withCommon(true, [
      { key: 'schedule', render: (o) => str(o, 'spec', 'schedule') },
      { key: 'suspend', render: (o) => yesNo(get(o, 'spec', 'suspend')) },
    ]),
  },
  {
    key: 'replicationcontrollers', kind: 'ReplicationController', icon: 'copy', group: 'workloads', namespaced: true,
    columns: withCommon(true, [
      { key: 'ready', render: (o) => `${num(o, 'status', 'readyReplicas')}/${num(o, 'spec', 'replicas')}` },
    ]),
  },
  // Network
  {
    key: 'ingresses', kind: 'Ingress', icon: 'globe', group: 'network', namespaced: true,
    columns: withCommon(true, [
      { key: 'class', render: (o) => str(o, 'spec', 'ingressClassName') },
      {
        key: 'hosts',
        render: (o) => {
          const rules = (get(o, 'spec', 'rules') as { host?: string }[] | undefined) ?? []
          return rules.map((r) => r.host).filter(Boolean).join(', ') || '—'
        },
      },
    ]),
  },
  {
    key: 'ingressclasses', kind: 'IngressClass', icon: 'globe', group: 'network', namespaced: false,
    columns: withCommon(false, [{ key: 'controller', render: (o) => str(o, 'spec', 'controller') }]),
  },
  {
    key: 'endpointslices', kind: 'EndpointSlice', icon: 'plug', group: 'network', namespaced: true,
    columns: withCommon(true, [
      { key: 'addressType', render: (o) => str(o, 'addressType') },
      { key: 'endpoints', render: (o) => String(((get(o, 'endpoints') as unknown[]) ?? []).length) },
    ]),
  },
  {
    key: 'endpoints', kind: 'Endpoints', icon: 'plug', group: 'network', namespaced: true,
    columns: withCommon(true, [
      {
        key: 'addresses',
        render: (o) => {
          const subsets = (get(o, 'subsets') as { addresses?: { ip?: string }[] }[] | undefined) ?? []
          const ips = subsets.flatMap((s) => (s.addresses ?? []).map((a) => a.ip))
          return ips.length ? tw('values.addresses', { count: ips.length }) : '—'
        },
      },
    ]),
  },
  {
    key: 'networkpolicies', kind: 'NetworkPolicy', icon: 'shield', group: 'network', namespaced: true,
    columns: withCommon(true, [
      {
        key: 'policyTypes',
        render: (o) => ((get(o, 'spec', 'policyTypes') as string[] | undefined) ?? []).join(', ') || '—',
      },
    ]),
  },
  // Storage
  {
    key: 'persistentvolumeclaims', kind: 'PersistentVolumeClaim', icon: 'save', group: 'storage', namespaced: true,
    columns: withCommon(true, [
      { key: 'status', render: (o) => <Badge tone={toneForStatus(str(o, 'status', 'phase'))}>{str(o, 'status', 'phase')}</Badge> },
      { key: 'capacity', render: (o) => str(o, 'status', 'capacity', 'storage') },
      { key: 'storageClass', render: (o) => str(o, 'spec', 'storageClassName') },
    ]),
  },
  {
    key: 'persistentvolumes', kind: 'PersistentVolume', icon: 'save-all', group: 'storage', namespaced: false,
    columns: withCommon(false, [
      { key: 'status', render: (o) => <Badge tone={toneForStatus(str(o, 'status', 'phase'))}>{str(o, 'status', 'phase')}</Badge> },
      { key: 'capacity', render: (o) => str(o, 'spec', 'capacity', 'storage') },
      { key: 'storageClass', render: (o) => str(o, 'spec', 'storageClassName') },
    ]),
  },
  {
    key: 'storageclasses', kind: 'StorageClass', icon: 'layers', group: 'storage', namespaced: false,
    columns: withCommon(false, [
      { key: 'provisioner', render: (o) => str(o, 'provisioner') },
      { key: 'reclaim', render: (o) => str(o, 'reclaimPolicy') },
      { key: 'binding', render: (o) => str(o, 'volumeBindingMode') },
    ]),
  },
  // Access Control
  {
    key: 'serviceaccounts', kind: 'ServiceAccount', icon: 'account', group: 'accessControl', namespaced: true,
    columns: withCommon(true, []),
  },
  {
    key: 'roles', kind: 'Role', icon: 'shield', group: 'accessControl', namespaced: true,
    columns: withCommon(true, []),
  },
  {
    key: 'rolebindings', kind: 'RoleBinding', icon: 'references', group: 'accessControl', namespaced: true,
    columns: withCommon(true, [{ key: 'role', render: (o) => str(o, 'roleRef', 'name') }]),
  },
  {
    key: 'clusterroles', kind: 'ClusterRole', icon: 'shield', group: 'accessControl', namespaced: false,
    columns: withCommon(false, []),
  },
  {
    key: 'clusterrolebindings', kind: 'ClusterRoleBinding', icon: 'references', group: 'accessControl', namespaced: false,
    columns: withCommon(false, [{ key: 'role', render: (o) => str(o, 'roleRef', 'name') }]),
  },
  // Cluster
  {
    // The detail page's summary; the Events view (pages/EventsListPage) has its own list columns.
    key: 'events', kind: 'Event', icon: 'bell', group: 'cluster', namespaced: true,
    columns: [
      { key: 'type', render: (o) => <Badge tone={str(o, 'type') === 'Warning' ? 'warning' : 'neutral'}>{str(o, 'type')}</Badge>, sortValue: (o) => str(o, 'type') },
      { key: 'reason', render: (o) => str(o, 'reason') },
      { key: 'namespace', render: objNamespace, sortValue: objNamespace },
      { key: 'message', render: (o) => str(o, 'message'), width: '40%' },
      { key: 'count', render: (o) => str(o, 'count') },
      ageCol,
    ],
  },
  // OpenShift (only where served)
  ...openShiftResources({ get, str, num, withCommon }),
]

export const catalogByKey = new Map(RESOURCE_CATALOG.map((d) => [d.key, d]))
