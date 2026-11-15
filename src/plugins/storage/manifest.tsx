import type { PluginManifest } from '@/core/plugin/types'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { ClassListPage } from './pages/ClassListPage'
import { OverviewPage } from './pages/OverviewPage'
import { PvcListPage } from './pages/PvcListPage'
import { PvListPage } from './pages/PvListPage'
import { SnapshotListPage } from './pages/SnapshotListPage'
import { StorageSidebar } from './StorageSidebar'
import { openClassWizard, openListTab, openPvcWizard, openPvWizard, TAB_KINDS } from './tabs'
import { ClassWizard } from './wizards/ClassWizard'
import { PvcWizard } from './wizards/PvcWizard'
import { PvWizard } from './wizards/PvWizard'

/** Display strings below are i18n keys in the plugin's namespace (locales/*.json). */
export const storagePlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'storage',
  backendPlugin: NS,
  locales: { en, tr },
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
