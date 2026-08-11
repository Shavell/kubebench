import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { Badge } from '@/shared/Badge'
import { EmptyState } from '@/shared/EmptyState'
import { EventsTable } from '@/shared/EventsTable'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import { DetailRow, DetailSection } from './detail/DetailBits'
import { PodDetail } from './detail/PodDetail'
import { NodeDetail } from './detail/NodeDetail'
import { DeploymentDetail } from './detail/DeploymentDetail'
import './pages.css'

export function ResourceDetailPage({ tab }: { tab: TabDescriptor }) {
  const kind = tab.props?.kind as string
  const namespace = (tab.props?.namespace as string | null) ?? null
  const name = tab.props?.name as string
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)

  const { data: pods, loading: podsLoading } = useProviderQuery((p) => p.cluster.listPods(), [])
  const { data: nodes } = useProviderQuery((p) => p.cluster.listNodes(), [])
  const { data: deployments } = useProviderQuery((p) => p.cluster.listDeployments(), [])
  const { data: services } = useProviderQuery((p) => p.cluster.listServices(), [])

  if (kind === 'Pod') {
    const pod = pods?.find((p) => p.namespace === namespace && p.name === name)
    if (!pod) {
      if (podsLoading || !pods) return <div className="page-loading">Loading {name}…</div>
      return (
        <EmptyState
          icon="trash"
          title={`Pod ${name} no longer exists`}
          hint="It may have been deleted"
        />
      )
    }
    return <PodDetail pod={pod} tabId={tab.id} />
  }

  if (kind === 'Node') {
    const node = nodes?.find((n) => n.name === name)
    if (!node) return <div className="page-loading">Loading {name}…</div>
    return <NodeDetail node={node} />
  }

  if (kind === 'Deployment') {
    const d = deployments?.find((x) => x.namespace === namespace && x.name === name)
    if (!d) return <div className="page-loading">Loading {name}…</div>
    return <DeploymentDetail deployment={d} />
  }

  if (kind === 'Service') {
    const s = services?.find((x) => x.namespace === namespace && x.name === name)
    if (!s) return <div className="page-loading">Loading {name}…</div>
    const viewYaml = async () => {
      const text = await providers.cluster.getResourceYaml('Service', s.namespace, s.name)
      openTab({
        id: `yaml:service:${s.namespace}/${s.name}`,
        title: `${s.name}.yaml`,
        icon: 'file-code',
        kind: 'yaml',
        props: { text, readOnly: true },
      })
    }
    return (
      <div className="page">
        <Toolbar title={`${s.namespace} / ${s.name}`}>
          <button className="kb-button kb-button--secondary" onClick={viewYaml}>
            <Icon name="file-code" size={14} /> YAML
          </button>
        </Toolbar>
        <div className="detail">
          <div className="detail__grid">
            <DetailRow label="Type" value={s.type} />
            <DetailRow label="Cluster IP" value={s.clusterIP} />
            <DetailRow label="External IP" value={s.externalIP ?? '—'} />
            <DetailRow label="Ports" value={s.ports} />
            <DetailRow label="Age" value={s.age} />
            <DetailRow label="Status" value={<Badge tone="success">Active</Badge>} />
          </div>
          <DetailSection title="Events" />
          <EventsTable kind="Service" namespace={s.namespace} name={s.name} />
        </div>
      </div>
    )
  }

  return <div className="page-loading">Unknown resource kind {kind}</div>
}
