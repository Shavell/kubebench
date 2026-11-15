import { useState } from 'react'
import { usePermission } from '@/core/permissions'
import { useProviderQuery } from '@/data/DataContext'
import type { RolloutRevision } from '@/data/types'
import { formatDateTime, formatNumber } from '@/i18n/format'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { Unavailable } from '@/shared/Unavailable'
import { useWorkloadsTranslation } from '../i18n'
import type { ObjectKey } from './opsDialogs'
import { RollbackDialog } from './RollbackDialog'
import './ops.css'

/**
 * The revisions of a Deployment, newest first, each with a Rollback
 * button: disabled for the current revision, while the rollout is paused
 * and without write permission, with the reason as tooltip.
 */
export function RolloutHistory({ deployment }: { deployment: ObjectKey }) {
  const { t } = useWorkloadsTranslation()
  const write = usePermission('write')
  const [target, setTarget] = useState<RolloutRevision | null>(null)
  const { data, error, errorCode } = useProviderQuery(
    (p) => p.workloadOps.deploymentHistory(deployment.namespace, deployment.name),
    [deployment.namespace, deployment.name],
  )

  if (error) {
    return <Unavailable compact code={errorCode} reason={error} title={t('workloadOps.rollout.loadFailed')} />
  }
  if (!data) return <p className="ops-muted">{t('workloadOps.rollout.loading')}</p>
  if (data.revisions.length === 0) return <p className="ops-muted">{t('workloadOps.rollout.empty')}</p>

  const blockedReason = (r: RolloutRevision): string | null => {
    if (!write.allowed) return write.reason
    if (r.current) return t('workloadOps.rollout.currentReason')
    if (data.paused) return t('workloadOps.rollout.pausedReason')
    return null
  }

  return (
    <>
      {data.paused && (
        <div className="ops-warning" role="note">
          {t('workloadOps.rollout.pausedNote')}
        </div>
      )}
      <table className="kb-table detail__env ops-history">
        <thead>
          <tr>
            <th>{t('workloadOps.rollout.revision')}</th>
            <th>{t('workloadOps.rollout.images')}</th>
            <th>{t('workloadOps.rollout.changeCause')}</th>
            <th>{t('workloadOps.rollout.created')}</th>
            <th>{t('workloadOps.rollout.pods')}</th>
            <th aria-label={t('common.actions')} />
          </tr>
        </thead>
        <tbody>
          {data.revisions.map((r) => {
            const reason = blockedReason(r)
            return (
              <tr key={r.revision}>
                <td>
                  <span className="ops-history__revision">{formatNumber(r.revision)}</span>
                  {r.current && <Badge tone="success">{t('workloadOps.rollout.current')}</Badge>}
                </td>
                <td className="ops-history__images" title={r.images.join('\n')}>
                  {r.images.join(', ')}
                </td>
                <td title={r.changeCause || undefined}>{r.changeCause || '—'}</td>
                <td>{formatDateTime(r.created)}</td>
                <td>{`${formatNumber(r.readyReplicas)}/${formatNumber(r.replicas)}`}</td>
                <td>
                  <Button
                    variant="secondary"
                    icon="discard"
                    disabled={reason !== null}
                    title={reason ?? undefined}
                    onClick={() => setTarget(r)}
                  >
                    {t('workloadOps.rollout.rollback')}
                  </Button>
                </td>
              </tr>
            )
          })}
        </tbody>
      </table>
      <RollbackDialog deployment={deployment} revision={target} onClose={() => setTarget(null)} />
    </>
  )
}
