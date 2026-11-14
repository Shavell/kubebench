import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { WorkloadsSidebar } from './WorkloadsSidebar'
import { ResourceListPage } from './pages/ResourceListPage'
import { ResourceDetailPage } from './pages/ResourceDetailPage'
import { GenericListPage } from './pages/GenericListPage'
import { GenericDetailPage } from './pages/GenericDetailPage'
import { WORKLOADS_NS } from './i18n'
import { richLabel } from './resourceCatalog'
import en from './locales/en.json'
import tr from './locales/tr.json'

const LIST_COMMANDS: { resource: string; icon: string }[] = [
  { resource: 'Pods', icon: 'package' },
  { resource: 'Deployments', icon: 'rocket' },
  { resource: 'Services', icon: 'plug' },
  { resource: 'Nodes', icon: 'server' },
  { resource: 'Namespaces', icon: 'folder' },
]

/** Display strings are i18n keys in the "workloads" namespace (./locales). */
export const workloadsPlugin: PluginManifest = {
  id: WORKLOADS_NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'workloads',
  core: true,
  locales: { en, tr },
  activityBar: { icon: 'layers', tooltip: 'manifest.tooltip', order: 1 },
  sidebar: { id: 'workloads-explorer', title: 'manifest.sidebarTitle', component: WorkloadsSidebar },
  tabRenderers: [
    { kind: 'workloads-list', component: ResourceListPage },
    { kind: 'workloads-detail', component: ResourceDetailPage },
    { kind: 'generic-list', component: GenericListPage },
    { kind: 'generic-detail', component: GenericDetailPage },
  ],
  commands: LIST_COMMANDS.map(({ resource, icon }) => ({
    id: `workloads.open${resource}`,
    title: `commands.open${resource}`,
    icon,
    run: () =>
      useLayoutStore.getState().openTab({
        id: `list:${resource}`,
        title: richLabel(resource),
        titleKey: `resources.${resource.toLowerCase()}`,
        titleNs: WORKLOADS_NS,
        icon,
        kind: 'workloads-list',
        props: { resource },
      }),
  })),
}
