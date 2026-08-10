import type { PluginManifest } from '@/core/plugin/types'
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
}
