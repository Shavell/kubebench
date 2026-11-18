import { useTranslation } from 'react-i18next'
import { TERMINAL_FONT_SIZES, useDisplaySettings } from '@/core/store/displayStore'
import { formatNumber } from '@/i18n/format'
import { Field, Select } from '@/shared/form'
import { SETTINGS_NS } from '../namespace'
import { SettingsSection } from '../SettingsSection'
import { FontFamilyField } from './FontFamilyField'

/** Terminal, exec and log view preferences. */
export function TerminalSection() {
  const { t } = useTranslation(SETTINGS_NS)
  const display = useDisplaySettings()
  // A size chosen before the fixed list changed stays selectable.
  const sizes = [...new Set<number>([...TERMINAL_FONT_SIZES, display.terminalFontSize])].sort((a, b) => a - b)

  return (
    <SettingsSection title={t('sections.terminal')}>
      <FontFamilyField
        label={t('terminal.fontFamily.label')}
        help={t('terminal.fontFamily.help')}
        value={display.terminalFontFamily}
        onChange={(value) => display.set('terminalFontFamily', value)}
      />
      <Field layout="inline" label={t('terminal.fontSize.label')} help={t('terminal.fontSize.help')}>
        <Select
          value={display.terminalFontSize}
          options={sizes.map((size) => ({
            value: size,
            label: t('fonts.size', { size: formatNumber(size) }),
          }))}
          onChange={(value) => display.set('terminalFontSize', value)}
        />
      </Field>
    </SettingsSection>
  )
}
