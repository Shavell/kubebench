import type { PluginManifest } from '@/core/plugin/types'
import { WorkloadsSidebar } from './WorkloadsSidebar'
import { ResourceListPage } from './pages/ResourceListPage'
import { ResourceDetailPage } from './pages/ResourceDetailPage'

export const workloadsPlugin: PluginManifest = {
  id: 'workloads',
  name: 'Workloads',
  activityBar: { icon: 'layers', tooltip: 'Workloads', order: 1 },
  sidebar: { id: 'workloads-explorer', title: 'Cluster Explorer', component: WorkloadsSidebar },
  tabRenderers: [
    { kind: 'workloads-list', component: ResourceListPage },
    { kind: 'workloads-detail', component: ResourceDetailPage },
  ],
}
