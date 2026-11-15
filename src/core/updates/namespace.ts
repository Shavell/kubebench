import { registerTranslations } from '@/i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'

/**
 * i18n namespace of the updates module (= its plugin id). Registered on
 * import, so the About dialog, which shows the latest release too, can use
 * it whatever the plugin registration order.
 */
export const UPDATES_NS = 'updates'

export const updatesLocales = { en, tr }

registerTranslations(UPDATES_NS, updatesLocales)
