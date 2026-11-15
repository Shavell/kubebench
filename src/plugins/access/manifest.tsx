import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { localeBundles } from '@/i18n/bundles'
import { AccessSidebar } from './AccessSidebar'
import { NS } from './i18n'
import { openListTab, openSubject, TAB_KINDS } from './tabs'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const GraphPage = lazyComponent(() => import('./pages/GraphPage').then((m) => m.GraphPage))
const RiskyRolesPage = lazyComponent(() => import('./pages/RiskyRolesPage').then((m) => m.RiskyRolesPage))
const SubjectPage = lazyComponent(() => import('./pages/SubjectPage').then((m) => m.SubjectPage))
const SubjectsPage = lazyComponent(() => import('./pages/SubjectsPage').then((m) => m.SubjectsPage))
const WhoCanPage = lazyComponent(() => import('./pages/WhoCanPage').then((m) => m.WhoCanPage))

/** Display strings below are i18n keys in the plugin's namespace (locales/*.json). */
export const accessPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'security',
  backendPlugin: NS,
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
  activityBar: { icon: 'key', tooltip: 'manifest.tooltip' },
  sidebar: { id: 'access-explorer', title: 'manifest.sidebarTitle', component: AccessSidebar },
  tabRenderers: [
    { kind: TAB_KINDS.whoCan, component: WhoCanPage },
    { kind: TAB_KINDS.subjects, component: SubjectsPage },
    { kind: TAB_KINDS.risky, component: RiskyRolesPage },
    { kind: TAB_KINDS.graph, component: GraphPage },
    { kind: TAB_KINDS.subject, component: SubjectPage },
  ],
  commands: [
    { id: 'access.whoCan', title: 'commands.whoCan', icon: 'search', run: () => openListTab('whoCan') },
    { id: 'access.subjects', title: 'commands.subjects', icon: 'organization', run: () => openListTab('subjects') },
    { id: 'access.risky', title: 'commands.risky', icon: 'warning', run: () => openListTab('risky') },
    { id: 'access.graph', title: 'commands.graph', icon: 'type-hierarchy', run: () => openListTab('graph') },
  ],
  resourceActions: [
    {
      id: 'access.serviceAccountPermissions',
      title: 'resourceActions.permissions',
      icon: 'key',
      kinds: ['ServiceAccount'],
      permission: 'read',
      run: (ref) => openSubject({ kind: 'ServiceAccount', namespace: ref.namespace, name: ref.name }),
    },
  ],
}
