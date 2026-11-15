import { errorMessage, isApiError } from '@/data/apiClient'
import type { UpdatesProvider } from '@/data/providers'
import { i18n } from '@/i18n'
import { notify } from '@/shared/notify'
import { UPDATES_NS } from './namespace'
import { useUpdatesStore } from './updatesStore'

const t = (key: string, values?: Record<string, string>) => i18n.t(key, { ns: UPDATES_NS, ...values })

/**
 * Admin: checks for a release now and reports the outcome. A new release
 * opens the Update available dialog when `openDialog`, else a toast says so.
 * With update checks off the server refuses the check (409 conflict); that
 * is a setting, not a failure, so it is reported as a hint.
 */
export async function runUpdateCheck(provider: UpdatesProvider, { openDialog }: { openDialog: boolean }): Promise<void> {
  const store = useUpdatesStore.getState()
  try {
    const status = await store.check(provider)
    if (status.error) notify.error(t('notices.checkFailed'), { detail: status.error })
    else if (status.available && openDialog) store.showAvailable()
    else if (status.available) notify.info(t('statusBar.available', { version: status.latest }))
    else if (!status.latest) notify.info(t('notices.noReleases'))
    else if (status.current === 'unknown') notify.info(t('notices.unknownCurrent', { version: status.latest }))
    else notify.success(t('notices.upToDate', { version: status.current }))
  } catch (err) {
    if (isApiError(err, 'conflict')) notify.info(t('settings.checkOff'))
    else notify.error(t('notices.checkFailed'), { detail: errorMessage(err) })
  }
}
