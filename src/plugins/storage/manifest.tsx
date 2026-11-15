import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { localeBundles } from '@/i18n/bundles'
import { NS } from './i18n'
import { StorageSidebar } from './StorageSidebar'
import { openClassWizard, openListTab, openPvcWizard, openPvWizard, TAB_KINDS } from './tabs'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const ClassListPage = lazyComponent(() => import('./pages/ClassListPage').then((m) => m.ClassListPage))
const OverviewPage = lazyComponent(() => import('./pages/OverviewPage').then((m) => m.OverviewPage))
const PvcListPage = lazyComponent(() => import('./pages/PvcListPage').then((m) => m.PvcListPage))
const PvListPage = lazyComponent(() => import('./pages/PvListPage').then((m) => m.PvListPage))
const SnapshotListPage = lazyComponent(() => import('./pages/SnapshotListPage').then((m) => m.SnapshotListPage))
const ClassWizard = lazyComponent(() => import('./wizards/ClassWizard').then((m) => m.ClassWizard))
const PvcWizard = lazyComponent(() => import('./wizards/PvcWizard').then((m) => m.PvcWizard))
const PvWizard = lazyComponent(() => import('./wizards/PvWizard').then((m) => m.PvWizard))

/** Display strings below are i18n keys in the plugin's namespace (locales/*.json). */
export const storagePlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'storage',
  backendPlugin: NS,
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
  activityBar: { icon: 'database', tooltip: 'manifest.tooltip' },
  sidebar: { id: 'storage-explorer', title: 'manifest.sidebarTitle', component: StorageSidebar },
  tabRenderers: [
    { kind: TAB_KINDS.overview, component: OverviewPage },
    { kind: TAB_KINDS.pvcs, component: PvcListPage },
    { kind: TAB_KINDS.pvs, component: PvListPage },
    { kind: TAB_KINDS.classes, component: ClassListPage },
    { kind: TAB_KINDS.snapshots, component: SnapshotListPage },
    { kind: TAB_KINDS.newPvc, component: PvcWizard },
    { kind: TAB_KINDS.newPv, component: PvWizard },
    { kind: TAB_KINDS.newClass, component: ClassWizard },
  ],
  commands: [
    { id: 'storage.openOverview', title: 'commands.openOverview', icon: 'database', run: () => openListTab('overview') },
    { id: 'storage.openPvcs', title: 'commands.openPvcs', icon: 'save', run: () => openListTab('pvcs') },
    { id: 'storage.newPvc', title: 'commands.newPvc', icon: 'add', run: () => openPvcWizard() },
    { id: 'storage.newPv', title: 'commands.newPv', icon: 'add', run: openPvWizard },
    { id: 'storage.newClass', title: 'commands.newClass', icon: 'add', run: openClassWizard },
  ],
  resourceActions: [
    {
      id: 'storage.clonePvc',
      title: 'resourceActions.clonePvc',
      icon: 'copy',
      kinds: ['PersistentVolumeClaim'],
      permission: 'write',
      run: (ref) =>
        openPvcWizard({ namespace: ref.namespace, dataSource: { kind: 'PersistentVolumeClaim', name: ref.name } }),
    },
    {
      id: 'storage.claimPv',
      title: 'resourceActions.claimPv',
      icon: 'link',
      kinds: ['PersistentVolume'],
      permission: 'write',
      run: (ref) => openPvcWizard({ volumeName: ref.name }),
    },
  ],
}
