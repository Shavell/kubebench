import { useTranslation } from 'react-i18next'
import { CORE_NS, i18n } from '@/i18n'

/** i18n namespace of this plugin (= plugin id); bundles in ./locales. */
export const WORKLOADS_NS = 'workloads'

/**
 * Lookup order for every workloads string: this plugin's keys first, then
 * the shared core keys, so generic words (`common.delete`, `common.age`, …)
 * are translated once for the whole app.
 */
const NAMESPACES = [WORKLOADS_NS, CORE_NS]

/** `t()` for workloads components; re-renders on language change. */
export function useWorkloadsTranslation() {
  return useTranslation(NAMESPACES, { nsMode: 'fallback' })
}

/** Non-hook `t()` for code that runs outside render (menus built on click, commands). */
export function tw(key: string, options?: Record<string, unknown>): string {
  return i18n.t(key, { ...options, ns: NAMESPACES })
}
