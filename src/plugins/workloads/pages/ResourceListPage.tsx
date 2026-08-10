import { useLayoutStore } from '@/core/store/layoutStore'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { useProviderQuery } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import type { Deployment, K8sNode, Namespace, Pod, Service } from '@/data/types'
import { Badge } from '@/shared/Badge'
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

const podColumns: Column<Pod>[] = [
  { key: 'name', header: 'Name', render: (p) => p.name, sortValue: (p) => p.name },
  { key: 'ns', header: 'Namespace', render: (p) => p.namespace, sortValue: (p) => p.namespace },
  {
    key: 'ready',
    header: 'Ready',
    render: (p) => `${p.readyContainers}/${p.totalContainers}`,
  },
  { key: 'status', header: 'Status', render: (p) => <Badge>{p.status}</Badge>, sortValue: (p) => p.status },
  { key: 'restarts', header: 'Restarts', render: (p) => String(p.restarts), sortValue: (p) => p.restarts },
  { key: 'age', header: 'Age', render: (p) => p.age },
  { key: 'node', header: 'Node', render: (p) => p.node || '—' },
]

const nodeColumns: Column<K8sNode>[] = [
  { key: 'name', header: 'Name', render: (n) => n.name, sortValue: (n) => n.name },
  { key: 'status', header: 'Status', render: (n) => <Badge>{n.status}</Badge> },
  { key: 'roles', header: 'Roles', render: (n) => n.roles.join(', ') },
  { key: 'version', header: 'Version', render: (n) => n.version },
  { key: 'cpu', header: 'CPU', render: (n) => `${n.cpuCapacity} cores` },
  { key: 'mem', header: 'Memory', render: (n) => `${n.memoryCapacityGi} Gi` },
  { key: 'age', header: 'Age', render: (n) => n.age },
  { key: 'ip', header: 'Internal IP', render: (n) => n.internalIP },
]

const namespaceColumns: Column<Namespace>[] = [
  { key: 'name', header: 'Name', render: (ns) => ns.name, sortValue: (ns) => ns.name },
  { key: 'status', header: 'Status', render: (ns) => <Badge>{ns.status}</Badge> },
  { key: 'age', header: 'Age', render: (ns) => ns.age },
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
  },
  { key: 'uptodate', header: 'Up-to-date', render: (d) => String(d.upToDate) },
  { key: 'available', header: 'Available', render: (d) => String(d.available) },
  { key: 'age', header: 'Age', render: (d) => d.age },
]

const serviceColumns: Column<Service>[] = [
  { key: 'name', header: 'Name', render: (s) => s.name, sortValue: (s) => s.name },
  { key: 'ns', header: 'Namespace', render: (s) => s.namespace, sortValue: (s) => s.namespace },
  { key: 'type', header: 'Type', render: (s) => s.type },
  { key: 'clusterip', header: 'Cluster IP', render: (s) => s.clusterIP },
  { key: 'externalip', header: 'External IP', render: (s) => s.externalIP ?? '—' },
  { key: 'ports', header: 'Ports', render: (s) => s.ports },
  { key: 'age', header: 'Age', render: (s) => s.age },
]

export function ResourceListPage({ tab }: { tab: TabDescriptor }) {
  const resource = tab.props?.resource as string
  const namespace = useContextStore((s) => s.namespace)
  const nsFilter = namespace === ALL_NAMESPACES ? undefined : namespace

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

  if (query.loading) {
    return <div className="page-loading">Loading {resource}…</div>
  }

  const rows = query.data ?? []
  const scopedTitle =
    nsFilter && resource !== 'Nodes' && resource !== 'Namespaces'
      ? `${resource} · ${nsFilter}`
      : resource

  if (rows.length === 0) {
    return (
      <div className="page">
        <Toolbar title={scopedTitle} />
        <EmptyState icon="inbox" title={`No ${resource}`} hint="Try clearing the namespace filter in the status bar" />
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={scopedTitle} />
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
    </div>
  )
}
