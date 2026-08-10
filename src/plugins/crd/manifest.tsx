import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { CrdSidebar } from './CrdSidebar'
import { CrdListPage } from './pages/CrdListPage'
import { CustomResourceListPage } from './pages/CustomResourceListPage'

export const crdPlugin: PluginManifest = {
  id: 'crd',
  name: 'Custom Resources',
  activityBar: { icon: 'symbol-class', tooltip: 'Custom Resources (CRDs)', order: 2 },
  sidebar: { id: 'crd-explorer', title: 'Custom Resources', component: CrdSidebar },
  tabRenderers: [
    { kind: 'crd-list', component: CrdListPage },
    { kind: 'crd-instances', component: CustomResourceListPage },
  ],
  commands: [
    {
      id: 'crd.openDefinitions',
      title: 'CRDs: Open All Definitions',
      icon: 'list-tree',
      run: () =>
        useLayoutStore.getState().openTab({
          id: 'crd-all',
          title: 'Custom Resource Definitions',
          icon: 'list-tree',
          kind: 'crd-list',
        }),
    },
  ],
}
