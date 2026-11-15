import { useEffect } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermissions } from '@/core/permissions'
import { useProviders } from '@/data/DataContext'
import { Icon } from '@/shared/Icon'
import { UPDATES_NS } from './namespace'
import { useUpdatesStore } from './updatesStore'

/** How often the status is re-read; the server caches its check for hours. */
const RELOAD_MS = 60 * 60 * 1000

/** Status bar (administrators only): "Update available: vX.Y.Z", opening the release dialog. */
export function UpdateStatusItem() {
  const { isAdmin } = usePermissions()
  return isAdmin ? <AdminUpdateItem /> : null
}

function AdminUpdateItem() {
  const { t } = useTranslation(UPDATES_NS)
  const { updates } = useProviders()
  const status = useUpdatesStore((s) => s.status)
  const showAvailable = useUpdatesStore((s) => s.showAvailable)

  useEffect(() => {
    const load = () => void useUpdatesStore.getState().load(updates)
    load()
    const timer = setInterval(load, RELOAD_MS)
    return () => clearInterval(timer)
  }, [updates])

  if (!status?.available) return null
  return (
    <button
      type="button"
      className="status-bar__item status-bar__item--button"
      title={t('statusBar.tooltip', { version: status.latest })}
      onClick={showAvailable}
    >
      <Icon name="arrow-circle-up" size={13} />
      <span>{t('statusBar.available', { version: status.latest })}</span>
    </button>
  )
}
