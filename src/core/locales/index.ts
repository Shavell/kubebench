import { registerTranslations } from '@/i18n'
import en from './en.json'
import tr from './tr.json'

/**
 * i18n namespace for core-level UI that is not part of the `core` bundle:
 * boot and password screens, status bar items, session dialogs, shared
 * building blocks (dialogs, forms, toasts, editors) and data-layer errors.
 * Imported by main.tsx so the bundles are registered before first render.
 */
export const PLATFORM_NS = 'platform'

registerTranslations(PLATFORM_NS, { en, tr })
