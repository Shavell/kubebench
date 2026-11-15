import { useTranslation } from 'react-i18next'
import { CORE_NS, i18n } from '@/i18n'

/** i18n namespace of this plugin (= plugin id); bundles in ./locales. */
export const EDITOR_NS = 'editor'

/** This plugin's keys first, then the shared core keys (`common.*`). */
const NAMESPACES = [EDITOR_NS, CORE_NS]

/** `t()` for editor components; re-renders on language change. */
export function useEditorTranslation() {
  return useTranslation(NAMESPACES, { nsMode: 'fallback' })
}

/** Non-hook `t()` for code outside render (notifications after an apply). */
export function te(key: string, options?: Record<string, unknown>): string {
  return i18n.t(key, { ...options, ns: NAMESPACES })
}
