import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { usePermissions } from '@/core/permissions'
import { useDataMode } from '@/data/mode'
import { Icon } from '@/shared/Icon'

/**
 * "Read-only" when nothing can be changed on the active cluster: it is
 * flagged read-only (for everyone), or the user's role on it is viewer.
 */
export function ReadOnlyItem() {
  const { t } = useTranslation(PLATFORM_NS)
  const { cluster } = useDataMode()
  const perms = usePermissions()
  if (!cluster || (!perms.readOnly && perms.role !== 'viewer')) return null
  return (
    <div
      className="status-bar__item status-bar__item--readonly"
      title={perms.readOnly ? t('statusBar.readOnly.clusterHint') : t('statusBar.readOnly.viewerHint')}
    >
      <Icon name="lock" size={13} />
      <span>{t('statusBar.readOnly.label')}</span>
    </div>
  )
}
