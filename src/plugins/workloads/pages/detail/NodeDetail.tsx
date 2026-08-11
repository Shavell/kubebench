import { useLayoutStore } from '@/core/store/layoutStore'
import { useSettingsStore } from '@/core/store/settingsStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { K8sNode } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { EventsTable } from '@/shared/EventsTable'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import { TimeSeriesChart } from '@/shared/charts/TimeSeriesChart'
import { chartColor } from '@/shared/charts/chartTheme'
import { DetailRow, DetailSection } from './DetailBits'
import '../pages.css'

export function NodeDetail({ node }: { node: K8sNode }) {
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)
  const windowMinutes = useSettingsStore((s) => s.metricsWindow)

  const { data: cpu } = useProviderQuery(
    (p) => p.metrics.nodeSeries(node.name, 'cpu', windowMinutes),
    [node.name, windowMinutes],
  )
  const { data: memory } = useProviderQuery(
    (p) => p.metrics.nodeSeries(node.name, 'memory', windowMinutes),
    [node.name, windowMinutes],
  )
  const { data: pods } = useProviderQuery((p) => p.cluster.listPods(), [])
  const nodePods = (pods ?? []).filter((p) => p.node === node.name)

  const viewYaml = async () => {
    const text = await providers.cluster.getResourceYaml('Node', null, node.name)
    openTab({
      id: `yaml:node:${node.name}`,
      title: `${node.name}.yaml`,
      icon: 'file-code',
      kind: 'yaml',
      props: { text, readOnly: true },
    })
  }

  return (
    <div className="page">
      <Toolbar title={node.name}>
        <button className="kb-button kb-button--secondary" onClick={viewYaml}>
          <Icon name="file-code" size={14} /> YAML
        </button>
      </Toolbar>
      <div className="detail">
        <div className="detail__grid">
          <DetailRow label="Status" value={<Badge>{node.status}</Badge>} />
          <DetailRow label="Roles" value={node.roles.join(', ')} />
          <DetailRow label="Kubelet version" value={node.version} />
          <DetailRow label="Container runtime" value={node.containerRuntime} />
          <DetailRow label="OS image" value={node.os} />
          <DetailRow label="Kernel version" value={node.kernelVersion} />
          <DetailRow label="CPU capacity" value={`${node.cpuCapacity} cores`} />
          <DetailRow label="Memory capacity" value={`${node.memoryCapacityGi} Gi`} />
          <DetailRow label="Internal IP" value={node.internalIP} />
          <DetailRow label="Age" value={node.age} />
        </div>

        <DetailSection title="Labels" />
        {Object.entries(node.labels).map(([k, v]) => (
          <span key={k} className="detail__chip">
            {v ? `${k}=${v}` : k}
          </span>
        ))}

        <DetailSection title="Annotations" />
        <div className="detail__grid detail__grid--wide">
          {Object.entries(node.annotations).map(([k, v]) => (
            <DetailRow key={k} label={k} value={<code className="detail__code">{v}</code>} />
          ))}
        </div>

        <DetailSection title={`Utilization · last ${windowMinutes}m`} />
        <div className="detail__charts">
          <TimeSeriesChart title="CPU" data={cpu ?? []} color={chartColor('blue')} height={150} />
          <TimeSeriesChart title="Memory" data={memory ?? []} color={chartColor('purple')} height={150} />
        </div>

        <DetailSection title={`Pods on this node (${nodePods.length})`} />
        <table className="kb-table detail__env">
          <thead>
            <tr>
              <th>Name</th>
              <th>Namespace</th>
              <th>Status</th>
              <th>Restarts</th>
            </tr>
          </thead>
          <tbody>
            {nodePods.map((p) => (
              <tr key={`${p.namespace}/${p.name}`}>
                <td>{p.name}</td>
                <td>{p.namespace}</td>
                <td>
                  <Badge>{p.status}</Badge>
                </td>
                <td>{p.restarts}</td>
              </tr>
            ))}
          </tbody>
        </table>

        <DetailSection title="Events" />
        <EventsTable kind="Node" namespace={null} name={node.name} />
      </div>
    </div>
  )
}
