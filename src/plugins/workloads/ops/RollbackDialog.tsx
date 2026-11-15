import { refreshData, useProviders } from '@/data/DataContext'
import type { RolloutRevision } from '@/data/types'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { notify } from '@/shared/notify'
import { useWorkloadsTranslation } from '../i18n'
import type { ObjectKey } from './opsDialogs'

/**
 * Asks before rolling a Deployment back to `revision`; on a production
 * cluster the Deployment's name has to be typed.
 */
export function RollbackDialog({
  deployment,
  revision,
  onClose,
}: {
  deployment: ObjectKey
  revision: RolloutRevision | null
  onClose: () => void
}) {
  const { t } = useWorkloadsTranslation()
  const { workloadOps } = useProviders()

  const rollback = async (target: RolloutRevision) => {
    await workloadOps.rollbackDeployment(deployment.namespace, deployment.name, target.revision)
    notify.success(t('workloadOps.rollout.rolledBack', { name: deployment.name, revision: target.revision }))
    refreshData()
  }

  return (
    <ConfirmDialog
      open={revision !== null}
      title={revision ? t('workloadOps.rollout.rollbackTitle', { name: deployment.name, revision: revision.revision }) : ''}
      message={
        revision
          ? t('workloadOps.rollout.rollbackMessage', { revision: revision.revision, images: revision.images.join(', ') })
          : ''
      }
      confirmLabel={t('workloadOps.rollout.rollback')}
      confirmPhrase={deployment.name}
      onConfirm={() => (revision ? rollback(revision) : undefined)}
      onClose={onClose}
    />
  )
}
