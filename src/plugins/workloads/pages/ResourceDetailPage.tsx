import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import type { Pod } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import './pages.css'

function Row({ label, value }: { label: string; value: React.ReactNode }) {
  return (
    <>
      <span className="detail__label">{label}</span>
      <span className="detail__value">{value}</span>
    </>
  )
}

export function ResourceDetailPage({ tab }: { tab: TabDescriptor }) {
  const kind = tab.props?.kind as string
  const namespace = (tab.props?.namespace as string | null) ?? null
  const name = tab.props?.name as string
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)

  const { data: pods } = useProviderQuery((p) => p.cluster.listPods(), [])
  const { data: nodes } = useProviderQuery((p) => p.cluster.listNodes(), [])
  const { data: deployments } = useProviderQuery((p) => p.cluster.listDeployments(), [])
  const { data: services } = useProviderQuery((p) => p.cluster.listServices(), [])

  const viewYaml = async () => {
    const text = await providers.cluster.getResourceYaml(kind, namespace, name)
    openTab({
      id: `yaml:${tab.id}`,
      title: `${name}.yaml`,
      icon: 'file-code',
      kind: 'yaml',
      props: { text, readOnly: true },
    })
  }

  const yamlButton = (
    <button className="kb-button" onClick={viewYaml}>
      <Icon name="file-code" size={14} /> View YAML
    </button>
  )

  if (kind === 'Pod') {
    const pod = pods?.find((p) => p.namespace === namespace && p.name === name)
    if (!pod) return <div className="page-loading">Loading {name}…</div>
    return <PodDetail pod={pod} actions={yamlButton} />
  }

  if (kind === 'Node') {
    const node = nodes?.find((n) => n.name === name)
    if (!node) return <div className="page-loading">Loading {name}…</div>
    return (
      <div className="page">
        <Toolbar title={node.name}>{yamlButton}</Toolbar>
        <div className="detail">
          <div className="detail__grid">
            <Row label="Status" value={<Badge>{node.status}</Badge>} />
            <Row label="Roles" value={node.roles.join(', ')} />
            <Row label="Kubelet version" value={node.version} />
            <Row label="OS image" value={node.os} />
            <Row label="CPU capacity" value={`${node.cpuCapacity} cores`} />
            <Row label="Memory capacity" value={`${node.memoryCapacityGi} Gi`} />
            <Row label="Internal IP" value={node.internalIP} />
            <Row label="Age" value={node.age} />
          </div>
        </div>
      </div>
    )
  }

  if (kind === 'Deployment') {
    const d = deployments?.find((x) => x.namespace === namespace && x.name === name)
    if (!d) return <div className="page-loading">Loading {name}…</div>
    return (
      <div className="page">
        <Toolbar title={`${d.namespace} / ${d.name}`}>{yamlButton}</Toolbar>
        <div className="detail">
          <div className="detail__grid">
            <Row label="Replicas" value={`${d.readyReplicas} ready / ${d.replicas} desired`} />
            <Row label="Up-to-date" value={String(d.upToDate)} />
            <Row label="Available" value={String(d.available)} />
            <Row label="Age" value={d.age} />
          </div>
          <div className="detail__section">Labels</div>
          {Object.entries(d.labels).map(([k, v]) => (
            <span key={k} className="detail__chip">
              {k}={v}
            </span>
          ))}
        </div>
      </div>
    )
  }

  if (kind === 'Service') {
    const s = services?.find((x) => x.namespace === namespace && x.name === name)
    if (!s) return <div className="page-loading">Loading {name}…</div>
    return (
      <div className="page">
        <Toolbar title={`${s.namespace} / ${s.name}`}>{yamlButton}</Toolbar>
        <div className="detail">
          <div className="detail__grid">
            <Row label="Type" value={s.type} />
            <Row label="Cluster IP" value={s.clusterIP} />
            <Row label="External IP" value={s.externalIP ?? '—'} />
            <Row label="Ports" value={s.ports} />
            <Row label="Age" value={s.age} />
          </div>
        </div>
      </div>
    )
  }

  return <div className="page-loading">Unknown resource kind {kind}</div>
}

function PodDetail({ pod, actions }: { pod: Pod; actions: React.ReactNode }) {
  return (
    <div className="page">
      <Toolbar title={`${pod.namespace} / ${pod.name}`}>{actions}</Toolbar>
      <div className="detail">
        <div className="detail__grid">
          <Row label="Status" value={<Badge>{pod.status}</Badge>} />
          <Row label="Ready" value={`${pod.readyContainers}/${pod.totalContainers}`} />
          <Row label="Restarts" value={String(pod.restarts)} />
          <Row label="Node" value={pod.node || '—'} />
          <Row label="Pod IP" value={pod.podIP || '—'} />
          <Row label="Age" value={pod.age} />
        </div>
        <div className="detail__section">Labels</div>
        {Object.entries(pod.labels).map(([k, v]) => (
          <span key={k} className="detail__chip">
            {k}={v}
          </span>
        ))}
        <div className="detail__section">Containers</div>
        <div className="detail__grid">
          {pod.containers.map((c) => (
            <Row key={c.name} label={c.name} value={c.image} />
          ))}
        </div>
      </div>
    </div>
  )
}
