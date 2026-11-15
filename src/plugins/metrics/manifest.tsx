import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { localeBundles } from '@/i18n/bundles'
import { NS } from './i18n'
import { MetricsSidebar } from './MetricsSidebar'
import { StatusBarMetrics } from './StatusBarMetrics'
import { openOverviewTab } from './tabs'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const ClusterOverviewPage = lazyComponent(() => import('./pages/ClusterOverviewPage').then((m) => m.ClusterOverviewPage))
const NodeMetricsPage = lazyComponent(() => import('./pages/NodeMetricsPage').then((m) => m.NodeMetricsPage))

/** Display strings below are i18n keys in the plugin's namespace (locales/*.json). */
export const metricsPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'observability',
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
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
