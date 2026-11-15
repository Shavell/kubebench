import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { localeBundles } from '@/i18n/bundles'
import { CrdSidebar } from './CrdSidebar'
import { NS } from './i18n'
import { openDefinitionsTab } from './tabs'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const CrdListPage = lazyComponent(() => import('./pages/CrdListPage').then((m) => m.CrdListPage))
const CustomResourceListPage = lazyComponent(() => import('./pages/CustomResourceListPage').then((m) => m.CustomResourceListPage))

/** Display strings below are i18n keys in the plugin's namespace (locales/*.json). */
export const crdPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'workloads',
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
  activityBar: { icon: 'symbol-class', tooltip: 'manifest.tooltip', order: 2 },
  sidebar: { id: 'crd-explorer', title: 'manifest.sidebarTitle', component: CrdSidebar },
  tabRenderers: [
    { kind: 'crd-list', component: CrdListPage },
    { kind: 'crd-instances', component: CustomResourceListPage },
  ],
  commands: [
    {
      id: 'crd.openDefinitions',
      title: 'commands.openDefinitions',
      icon: 'list-tree',
      run: openDefinitionsTab,
    },
  ],
}
