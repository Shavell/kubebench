import { useState } from 'react'
import { refreshData, useProviders } from '@/data/DataContext'
import type { PodKey } from '@/data/types'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { Checkbox } from '@/shared/form'
import { notify } from '@/shared/notify'
import { useWorkloadsTranslation } from '../i18n'
import './ops.css'

/** Pods listed by name; the rest are summed up. */
const LISTED = 8

/**
 * Asks before deleting several Pods at once (POST /api/pods/bulk-delete),
 * optionally with force. Each Pod's outcome comes back; failures are
 * reported in a toast with the reasons.
 */
export function DeletePodsDialog({
  pods,
  onClose,
  onDeleted,
}: {
  pods: PodKey[]
  onClose: () => void
  /** After the request, e.g. to clear the selection. */
  onDeleted?: () => void
}) {
  const { t } = useWorkloadsTranslation()
  const { workloadOps } = useProviders()
  const [force, setForce] = useState(false)

  const remove = async () => {
    const results = await workloadOps.deletePods(pods, force)
    const failed = results.filter((r) => r.error)
    if (failed.length === 0) {
      notify.success(t('workloadOps.pods.bulkDeleted', { count: results.length }))
    } else {
      notify.error(
        t('workloadOps.pods.bulkPartial', { deleted: results.length - failed.length, total: results.length }),
        { detail: failed.map((r) => `${r.namespace}/${r.name}: ${r.error}`).join('\n') },
      )
    }
    refreshData()
    onDeleted?.()
  }

  return (
    <ConfirmDialog
      open
      danger
      title={t('workloadOps.pods.bulkTitle', { count: pods.length })}
      message={t('workloadOps.pods.bulkMessage')}
      confirmLabel={t('common.delete')}
      confirmPhrase={t('workloadOps.pods.bulkPhrase', { count: pods.length })}
      onConfirm={remove}
      onClose={onClose}
    >
      <ul className="ops-podlist">
        {pods.slice(0, LISTED).map((p) => (
          <li key={`${p.namespace}/${p.name}`}>
            <code>{`${p.namespace}/${p.name}`}</code>
          </li>
        ))}
        {pods.length > LISTED && <li className="ops-muted">{t('workloadOps.pods.bulkMore', { count: pods.length - LISTED })}</li>}
      </ul>
      <Checkbox
        checked={force}
        onChange={setForce}
        label={t('workloadOps.pods.bulkForce')}
        description={t('workloadOps.pods.bulkForceHelp')}
      />
    </ConfirmDialog>
  )
}
