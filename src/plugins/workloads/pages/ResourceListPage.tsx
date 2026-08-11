import { useState } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useProviderQuery } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import type { Deployment, K8sNode, Namespace, Pod, Service } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { ListFilterBar } from '@/shared/ListFilterBar'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { EmptyState } from '@/shared/EmptyState'
import './pages.css'

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

const podColumns: Column<Pod>[] = [
  { key: 'name', header: 'Name', render: (p) => p.name, sortValue: (p) => p.name },
  { key: 'ns', header: 'Namespace', render: (p) => p.namespace, sortValue: (p) => p.namespace },
  {
    key: 'ready',
    header: 'Ready',
    render: (p) => `${p.readyContainers}/${p.totalContainers}`,
    sortValue: (p) => p.readyContainers / Math.max(1, p.totalContainers),
  },
  { key: 'status', header: 'Status', render: (p) => <Badge>{p.status}</Badge>, sortValue: (p) => p.status },
  { key: 'restarts', header: 'Restarts', render: (p) => String(p.restarts), sortValue: (p) => p.restarts },
  { key: 'age', header: 'Age', render: (p) => p.age, sortValue: (p) => ageMinutes(p.age) },
  { key: 'node', header: 'Node', render: (p) => p.node || '—', sortValue: (p) => p.node },
]

const nodeColumns: Column<K8sNode>[] = [
  { key: 'name', header: 'Name', render: (n) => n.name, sortValue: (n) => n.name },
  { key: 'status', header: 'Status', render: (n) => <Badge>{n.status}</Badge>, sortValue: (n) => n.status },
  { key: 'roles', header: 'Roles', render: (n) => n.roles.join(', ') },
  { key: 'version', header: 'Version', render: (n) => n.version },
  { key: 'cpu', header: 'CPU', render: (n) => `${n.cpuCapacity} cores`, sortValue: (n) => n.cpuCapacity },
  { key: 'mem', header: 'Memory', render: (n) => `${n.memoryCapacityGi} Gi`, sortValue: (n) => n.memoryCapacityGi },
  { key: 'age', header: 'Age', render: (n) => n.age, sortValue: (n) => ageMinutes(n.age) },
  { key: 'ip', header: 'Internal IP', render: (n) => n.internalIP },
]

const namespaceColumns: Column<Namespace>[] = [
  { key: 'name', header: 'Name', render: (ns) => ns.name, sortValue: (ns) => ns.name },
  { key: 'status', header: 'Status', render: (ns) => <Badge>{ns.status}</Badge>, sortValue: (ns) => ns.status },
  { key: 'age', header: 'Age', render: (ns) => ns.age, sortValue: (ns) => ageMinutes(ns.age) },
]

const deploymentColumns: Column<Deployment>[] = [
  { key: 'name', header: 'Name', render: (d) => d.name, sortValue: (d) => d.name },
  { key: 'ns', header: 'Namespace', render: (d) => d.namespace, sortValue: (d) => d.namespace },
  {
    key: 'ready',
    header: 'Ready',
    render: (d) => (
      <span className={d.readyReplicas < d.replicas ? 'text-warning' : undefined}>
        {d.readyReplicas}/{d.replicas}
      </span>
    ),
    sortValue: (d) => d.readyReplicas / Math.max(1, d.replicas),
  },
  { key: 'uptodate', header: 'Up-to-date', render: (d) => String(d.upToDate), sortValue: (d) => d.upToDate },
  { key: 'available', header: 'Available', render: (d) => String(d.available), sortValue: (d) => d.available },
  { key: 'age', header: 'Age', render: (d) => d.age, sortValue: (d) => ageMinutes(d.age) },
]

const serviceColumns: Column<Service>[] = [
  { key: 'name', header: 'Name', render: (s) => s.name, sortValue: (s) => s.name },
  { key: 'ns', header: 'Namespace', render: (s) => s.namespace, sortValue: (s) => s.namespace },
  { key: 'type', header: 'Type', render: (s) => s.type, sortValue: (s) => s.type },
  { key: 'clusterip', header: 'Cluster IP', render: (s) => s.clusterIP },
  { key: 'externalip', header: 'External IP', render: (s) => s.externalIP ?? '—' },
  { key: 'ports', header: 'Ports', render: (s) => s.ports },
  { key: 'age', header: 'Age', render: (s) => s.age, sortValue: (s) => ageMinutes(s.age) },
]

