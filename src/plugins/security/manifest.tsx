import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { localeBundles } from '@/i18n/bundles'
import { runScan } from './actions'
import { NS } from './i18n'
import { SecuritySidebar } from './SecuritySidebar'
import { openFindings, openSecurityTab, TAB_KINDS } from './tabs'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const AcceptedPage = lazyComponent(() => import('./pages/AcceptedPage').then((m) => m.AcceptedPage))
const ChecksPage = lazyComponent(() => import('./pages/ChecksPage').then((m) => m.ChecksPage))
const FindingsPage = lazyComponent(() => import('./pages/FindingsPage').then((m) => m.FindingsPage))
const OverviewPage = lazyComponent(() => import('./pages/OverviewPage').then((m) => m.OverviewPage))

/** Kinds whose objects findings are about; their views offer "Security findings". */
const FINDING_KINDS = [
  'Pod', 'Deployment', 'StatefulSet', 'DaemonSet', 'ReplicaSet', 'Job', 'CronJob', 'Namespace',
  'ServiceAccount', 'Role', 'ClusterRole', 'RoleBinding', 'ClusterRoleBinding',
]

/** Display strings below are i18n keys in the plugin's namespace (locales/*.json). */
export const securityPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'security',
  backendPlugin: NS,
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
  activityBar: { icon: 'shield', tooltip: 'manifest.tooltip' },
  sidebar: { id: 'security-explorer', title: 'manifest.sidebarTitle', component: SecuritySidebar },
  tabRenderers: [
    { kind: TAB_KINDS.overview, component: OverviewPage },
    { kind: TAB_KINDS.findings, component: FindingsPage },
    { kind: TAB_KINDS.checks, component: ChecksPage },
    { kind: TAB_KINDS.accepted, component: AcceptedPage },
  ],
  commands: [
    { id: 'security.openOverview', title: 'commands.openOverview', icon: 'shield', run: () => openSecurityTab('overview') },
    { id: 'security.openFindings', title: 'commands.openFindings', icon: 'warning', run: () => openFindings() },
    {
      id: 'security.scan',
      title: 'commands.scan',
      icon: 'play',
      run: () => {
        openSecurityTab('overview')
        void runScan()
      },
    },
  ],
  resourceActions: [
    {
      id: 'security.findings',
      title: 'resourceActions.findings',
      icon: 'shield',
      kinds: FINDING_KINDS,
      permission: 'read',
      run: (ref) => openFindings({ status: 'all', resource: { kind: ref.kind, namespace: ref.namespace, name: ref.name } }),
    },
  ],
}
