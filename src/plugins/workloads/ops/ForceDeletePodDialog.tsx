import { refreshData, useProviders } from '@/data/DataContext'
import type { PodKey } from '@/data/types'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { notify } from '@/shared/notify'
import { useWorkloadsTranslation } from '../i18n'

/**
 * Asks before deleting a Pod with grace period 0. On a production cluster
 * the name has to be typed; a failure keeps the dialog open with the reason.
 */
export function ForceDeletePodDialog({
  pod,
  onClose,
  onDeleted,
}: {
  pod: PodKey
  onClose: () => void
  /** After the Pod was deleted, e.g. to close its detail tab. */
  onDeleted?: () => void
}) {
  const { t } = useWorkloadsTranslation()
  const { workloadOps } = useProviders()

  const remove = async () => {
    await workloadOps.forceDeletePod(pod.namespace, pod.name)
    notify.success(t('workloadOps.pods.forceDeleted', { name: pod.name }))
    refreshData()
    onDeleted?.()
  }

  return (
    <ConfirmDialog
      open
      danger
      title={t('workloadOps.pods.forceDeleteTitle', { name: pod.name })}
      message={t('workloadOps.pods.forceDeleteMessage')}
      confirmLabel={t('workloadOps.pods.forceDelete')}
      confirmPhrase={pod.name}
      onConfirm={remove}
      onClose={onClose}
    />
  )
}