const NAMESPACED = ['Pods', 'Deployments', 'Services']

export function ResourceListPage({ tab }: { tab: TabDescriptor }) {
  const resource = tab.props?.resource as string
  const globalNamespace = useContextStore((s) => s.namespace)
  const [search, setSearch] = useState('')
  /** Page-local namespace filter; empty = follow the global status bar filter. */
  const [localNs, setLocalNs] = useState('')
  const [statusFilter, setStatusFilter] = useState('')

  const isNamespaced = NAMESPACED.includes(resource)
  const nsFilter = localNs || (globalNamespace === ALL_NAMESPACES ? undefined : globalNamespace)

  const { data: namespaces } = useProviderQuery((p) => p.cluster.listNamespaces(), [])

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
  )

  if (query.loading && !query.data) {
    return <div className="page-loading">Loading {resource}…</div>
  }

  const all = query.data ?? []
  const rows = all.filter((row) => {
    if (search && !row.name.toLowerCase().includes(search.toLowerCase())) return false
    if (statusFilter && resource === 'Pods' && (row as Pod).status !== statusFilter) return false
    return true
  })

  const scopedTitle = isNamespaced && nsFilter ? `${resource} · ${nsFilter}` : resource

  const filterBar = (
    <ListFilterBar
      search={search}
      onSearch={setSearch}
      summary={`${rows.length} of ${all.length}`}
    >
      {isNamespaced && (
        <>
          <span className="list-filters__label">Namespace</span>
          <select value={localNs} onChange={(e) => setLocalNs(e.target.value)}>
            <option value="">
              {globalNamespace === ALL_NAMESPACES ? 'all namespaces' : `global (${globalNamespace})`}
            </option>
            {(namespaces ?? []).map((ns) => (
              <option key={ns.name} value={ns.name}>
                {ns.name}
              </option>
            ))}
          </select>
        </>
      )}
      {resource === 'Pods' && (
        <>
          <span className="list-filters__label">Status</span>
          <select value={statusFilter} onChange={(e) => setStatusFilter(e.target.value)}>
            <option value="">all</option>
            {PODS_STATUSES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </>
      )}
    </ListFilterBar>
  )

  return (
    <div className="page">
      <Toolbar title={scopedTitle} />
      {filterBar}
      {rows.length === 0 ? (
        <EmptyState
          icon="inbox"
          title={`No ${resource}`}
          hint={search || statusFilter || nsFilter ? 'Try clearing the filters above' : undefined}
        />
      ) : (
        <div className="page__scroll">
          {resource === 'Pods' && (
            <Table
              columns={podColumns}
              rows={rows as Pod[]}
              rowKey={(p) => `${p.namespace}/${p.name}`}
              onRowClick={(p) => openDetail('Pod', p.namespace, p.name, 'package')}
            />
          )}
          {resource === 'Nodes' && (
            <Table
              columns={nodeColumns}
              rows={rows as K8sNode[]}
              rowKey={(n) => n.name}
              onRowClick={(n) => openDetail('Node', null, n.name, 'server')}
            />
          )}
          {resource === 'Namespaces' && (
            <Table columns={namespaceColumns} rows={rows as Namespace[]} rowKey={(ns) => ns.name} />
          )}
          {resource === 'Deployments' && (
            <Table
              columns={deploymentColumns}
              rows={rows as Deployment[]}
              rowKey={(d) => `${d.namespace}/${d.name}`}
              onRowClick={(d) => openDetail('Deployment', d.namespace, d.name, 'rocket')}
            />
          )}
          {resource === 'Services' && (
            <Table
              columns={serviceColumns}
              rows={rows as Service[]}
              rowKey={(s) => `${s.namespace}/${s.name}`}
              onRowClick={(s) => openDetail('Service', s.namespace, s.name, 'plug')}
            />
          )}
        </div>
      )}
    </div>
  )
}
