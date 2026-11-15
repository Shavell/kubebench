import { useState } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import type { Deployment, K8sNode, Namespace, Pod, Service } from '@/data/types'
import { formatNumber } from '@/i18n/format'
import { Badge } from '@/shared/Badge'
import { ListFilterBar } from '@/shared/ListFilterBar'
import { NamespaceOptions } from '@/shared/NamespaceOptions'
import { Table } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { EmptyState } from '@/shared/EmptyState'
import { Unavailable } from '@/shared/Unavailable'
import { tw, useWorkloadsTranslation } from '../i18n'
import { richLabel, withHeaders, type ColumnSpec } from '../resourceCatalog'
import { DeletePodDialog } from '../DeletePodDialog'
import { ResourceDialogs } from '../editing/ResourceDialogs'
import { NewResourceButton } from '../editing/ResourceEditButtons'
import { deploymentMenu, genericMenu, podMenu } from '../rowActions'
import { ListOpsActions } from '../ops/ListOpsActions'
import { NodeStatus } from '../ops/NodeScheduling'
import { usePodSelection } from '../ops/usePodSelection'
import { WorkloadOpsDialogs } from '../ops/WorkloadOpsDialogs'
import '@/shared/tabPage.css'

function openDetail(kind: string, namespace: string | null, name: string, icon: string) {
  useLayoutStore.getState().openTab({
    id: `${kind.toLowerCase()}:${namespace ?? '-'}/${name}`,
    title: name,
    icon,
    kind: 'workloads-detail',
    props: { kind, namespace, name },
  })
}

/** "5s" | "26m" | "23h" | "92d" | "2y1mo" → minutes, for correct age sorting. */
function ageMinutes(age: string): number {
  let total = 0
  for (const [, num, unit] of age.matchAll(/(\d+)(mo|[smhdy])/g)) {
    const factors: Record<string, number> = { s: 1 / 60, m: 1, h: 60, d: 1440, mo: 43200, y: 525600 }
    total += Number(num) * (factors[unit] ?? 0)
  }
  return total
}

const PODS_STATUSES = ['Running', 'Pending', 'CrashLoopBackOff', 'ImagePullBackOff', 'Failed', 'Succeeded']

// Column keys double as i18n keys (`columns.<key>`); see withHeaders.
const podColumns: ColumnSpec<Pod>[] = [
  { key: 'name', render: (p) => p.name, sortValue: (p) => p.name },
  { key: 'namespace', render: (p) => p.namespace, sortValue: (p) => p.namespace },
  {
    key: 'ready',
    render: (p) => `${p.readyContainers}/${p.totalContainers}`,
    sortValue: (p) => p.readyContainers / Math.max(1, p.totalContainers),
  },
  { key: 'status', render: (p) => <Badge>{p.status}</Badge>, sortValue: (p) => p.status },
  { key: 'restarts', render: (p) => String(p.restarts), sortValue: (p) => p.restarts },
  { key: 'age', render: (p) => p.age, sortValue: (p) => ageMinutes(p.age) },
  { key: 'node', render: (p) => p.node || '—', sortValue: (p) => p.node },
]

const nodeColumns: ColumnSpec<K8sNode>[] = [
  { key: 'name', render: (n) => n.name, sortValue: (n) => n.name },
  { key: 'status', render: (n) => <NodeStatus node={n} />, sortValue: (n) => n.status },
  { key: 'roles', render: (n) => n.roles.join(', ') },
  { key: 'version', render: (n) => n.version },
  {
    key: 'cpu',
    render: (n) => tw('values.cores', { count: n.cpuCapacity, value: formatNumber(n.cpuCapacity) }),
    sortValue: (n) => n.cpuCapacity,
  },
  {
    key: 'memory',
    render: (n) => `${formatNumber(n.memoryCapacityGi)} Gi`,
    sortValue: (n) => n.memoryCapacityGi,
  },
  { key: 'age', render: (n) => n.age, sortValue: (n) => ageMinutes(n.age) },
  { key: 'internalIP', render: (n) => n.internalIP },
]

