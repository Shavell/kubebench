import { useTranslation } from 'react-i18next'
import { appVersion, useDataMode } from '@/data/mode'
import { SETTINGS_NS } from '../namespace'
import { SettingsSection } from '../SettingsSection'

/** Version and data source of this session. */
export function AboutSection() {
  const { t } = useTranslation(SETTINGS_NS)
  // Follows a rename of the active cluster without a reload.
  const mode = useDataMode()
  const version = appVersion()

  return (
    <SettingsSection title={t('sections.about')}>
      <dl className="settings-about">
        <dt>{t('about.version')}</dt>
        <dd>{mode.commit ? t('about.build', { version, commit: mode.commit }) : version}</dd>
        <dt>{t('about.dataSource')}</dt>
        <dd>{mode.mode === 'live' ? t('about.live') : t('about.mock')}</dd>
        <dt>{t('about.context')}</dt>
        <dd>{mode.context || t('about.noCluster')}</dd>
        {mode.server && (
          <>
            <dt>{t('about.server')}</dt>
            <dd>{mode.server}</dd>
          </>
        )}
      </dl>
      <p className="settings-about__note">{t('about.storage')}</p>
    </SettingsSection>
  )
}
