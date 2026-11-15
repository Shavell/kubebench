import { useTranslation } from 'react-i18next'
import { ShortcutList } from '@/core/shortcuts/ShortcutList'
import { useSettingsStore } from '@/core/store/settingsStore'
import { Checkbox, Field } from '@/shared/form'
import { SETTINGS_NS } from '../namespace'
import { SettingsSection } from '../SettingsSection'

/** Turning the command shortcuts off, and the list of every shortcut (also behind `?`). */
export function ShortcutsSection() {
  const { t } = useTranslation(SETTINGS_NS)
  const enabled = useSettingsStore((s) => s.shortcutsEnabled)
  const setSetting = useSettingsStore((s) => s.set)

  return (
    <SettingsSection title={t('sections.shortcuts')}>
      <Field layout="inline" label={t('shortcuts.enabled.label')} help={t('shortcuts.enabled.help')}>
        <Checkbox checked={enabled} onChange={(on) => setSetting('shortcutsEnabled', on)} />
      </Field>
      <div className="settings-shortcuts">
        <ShortcutList />
      </div>
    </SettingsSection>
  )
}
