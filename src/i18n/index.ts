import i18n from 'i18next'
import { initReactI18next, useTranslation } from 'react-i18next'
import { useSettingsStore } from '@/core/store/settingsStore'
import { localeBundles } from './bundles'
import { languageMatcher } from './languageMatch'

/**
 * Internationalization (i18next + react-i18next).
 *
 * - Core strings (shell, shared components, login) live in the `core`
 *   namespace: `src/i18n/locales/<lang>.json`.
 * - Every plugin owns a namespace equal to its plugin id and ships
 *   `src/plugins/<id>/locales/<lang>.json`, registered through the
 *   `locales` field of its PluginManifest (see core/plugin/registry.ts).
 * - Keys are nested JSON objects addressed with dots (`sidebar.title`).
 *   The `ns:key` syntax is disabled (nsSeparator=false) so literal UI
 *   strings containing ':' are safe as fallbacks; pass `{ ns }` instead.
 *
 * The language set is data: every `src/i18n/locales/<code>.json` is a
 * supported language, named by its own `language.name`. Adding a language
 * means adding a `locales/<code>.json` next to every `locales/en.json`
 * (namespaces load their bundles with `localeBundles`), then running
 * `npm run i18n:check`.
 */

const coreBundles = localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' }))

/** A supported language code, the basename of a `src/i18n/locales/*.json` file. */
export type LanguageCode = string
/** What the user picked in Settings; 'auto' follows the browser. */
export type LanguagePreference = LanguageCode | 'auto'

export const DEFAULT_LANGUAGE: LanguageCode = 'en'
export const CORE_NS = 'core'

function languageName(code: string): string {
  const name = (coreBundles[code] as { language?: { name?: unknown } }).language?.name
  return typeof name === 'string' ? name : code
}

/** The UI languages, each named in its own language: the default first, then by code. */
export const SUPPORTED_LANGUAGES: readonly { code: LanguageCode; label: string }[] = Object.keys(coreBundles)
  .sort((a, b) => Number(b === DEFAULT_LANGUAGE) - Number(a === DEFAULT_LANGUAGE) || a.localeCompare(b))
  .map((code) => ({ code, label: languageName(code) }))

const supportedCodes = new Set<string>(SUPPORTED_LANGUAGES.map((l) => l.code))

export function isSupportedLanguage(code: string): boolean {
  return supportedCodes.has(code)
}

/** The supported language for a BCP 47 tag: the exact tag ("pt-BR"), else its base ("de-AT" → "de"). */
export const matchLanguage: (tag: string) => LanguageCode | undefined = languageMatcher([...supportedCodes])

/** The browser's preferred language tags, most preferred first. */
export function browserLanguages(): string[] {
  if (typeof navigator === 'undefined') return []
  return [...(navigator.languages ?? []), navigator.language].filter(Boolean)
}

/**
 * Resolves a stored preference ('auto', a supported code, or anything
 * stale from localStorage) to a concrete supported language code.
 */
export function resolveLanguage(pref: LanguagePreference): LanguageCode {
  if (pref !== 'auto' && isSupportedLanguage(pref)) return pref
  for (const candidate of browserLanguages()) {
    const code = matchLanguage(candidate)
    if (code) return code
  }
  return DEFAULT_LANGUAGE
}

void i18n.use(initReactI18next).init({
  resources: Object.fromEntries(Object.entries(coreBundles).map(([code, bundle]) => [code, { [CORE_NS]: bundle }])),
  lng: resolveLanguage(useSettingsStore.getState().language),
  fallbackLng: DEFAULT_LANGUAGE,
  supportedLngs: SUPPORTED_LANGUAGES.map((l) => l.code),
  defaultNS: CORE_NS,
  ns: [CORE_NS],
  nsSeparator: false,
  keySeparator: '.',
  interpolation: { escapeValue: false }, // React already escapes
  returnNull: false,
  initAsync: false,
})

function syncDocumentLanguage(lng: string): void {
  if (typeof document !== 'undefined') document.documentElement.lang = lng
}
syncDocumentLanguage(i18n.language)
i18n.on('languageChanged', syncDocumentLanguage)

// Follow the Settings preference at runtime.
useSettingsStore.subscribe((state, prev) => {
  if (state.language !== prev.language) {
    void i18n.changeLanguage(resolveLanguage(state.language))
  }
})

/** Sets the language preference (persisted through the settings store). */
export function setLanguagePreference(pref: LanguagePreference): void {
  useSettingsStore.getState().set('language', pref)
}

/**
 * Registers translation bundles for a namespace (a plugin id).
 * `bundles` maps language code → nested key/value object.
 */
export function registerTranslations(ns: string, bundles: Record<string, object>): void {
  for (const [lng, bundle] of Object.entries(bundles)) {
    i18n.addResourceBundle(lng, ns, bundle, true, true)
  }
}

/**
 * Translates a plugin-manifest display string (plugin name, activity bar
 * tooltip, sidebar title, command title, …). Manifest strings are i18n
 * keys in the plugin's namespace; unknown keys fall back to the literal,
 * so plain English strings keep working.
 */
export function translateManifestText(ns: string, text: string): string {
  return i18n.t(text, { ns, defaultValue: text })
}

/** Hook flavour of translateManifestText that re-renders on language change. */
export function useManifestText(): (ns: string, text: string) => string {
  const { t } = useTranslation()
  return (ns, text) => t(text, { ns, defaultValue: text })
}

export { i18n }
export default i18n
