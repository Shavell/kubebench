import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { Checkbox } from '@/shared/form'
import { NS } from '../../i18n'

/**
 * Asks before `helm uninstall`. Keeping the history leaves the release
 * listed as uninstalled, so it can be rolled back later.
 */
export function UninstallDialog({
  namespace,
  name,
  onConfirm,
  onClose,
}: {
  namespace: string
  name: string
  onConfirm: (keepHistory: boolean) => Promise<void>
  onClose: () => void
}) {
  const { t } = useTranslation(NS)
  const [keepHistory, setKeepHistory] = useState(false)
  return (
    <ConfirmDialog
      open
      danger
      title={t('release.uninstallDialog.title', { name })}
      message={t('release.uninstallDialog.message', { namespace })}
      confirmLabel={t('release.uninstall')}
      // Release names are unique per namespace only: the phrase names both.
      confirmPhrase={`${namespace}/${name}`}
      onConfirm={() => onConfirm(keepHistory)}
      onClose={onClose}
    >
      <Checkbox
        checked={keepHistory}
        onChange={setKeepHistory}
        label={t('release.uninstallDialog.keepHistory')}
        description={t('release.uninstallDialog.keepHistoryHint')}
      />
    </ConfirmDialog>
  )
}