const namespaceColumns: ColumnSpec<Namespace>[] = [
  { key: 'name', render: (ns) => ns.name, sortValue: (ns) => ns.name },
  { key: 'status', render: (ns) => <Badge>{ns.status}</Badge>, sortValue: (ns) => ns.status },
  { key: 'age', render: (ns) => ns.age, sortValue: (ns) => ageMinutes(ns.age) },
]

const deploymentColumns: ColumnSpec<Deployment>[] = [
  { key: 'name', render: (d) => d.name, sortValue: (d) => d.name },
  { key: 'namespace', render: (d) => d.namespace, sortValue: (d) => d.namespace },
  {
    key: 'ready',
    render: (d) => (
      <span className={d.readyReplicas < d.replicas ? 'text-warning' : undefined}>
        {d.readyReplicas}/{d.replicas}
      </span>
    ),
    sortValue: (d) => d.readyReplicas / Math.max(1, d.replicas),
  },
  { key: 'upToDate', render: (d) => String(d.upToDate), sortValue: (d) => d.upToDate },
  { key: 'available', render: (d) => String(d.available), sortValue: (d) => d.available },
  { key: 'age', render: (d) => d.age, sortValue: (d) => ageMinutes(d.age) },
]

const serviceColumns: ColumnSpec<Service>[] = [
  { key: 'name', render: (s) => s.name, sortValue: (s) => s.name },
  { key: 'namespace', render: (s) => s.namespace, sortValue: (s) => s.namespace },
  { key: 'type', render: (s) => s.type, sortValue: (s) => s.type },
  { key: 'clusterIP', render: (s) => s.clusterIP },
  { key: 'externalIP', render: (s) => s.externalIP ?? '—' },
  { key: 'ports', render: (s) => s.ports },
  { key: 'age', render: (s) => s.age, sortValue: (s) => ageMinutes(s.age) },
]

const NAMESPACED = ['Pods', 'Deployments', 'Services']

/** The kind of each namespaced list, for its New button's template. */
const KINDS: Record<string, string> = { Pods: 'Pod', Deployments: 'Deployment', Services: 'Service' }

