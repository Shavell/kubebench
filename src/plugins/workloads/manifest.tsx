import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { WorkloadsSidebar } from './WorkloadsSidebar'
import { ResourceListPage } from './pages/ResourceListPage'
import { ResourceDetailPage } from './pages/ResourceDetailPage'

const LIST_COMMANDS: { resource: string; icon: string }[] = [
  { resource: 'Pods', icon: 'package' },
  { resource: 'Deployments', icon: 'rocket' },
  { resource: 'Services', icon: 'plug' },
  { resource: 'Nodes', icon: 'server' },
  { resource: 'Namespaces', icon: 'folder' },
]

export const workloadsPlugin: PluginManifest = {
  id: 'workloads',
  name: 'Workloads',
  activityBar: { icon: 'layers', tooltip: 'Workloads', order: 1 },
  sidebar: { id: 'workloads-explorer', title: 'Cluster Explorer', component: WorkloadsSidebar },
  tabRenderers: [
    { kind: 'workloads-list', component: ResourceListPage },
    { kind: 'workloads-detail', component: ResourceDetailPage },
  ],
  commands: LIST_COMMANDS.map(({ resource, icon }) => ({
    id: `workloads.open${resource}`,
    title: `Workloads: Open ${resource}`,
    icon,
    run: () =>
      useLayoutStore.getState().openTab({
        id: `list:${resource}`,
        title: resource,
        icon,
        kind: 'workloads-list',
        props: { resource },
      }),
  })),
}
