import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { localeBundles } from '@/i18n/bundles'
import { ExtensionsSidebar } from './ExtensionsSidebar'
import { EXTENSIONS_NS } from './namespace'
import { EXTENSION_TAB_KIND } from './tabs'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const ExtensionDetailPage = lazyComponent(() => import('./ExtensionDetailPage').then((m) => m.ExtensionDetailPage))

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
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
  activityBar: { icon: 'extensions', tooltip: 'manifest.tooltip', order: 90 },
  sidebar: { id: 'extensions-list', title: 'manifest.sidebarTitle', component: ExtensionsSidebar },
  // "View: Show Extensions" comes with the activity bar entry.
  tabRenderers: [{ kind: EXTENSION_TAB_KIND, component: ExtensionDetailPage }],
}