export function ResourceListPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useWorkloadsTranslation()
  const resource = tab.props?.resource as string
  const providers = useProviders()
  const globalNamespace = useContextStore((s) => s.namespace)
  const [search, setSearch] = useState('')
  /** Page-local namespace filter; empty = follow the global status bar filter. Palette `-n` presets it. */
  const [localNs, setLocalNs] = useState((tab.props?.namespace as string | undefined) ?? '')
  const [statusFilter, setStatusFilter] = useState('')
  const [deleting, setDeleting] = useState<Pod | null>(null)
  const podSelection = usePodSelection()

  const isNamespaced = NAMESPACED.includes(resource)
  const nsFilter = localNs || (globalNamespace === ALL_NAMESPACES ? undefined : globalNamespace)
  const label = richLabel(resource)

  const { data: namespaces } = useProviderQuery((p) => p.cluster.listNamespaces(), [], { topics: ['namespaces'] })

  type AnyRow = Pod | K8sNode | Namespace | Deployment | Service
  const query = useProviderQuery<AnyRow[]>(
    (p) => {
      switch (resource) {
        case 'Pods':
          return p.cluster.listPods(nsFilter)
        case 'Deployments':
          return p.cluster.listDeployments(nsFilter)
        case 'Services':
          return p.cluster.listServices(nsFilter)
        case 'Nodes':
          return p.cluster.listNodes()
        case 'Namespaces':
          return p.cluster.listNamespaces()
        default:
          return Promise.resolve([])
      }
    },
    [resource, nsFilter],
    // The resources of this page ('Pods', …) are their watch topics in lower case.
    { topics: [resource.toLowerCase()], namespace: nsFilter },
  )

  if (query.loading && !query.data) {
    return <div className="page-loading">{t('list.loading', { resource: label })}</div>
  }

  const all = query.data ?? []
  const rows = all.filter((row) => {
    if (search && !row.name.toLowerCase().includes(search.toLowerCase())) return false
    if (statusFilter && resource === 'Pods' && (row as Pod).status !== statusFilter) return false
    return true
  })

  const scopedTitle = isNamespaced && nsFilter ? `${label} · ${nsFilter}` : label

  const filterBar = (
    <ListFilterBar
      search={search}
      onSearch={setSearch}
      filtersActive={Boolean(localNs || statusFilter)}
      summary={t('list.summary', { shown: formatNumber(rows.length), total: formatNumber(all.length) })}
    >
      {isNamespaced && (
        <select
          title={t('filters.namespaceTitle')}
          value={localNs}
          onChange={(e) => setLocalNs(e.target.value)}
        >
          <option value="">
            {globalNamespace === ALL_NAMESPACES
              ? t('filters.namespaceAll')
              : t('filters.namespaceGlobal', { namespace: globalNamespace })}
          </option>
          <NamespaceOptions names={(namespaces ?? []).map((ns) => ns.name)} selected={localNs} />
        </select>
      )}
      {resource === 'Pods' && (
        <select
          title={t('filters.statusTitle')}
          value={statusFilter}
          onChange={(e) => setStatusFilter(e.target.value)}
        >
          <option value="">{t('filters.statusAll')}</option>
          {PODS_STATUSES.map((s) => (
            <option key={s} value={s}>
              {s}
            </option>
          ))}
        </select>
      )}
    </ListFilterBar>
  )

  const body = () => {
    // A failed request must not pass for an empty list (live mode never shows stand-in data).
    if (query.error) {
      return (
        <Unavailable
          code={query.errorCode}
          reason={query.error}
          title={t('list.loadFailed', { resource: label })}
        />
      )
    }
    if (rows.length === 0) {
      return (
        <EmptyState
          icon="inbox"
          title={t('list.empty', { resource: label })}
          hint={search || statusFilter || nsFilter ? t('list.clearFiltersHint') : undefined}
        />
      )
    }
    return (
      <div className="page__scroll">
        {resource === 'Pods' && (
          <Table
            columns={withHeaders(podColumns)}
            rows={rows as Pod[]}
            rowKey={(p) => `${p.namespace}/${p.name}`}
            onRowClick={(p) => openDetail('Pod', p.namespace, p.name, 'package')}
            onRowContextMenu={(p, e) => podMenu(e, p, providers, setDeleting)}
            selection={podSelection.table}
          />
        )}
        {resource === 'Nodes' && (
          <Table
            columns={withHeaders(nodeColumns)}
            rows={rows as K8sNode[]}
            rowKey={(n) => n.name}
            onRowClick={(n) => openDetail('Node', null, n.name, 'server')}
            onRowContextMenu={(n, e) => genericMenu(e, { kind: 'Node', apiVersion: 'v1', name: n.name }, providers, n)}
          />
        )}
        {resource === 'Namespaces' && (
          <Table
            columns={withHeaders(namespaceColumns)}
            rows={rows as Namespace[]}
            rowKey={(ns) => ns.name}
            onRowContextMenu={(ns, e) =>
              genericMenu(e, { kind: 'Namespace', apiVersion: 'v1', name: ns.name }, providers, ns)
            }
          />
        )}
        {resource === 'Deployments' && (
          <Table
            columns={withHeaders(deploymentColumns)}
            rows={rows as Deployment[]}
            rowKey={(d) => `${d.namespace}/${d.name}`}
            onRowClick={(d) => openDetail('Deployment', d.namespace, d.name, 'rocket')}
            onRowContextMenu={(d, e) => deploymentMenu(e, d, providers)}
          />
        )}
        {resource === 'Services' && (
          <Table
            columns={withHeaders(serviceColumns)}
            rows={rows as Service[]}
            rowKey={(s) => `${s.namespace}/${s.name}`}
            onRowClick={(s) => openDetail('Service', s.namespace, s.name, 'plug')}
            onRowContextMenu={(s, e) =>
              genericMenu(e, { kind: 'Service', apiVersion: 'v1', namespace: s.namespace, name: s.name }, providers)
            }
          />
        )}
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={scopedTitle}>
        {filterBar}
        {isNamespaced && <NewResourceButton kind={KINDS[resource]} namespace={nsFilter} />}
        <ListOpsActions resource={resource} rows={rows} selection={podSelection} />
      </Toolbar>
      {body()}
      <ResourceDialogs />
      <DeletePodDialog pod={deleting} onClose={() => setDeleting(null)} />
      <WorkloadOpsDialogs />
    </div>
  )
}
