import { refreshData, useProviders } from '@/data/DataContext'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { notify } from '@/shared/notify'
import { useWorkloadsTranslation } from './i18n'

/** The Pod a delete dialog is open for; null while it is closed. */
export interface PodRef {
  namespace: string
  name: string
}

/**
 * Asks before deleting a Pod. On a production cluster the name has to be
 * typed (ConfirmDialog); a failure keeps the dialog open with the reason.
 */
export function DeletePodDialog({
  pod,
  onClose,
  onDeleted,
}: {
  pod: PodRef | null
  onClose: () => void
  /** After the Pod was deleted, e.g. to close its detail tab. */
  onDeleted?: () => void
}) {
  const { t } = useWorkloadsTranslation()
  const { cluster } = useProviders()

  const remove = async (target: PodRef) => {
    await cluster.deletePod(target.namespace, target.name)
    notify.success(t('deletePod.deleted', { name: target.name }))
    refreshData()
    onDeleted?.()
  }

  return (
    <ConfirmDialog
      open={pod !== null}
      danger
      title={pod ? t('deletePod.title', { name: pod.name }) : ''}
      message={t('deletePod.message')}
      confirmLabel={t('common.delete')}
      confirmPhrase={pod?.name}
      onConfirm={() => (pod ? remove(pod) : undefined)}
      onClose={onClose}
    />
  )
}
