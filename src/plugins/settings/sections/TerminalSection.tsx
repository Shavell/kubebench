import { useTranslation } from 'react-i18next'
import { useSettingsStore } from '@/core/store/settingsStore'
import { formatNumber } from '@/i18n/format'
import { Field, Select } from '@/shared/form'
import { SETTINGS_NS } from '../namespace'
import { SettingsSection } from '../SettingsSection'

const FONT_SIZES = [11, 12, 12.5, 13, 14, 15, 16]

/** Terminal, exec and log view preferences. */
export function TerminalSection() {
  const { t } = useTranslation(SETTINGS_NS)
  const fontSize = useSettingsStore((s) => s.terminalFontSize)
  const setSetting = useSettingsStore((s) => s.set)

  return (
    <SettingsSection title={t('sections.terminal')}>
      <Field layout="inline" label={t('terminal.fontSize.label')} help={t('terminal.fontSize.help')}>
        <Select
          value={fontSize}
          options={FONT_SIZES.map((size) => ({
            value: size,
            label: t('terminal.fontSize.value', { size: formatNumber(size) }),
          }))}
          onChange={(value) => setSetting('terminalFontSize', value)}
        />
      </Field>
    </SettingsSection>
  )
}
