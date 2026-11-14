import type { PluginManifest } from '@/core/plugin/types'
import { ExtensionDetailPage } from './ExtensionDetailPage'
import { ExtensionsSidebar } from './ExtensionsSidebar'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { EXTENSIONS_NS } from './namespace'
import { EXTENSION_TAB_KIND } from './tabs'

/**
 * Extensions view (design §6.4): every plugin with its version, category,
 * source, enabled state and availability; admins enable or disable them.
 * Display strings are keys in the "extensions" namespace.
 */
export const extensionsPlugin: PluginManifest = {
  id: EXTENSIONS_NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'core',
  core: true,
  locales: { en, tr },
  activityBar: { icon: 'extensions', tooltip: 'manifest.tooltip', order: 90 },
  sidebar: { id: 'extensions-list', title: 'manifest.sidebarTitle', component: ExtensionsSidebar },
  // "View: Show Extensions" comes with the activity bar entry.
  tabRenderers: [{ kind: EXTENSION_TAB_KIND, component: ExtensionDetailPage }],
}
