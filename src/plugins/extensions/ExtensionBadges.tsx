import { useTranslation } from 'react-i18next'
import { Badge } from '@/shared/Badge'
import type { Extension } from './catalog'
import { EXTENSIONS_NS } from './namespace'

/**
 * Why a plugin is not active (disabled, unavailable, failed to load), and
 * an external plugin that clashes with its id; nothing when all is well.
 */
export function ExtensionBadges({ extension }: { extension: Extension }) {
  const { t } = useTranslation(EXTENSIONS_NS)
  const { status, loadError } = extension
  return (
    <>
      {extension.conflict && <Badge tone="warning">{t('status.conflict')}</Badge>}
      {!status.enabled && <Badge tone="neutral">{t('status.disabled')}</Badge>}
      {status.enabled && loadError && <Badge tone="error">{t('status.loadFailed')}</Badge>}
      {status.enabled && !loadError && !status.available && <Badge tone="warning">{t('status.unavailable')}</Badge>}
    </>
  )
}
