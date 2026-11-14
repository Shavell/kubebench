import type { PluginManifest } from '@/core/plugin/types'
import { ClustersPage } from './ClustersPage'
import { ClustersSidebar } from './ClustersSidebar'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { CLUSTERS_NS } from './namespace'
import { INVENTORY_TAB_KIND, openInventoryTab } from './tabs'

/** Cluster inventory and switching. Display strings are keys in the "clusters" namespace. */
export const clustersPlugin: PluginManifest = {
  id: CLUSTERS_NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'core',
  core: true,
  locales: { en, tr },
  activityBar: { icon: 'server-environment', tooltip: 'manifest.tooltip', order: 0 },
  sidebar: { id: 'clusters-explorer', title: 'manifest.sidebarTitle', component: ClustersSidebar },
  tabRenderers: [{ kind: INVENTORY_TAB_KIND, component: ClustersPage }],
  commands: [
    { id: 'clusters.openInventory', title: 'commands.openInventory', icon: 'server-environment', run: openInventoryTab },
  ],
}
