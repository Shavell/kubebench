import type { PluginManifest } from '@/core/plugin/types'
import { runScan } from './actions'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { AcceptedPage } from './pages/AcceptedPage'
import { ChecksPage } from './pages/ChecksPage'
import { FindingsPage } from './pages/FindingsPage'
import { OverviewPage } from './pages/OverviewPage'
import { SecuritySidebar } from './SecuritySidebar'
import { openFindings, openSecurityTab, TAB_KINDS } from './tabs'

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
  locales: { en, tr },
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
