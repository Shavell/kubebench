import { useState } from 'react'
import { isApiError } from '@/data/apiClient'
import { refreshData, useProviders } from '@/data/DataContext'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { Checkbox } from '@/shared/form'
import { notify } from '@/shared/notify'
import { useWorkloadsTranslation } from '../i18n'

/**
 * Asks before deleting a namespace and everything in it; the name always
 * has to be typed. KubeBench refuses its own namespace (code `forbidden`);
 * the dialog then offers to delete it anyway.
 */
export function DeleteNamespaceDialog({ name, onClose }: { name: string; onClose: () => void }) {
  const { t } = useWorkloadsTranslation()
  const { workloadOps } = useProviders()
  const [refused, setRefused] = useState(false)
  const [force, setForce] = useState(false)

  const remove = async () => {
    try {
      await workloadOps.deleteNamespace(name, force)
    } catch (err) {
      if (isApiError(err, 'forbidden')) setRefused(true)
      throw err
    }
    notify.success(t('workloadOps.namespace.deleted', { name }))
    refreshData()
  }

  return (
    <ConfirmDialog
      open
      danger
      requirePhrase
      title={t('workloadOps.namespace.deleteTitle', { name })}
      message={t('workloadOps.namespace.deleteMessage')}
      confirmLabel={t('common.delete')}
      confirmPhrase={name}
      onConfirm={remove}
      onClose={onClose}
    >
      {refused && (
        <Checkbox
          checked={force}
          onChange={setForce}
          label={t('workloadOps.namespace.force')}
          description={t('workloadOps.namespace.forceHelp')}
        />
      )}
    </ConfirmDialog>
  )
}
