import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { Icon } from '@/shared/Icon'
import './fullscreen.css'

/** Shown while boot asks the server about the session and loads plugins. */
export function BootSplash() {
  const { t } = useTranslation(PLATFORM_NS)
  return (
    <main className="kb-fullscreen" aria-busy="true">
      <div className="kb-fullscreen__splash" role="status">
        <Icon name="dashboard" size={40} />
        <Icon name="loading" size={16} className="codicon-modifier-spin" />
        <span>{t('boot.starting')}</span>
      </div>
    </main>
  )
}
