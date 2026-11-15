import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { errorMessage } from '@/data/apiClient'
import { useProviders } from '@/data/DataContext'
import { notify } from '@/shared/notify'
import { UPDATES_NS } from './namespace'
import { UpdateAvailableDialog } from './UpdateAvailableDialog'
import { useUpdatesStore } from './updatesStore'
import { showWhatsNewAfterUpgrade } from './whatsNew'
import { WhatsNewDialog } from './WhatsNewDialog'

/**
 * Hosts the updates dialogs (mounted once by the shell) and, on mount,
 * shows What's new when the user has not seen this version yet.
 */
export function UpdatesDialogs() {
  const { t } = useTranslation(UPDATES_NS)
  const { updates } = useProviders()
  const dialog = useUpdatesStore((s) => s.dialog)
  const status = useUpdatesStore((s) => s.status)
  const closeDialog = useUpdatesStore((s) => s.closeDialog)

  useEffect(() => {
    void showWhatsNewAfterUpgrade(updates)
  }, [updates])

  if (dialog?.kind === 'available' && status?.latest) {
    return <UpdateAvailableDialog status={status} onClose={closeDialog} />
  }
  if (dialog?.kind === 'whatsNew') {
    const closeWhatsNew = () => {
      closeDialog()
      updates.markSeen(dialog.version).catch((err: unknown) => {
        notify.error(t('notices.markSeenFailed'), { detail: errorMessage(err) })
      })
    }
    return <WhatsNewDialog request={dialog} onClose={closeWhatsNew} />
  }
  return null
}
