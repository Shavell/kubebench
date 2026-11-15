import { useTranslation } from 'react-i18next'
import { Toolbar } from '@/shared/Toolbar'
import { SETTINGS_NS } from './namespace'
import { AboutSection } from './sections/AboutSection'
import { AliasesSection } from './sections/AliasesSection'
import { AppearanceSection } from './sections/AppearanceSection'
import { DataSection } from './sections/DataSection'
import { PluginSettingsSections } from './sections/PluginSettingsSections'
import { ShortcutsSection } from './sections/ShortcutsSection'
import { TerminalSection } from './sections/TerminalSection'
import './settings.css'

/** The Settings tab: built-in preferences, then sections contributed by plugins. */
export function SettingsPage() {
  const { t } = useTranslation(SETTINGS_NS)
  return (
    <div className="settings-page">
      <Toolbar title={t('page.title')} />
      <div className="settings-page__scroll">
        <div className="settings">
          <AppearanceSection />
          <DataSection />
          <TerminalSection />
          <ShortcutsSection />
          <AliasesSection />
          <PluginSettingsSections />
          <AboutSection />
        </div>
      </div>
    </div>
  )
}
