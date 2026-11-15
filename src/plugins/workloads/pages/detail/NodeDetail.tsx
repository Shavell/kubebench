import { ResourceActionButtons } from '@/core/plugin/ResourceActionButtons'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useSettingsStore } from '@/core/store/settingsStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { K8sNode } from '@/data/types'
import { formatNumber } from '@/i18n/format'
import { Badge } from '@/shared/Badge'
import { EventsTable } from '@/shared/EventsTable'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Toolbar } from '@/shared/Toolbar'
import { TimeSeriesChart } from '@/shared/charts/TimeSeriesChart'
import { chartColor } from '@/shared/charts/chartTheme'
import { useWorkloadsTranslation } from '../../i18n'
import { NodeOpsActions } from '../../ops/NodeOpsActions'
import { NodeScheduling } from '../../ops/NodeScheduling'
import { WorkloadOpsDialogs } from '../../ops/WorkloadOpsDialogs'
import { DetailRow, DetailSection } from './DetailBits'
import '@/shared/tabPage.css'

export function NodeDetail({ node }: { node: K8sNode }) {
  const { t } = useWorkloadsTranslation()
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
    let text: string
    try {
      text = await providers.cluster.getResourceYaml('Node', null, node.name)
    } catch (err) {
      notify.error(err)
      return
    }
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
        <NodeOpsActions node={node} />
        <button className="kb-button kb-button--secondary" onClick={viewYaml}>
          <Icon name="file-code" size={14} /> {t('actions.yaml')}
        </button>
        <ResourceActionButtons resource={{ kind: 'Node', apiVersion: 'v1', name: node.name }} />
      </Toolbar>
      <WorkloadOpsDialogs />
      <div className="detail">
        <div className="detail__grid">
          <DetailRow label={t('columns.status')} value={<Badge>{node.status}</Badge>} />
          <DetailRow label={t('columns.roles')} value={node.roles.join(', ')} />
          <DetailRow label={t('detail.fields.kubeletVersion')} value={node.version} />
          <DetailRow label={t('detail.fields.containerRuntime')} value={node.containerRuntime} />
          <DetailRow label={t('detail.fields.osImage')} value={node.os} />
          <DetailRow label={t('detail.fields.kernelVersion')} value={node.kernelVersion} />
          <DetailRow
            label={t('detail.fields.cpuCapacity')}
            value={t('values.cores', { count: node.cpuCapacity, value: formatNumber(node.cpuCapacity) })}
          />
          <DetailRow label={t('detail.fields.memoryCapacity')} value={`${formatNumber(node.memoryCapacityGi)} Gi`} />
          <DetailRow label={t('columns.internalIP')} value={node.internalIP} />
          <DetailRow label={t('columns.age')} value={node.age} />
        </div>

        <NodeScheduling node={node} />

        <DetailSection title={t('common.labels')} />
        {Object.entries(node.labels).map(([k, v]) => (
          <span key={k} className="detail__chip">
            {v ? `${k}=${v}` : k}
          </span>
        ))}

        <DetailSection title={t('common.annotations')} />
        <div className="detail__grid detail__grid--wide">
          {Object.entries(node.annotations).map(([k, v]) => (
            <DetailRow key={k} label={k} value={<code className="detail__code">{v}</code>} />
          ))}
        </div>

        <DetailSection title={t('detail.utilization', { minutes: windowMinutes })} />
        <div className="detail__charts">
          <TimeSeriesChart title={t('columns.cpu')} data={cpu ?? []} color={chartColor('blue')} height={150} />
          <TimeSeriesChart title={t('columns.memory')} data={memory ?? []} color={chartColor('purple')} height={150} />
        </div>

        <DetailSection title={t('detail.podsOnNode', { total: nodePods.length })} />
        <table className="kb-table detail__env">
          <thead>
            <tr>
              <th>{t('columns.name')}</th>
              <th>{t('columns.namespace')}</th>
              <th>{t('columns.status')}</th>
              <th>{t('columns.restarts')}</th>
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

        <DetailSection title={t('common.events')} />
        <EventsTable kind="Node" namespace={null} name={node.name} />
      </div>
    </div>
  )
}
