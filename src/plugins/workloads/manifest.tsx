import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { WorkloadsSidebar } from './WorkloadsSidebar'
import { WORKLOADS_NS } from './i18n'
import { workloadKinds } from './kinds'
import { richLabel } from './resourceCatalog'
import en from './locales/en.json'
import tr from './locales/tr.json'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const ResourceDetailPage = lazyComponent(() => import('./pages/ResourceDetailPage').then((m) => m.ResourceDetailPage))
const GenericDetailPage = lazyComponent(() => import('./pages/GenericDetailPage').then((m) => m.GenericDetailPage))
const ResourceListPage = lazyComponent(() => import('./pages/ResourceListPage').then((m) => m.ResourceListPage))
const GenericListPage = lazyComponent(() => import('./pages/GenericListPage').then((m) => m.GenericListPage))

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
  resourceKinds: workloadKinds,
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
