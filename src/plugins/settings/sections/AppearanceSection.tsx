import { useTranslation } from 'react-i18next'
import { UI_SCALES, useDisplaySettings } from '@/core/store/displayStore'
import { useSettingsStore, type ThemePreference } from '@/core/store/settingsStore'
import { useSystemTheme } from '@/core/theme/themeController'
import {
  isSupportedLanguage,
  resolveLanguage,
  setLanguagePreference,
  SUPPORTED_LANGUAGES,
  type LanguagePreference,
} from '@/i18n'
import { formatPercent } from '@/i18n/format'
import { Field, Select, type SelectOption } from '@/shared/form'
import { SETTINGS_NS } from '../namespace'
import { SettingsSection } from '../SettingsSection'

/** Colour theme, UI language and interface scale. */
export function AppearanceSection() {
  const { t } = useTranslation(SETTINGS_NS)
  const theme = useSettingsStore((s) => s.theme)
  const language = useSettingsStore((s) => s.language)
  const setSetting = useSettingsStore((s) => s.set)
  const display = useDisplaySettings()

  // "System" names the palette the operating system currently asks for.
  const systemTheme = useSystemTheme()
  const themes: SelectOption<ThemePreference>[] = [
    { value: 'dark', label: t('appearance.theme.dark') },
    { value: 'light', label: t('appearance.theme.light') },
    {
      value: 'system',
      label: t('appearance.theme.system', { theme: t(`appearance.theme.${systemTheme}`) }),
    },
  ]

  // "Automatic" names the language it currently resolves to.
  const browserLanguage = resolveLanguage('auto')
  const browserLabel = SUPPORTED_LANGUAGES.find((l) => l.code === browserLanguage)?.label ?? browserLanguage
  const languages: SelectOption<LanguagePreference>[] = [
    { value: 'auto', label: t('appearance.language.auto', { language: browserLabel }) },
    // Language names stay in their own language.
    ...SUPPORTED_LANGUAGES.map((l) => ({ value: l.code, label: l.label })),
  ]
  // 'auto', or a stale stored code of a removed language, shows as Automatic.
  const languageValue: LanguagePreference = isSupportedLanguage(language) ? language : 'auto'

  return (
    <SettingsSection title={t('sections.appearance')}>
      <Field layout="inline" label={t('appearance.theme.label')} help={t('appearance.theme.help')}>
        <Select value={theme} options={themes} onChange={(value) => setSetting('theme', value)} />
      </Field>
      <Field layout="inline" label={t('appearance.language.label')} help={t('appearance.language.help')}>
        <Select value={languageValue} options={languages} onChange={setLanguagePreference} />
      </Field>
      <Field layout="inline" label={t('appearance.scale.label')} help={t('appearance.scale.help')}>
        <Select
          value={display.uiScale}
          options={UI_SCALES.map((scale) => ({
            value: scale,
            label: scale === 100 ? t('appearance.scale.default', { scale: formatPercent(1) }) : formatPercent(scale / 100),
          }))}
          onChange={(value) => display.set('uiScale', value)}
        />
      </Field>
    </SettingsSection>
  )
}
