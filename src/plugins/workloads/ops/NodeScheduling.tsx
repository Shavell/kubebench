import type { K8sNode } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { useWorkloadsTranslation } from '../i18n'
import { DetailRow, DetailSection } from '../pages/detail/DetailBits'
import './ops.css'

/** The node's status badge, plus "Cordoned" while it is unschedulable (node list column). */
export function NodeStatus({ node }: { node: K8sNode }) {
  const { t } = useWorkloadsTranslation()
  return (
    <span className="ops-badges">
      <Badge>{node.status}</Badge>
      {node.unschedulable && <Badge tone="warning">{t('workloadOps.node.cordonedBadge')}</Badge>}
    </span>
  )
}

/** Node detail section: whether Pods can be scheduled, and the taints. */
export function NodeScheduling({ node }: { node: K8sNode }) {
  const { t } = useWorkloadsTranslation()
  const taints = node.taints ?? []
  return (
    <>
      <DetailSection title={t('workloadOps.node.scheduling')} />
      <div className="detail__grid">
        <DetailRow
          label={t('workloadOps.node.schedulingState')}
          value={
            node.unschedulable ? (
              <Badge tone="warning">{t('workloadOps.node.cordonedBadge')}</Badge>
            ) : (
              <Badge tone="success">{t('workloadOps.node.schedulable')}</Badge>
            )
          }
        />
      </div>
      <div className="detail__subheading">{t('workloadOps.taints.heading', { count: taints.length })}</div>
      {taints.length === 0 ? (
        <p className="ops-muted">{t('workloadOps.node.noTaints')}</p>
      ) : (
        <table className="kb-table detail__env">
          <thead>
            <tr>
              <th>{t('workloadOps.taints.key')}</th>
              <th>{t('workloadOps.taints.value')}</th>
              <th>{t('workloadOps.taints.effect')}</th>
            </tr>
          </thead>
          <tbody>
            {taints.map((taint) => (
              <tr key={`${taint.key}:${taint.effect}`}>
                <td>
                  <code className="detail__code">{taint.key}</code>
                </td>
                <td>{taint.value || '—'}</td>
                <td>{taint.effect}</td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </>
  )
}
