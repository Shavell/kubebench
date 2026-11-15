import { ResourceActionButtons } from '@/core/plugin/ResourceActionButtons'
import { useSettingsStore } from '@/core/store/settingsStore'
import { useProviderQuery } from '@/data/DataContext'
import type { K8sNode } from '@/data/types'
import { formatNumber } from '@/i18n/format'
import { Badge } from '@/shared/Badge'
import { EventsTable } from '@/shared/EventsTable'
import { Toolbar } from '@/shared/Toolbar'
import { MetricChart } from '@/shared/charts/MetricChart'
import { chartColor } from '@/shared/charts/chartTheme'
import { ResourceEditButtons } from '../../editing/ResourceEditButtons'
import { useWorkloadsTranslation } from '../../i18n'
import { NodeOpsActions } from '../../ops/NodeOpsActions'
import { NodeScheduling } from '../../ops/NodeScheduling'
import { WorkloadOpsDialogs } from '../../ops/WorkloadOpsDialogs'
import { DetailRow, DetailSection } from './DetailBits'
import '@/shared/tabPage.css'

export function NodeDetail({ node, onDeleted }: { node: K8sNode; onDeleted: () => void }) {
  const { t } = useWorkloadsTranslation()
  const windowMinutes = useSettingsStore((s) => s.metricsWindow)

  const cpu = useProviderQuery(
    (p) => p.metrics.nodeSeries(node.name, 'cpu', windowMinutes),
    [node.name, windowMinutes],
  )
  const memory = useProviderQuery(
    (p) => p.metrics.nodeSeries(node.name, 'memory', windowMinutes),
    [node.name, windowMinutes],
  )
  const { data: pods } = useProviderQuery((p) => p.cluster.listPods(), [], { topics: ['pods'] })
  const nodePods = (pods ?? []).filter((p) => p.node === node.name)

  const resource = { kind: 'Node', apiVersion: 'v1', name: node.name }

  return (
    <div className="page">
      <Toolbar title={node.name}>
        <NodeOpsActions node={node} />
        <ResourceEditButtons resource={resource} onDeleted={onDeleted} />
        <ResourceActionButtons resource={resource} />
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
          <MetricChart title={t('columns.cpu')} series={cpu} color={chartColor('blue')} height={150} />
          <MetricChart title={t('columns.memory')} series={memory} color={chartColor('purple')} height={150} />
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
