import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { ClustersSidebar } from './ClustersSidebar'
import { InventoryPage } from './InventoryPage'

export const clustersPlugin: PluginManifest = {
  id: 'clusters',
  name: 'Clusters',
  activityBar: { icon: 'server-environment', tooltip: 'Cluster Inventory', order: 0 },
  sidebar: { id: 'clusters-explorer', title: 'Clusters', component: ClustersSidebar },
  tabRenderers: [{ kind: 'clusters-inventory', component: InventoryPage }],
  commands: [
    {
      id: 'clusters.openInventory',
      title: 'Clusters: Open Inventory',
      icon: 'server-environment',
      run: () =>
        useLayoutStore.getState().openTab({
          id: 'clusters-inventory',
          title: 'Cluster Inventory',
          icon: 'server-environment',
          kind: 'clusters-inventory',
        }),
    },
  ],
}
