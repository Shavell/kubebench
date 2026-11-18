import { useTranslation } from 'react-i18next'
import { EDITOR_FONT_SIZES, useDisplaySettings } from '@/core/store/displayStore'
import { formatNumber } from '@/i18n/format'
import { Field, Select } from '@/shared/form'
import { SETTINGS_NS } from '../namespace'
import { SettingsSection } from '../SettingsSection'
import { FontFamilyField } from './FontFamilyField'

/** Font of the code editors (YAML, values, notes, diffs). */
export function EditorSection() {
  const { t } = useTranslation(SETTINGS_NS)
  const display = useDisplaySettings()

  return (
    <SettingsSection title={t('sections.editor')}>
      <FontFamilyField
        label={t('editor.fontFamily.label')}
        help={t('editor.fontFamily.help')}
        value={display.editorFontFamily}
        onChange={(value) => display.set('editorFontFamily', value)}
      />
      <Field layout="inline" label={t('editor.fontSize.label')} help={t('editor.fontSize.help')}>
        <Select
          value={display.editorFontSize}
          options={EDITOR_FONT_SIZES.map((size) => ({
            value: size,
            label: t('fonts.size', { size: formatNumber(size) }),
          }))}
          onChange={(value) => display.set('editorFontSize', value)}
        />
      </Field>
    </SettingsSection>
  )
}
