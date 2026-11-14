import type { PluginManifest } from '@/core/plugin/types'
import { NS } from './i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { MetricsSidebar } from './MetricsSidebar'
import { ClusterOverviewPage } from './pages/ClusterOverviewPage'
import { NodeMetricsPage } from './pages/NodeMetricsPage'
import { StatusBarMetrics } from './StatusBarMetrics'
import { openOverviewTab } from './tabs'

/** Display strings below are i18n keys in the plugin's namespace (locales/*.json). */
export const metricsPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'observability',
  locales: { en, tr },
  activityBar: { icon: 'graph-line', tooltip: 'manifest.tooltip', order: 4 },
  sidebar: { id: 'metrics-explorer', title: 'manifest.sidebarTitle', component: MetricsSidebar },
  tabRenderers: [
    { kind: 'metrics-overview', component: ClusterOverviewPage },
    { kind: 'metrics-node', component: NodeMetricsPage },
  ],
  statusBarItems: [
    { id: 'metrics-usage', alignment: 'right', priority: 10, component: StatusBarMetrics },
  ],
  commands: [
    {
      id: 'metrics.openOverview',
      title: 'commands.openOverview',
      icon: 'dashboard',
      run: openOverviewTab,
    },
  ],
}
