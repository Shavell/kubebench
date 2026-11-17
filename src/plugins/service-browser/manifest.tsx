import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { localeBundles } from '@/i18n/bundles'
import { NS } from './i18n'
import { BROWSABLE_KINDS, ICON, openBrowser, TAB_KIND } from './tabs'

// The browser page loads on first use (a separate chunk).
const BrowserPage = lazyComponent(() => import('./components/BrowserPage').then((m) => m.BrowserPage))

/**
 * Service browser (design §7.14): opens a Service's or Pod's HTTP port in
 * a browser tab of the side editor group, through KubeBench's proxy.
 * Display strings below are i18n keys in its namespace.
 */
export const serviceBrowserPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'tools',
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
  tabRenderers: [{ kind: TAB_KIND, component: BrowserPage }],
  resourceActions: [
    {
      id: 'service-browser.open',
      title: 'resourceActions.open',
      icon: ICON,
      kinds: BROWSABLE_KINDS,
      permission: 'browse',
      run: openBrowser,
    },
  ],
}
