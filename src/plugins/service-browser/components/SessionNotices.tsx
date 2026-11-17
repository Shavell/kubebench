import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Icon } from '@/shared/Icon'
import type { ProxySession } from '../data/types'
import { NS } from '../i18n'

/**
 * What the user should know about a session: that the page runs sandboxed
 * (some apps need cookies or storage and will not work), or on the
 * separate proxy origin; and that a read-only cluster only lets it read.
 */
export function SessionNotices({ session }: { session: ProxySession }) {
  const { t } = useTranslation(NS)
  const [hidden, setHidden] = useState(false)

  return (
    <>
      {session.readOnly && (
        <div className="sb-notice sb-notice--warning" role="status">
          <Icon name="lock" size={14} />
          <span>{t('notice.readOnly')}</span>
        </div>
      )}
      {!hidden && (
        <div className="sb-notice" role="note">
          <Icon name="shield" size={14} />
          <span>{session.sandboxed ? t('notice.sandboxed') : t('notice.proxyOrigin')}</span>
          <button type="button" className="sb-icon sb-notice__close" title={t('notice.hide')} aria-label={t('notice.hide')} onClick={() => setHidden(true)}>
            <Icon name="close" size={14} />
          </button>
        </div>
      )}
    </>
  )
}
