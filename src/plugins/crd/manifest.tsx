import type { PluginManifest } from '@/core/plugin/types'
import { CrdSidebar } from './CrdSidebar'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { CrdListPage } from './pages/CrdListPage'
import { CustomResourceListPage } from './pages/CustomResourceListPage'
import { openDefinitionsTab } from './tabs'

/** Display strings below are i18n keys in the plugin's namespace (locales/*.json). */
export const crdPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'workloads',
  locales: { en, tr },
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
