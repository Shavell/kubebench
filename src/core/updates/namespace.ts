import { registerTranslations } from '@/i18n'
import { localeBundles } from '@/i18n/bundles'

/**
 * i18n namespace of the updates module (= its plugin id). Registered on
 * import, so the About dialog, which shows the latest release too, can use
 * it whatever the plugin registration order.
 */
export const UPDATES_NS = 'updates'

export const updatesLocales = localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' }))

registerTranslations(UPDATES_NS, updatesLocales)
