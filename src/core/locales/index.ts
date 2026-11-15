import { registerTranslations } from '@/i18n'
import { localeBundles } from '@/i18n/bundles'

/**
 * i18n namespace for core-level UI that is not part of the `core` bundle:
 * boot and password screens, status bar items, session dialogs, shared
 * building blocks (dialogs, forms, toasts, editors) and data-layer errors.
 * Imported by main.tsx so the bundles are registered before first render.
 */
export const PLATFORM_NS = 'platform'

/** The platform bundles by language (every `./<code>.json`). */
export const platformLocales = localeBundles(import.meta.glob('./*.json', { eager: true, import: 'default' }))

registerTranslations(PLATFORM_NS, platformLocales)
