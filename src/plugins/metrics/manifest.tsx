import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { MetricsSidebar } from './MetricsSidebar'
import { ClusterOverviewPage } from './pages/ClusterOverviewPage'
import { NodeMetricsPage } from './pages/NodeMetricsPage'
import { StatusBarMetrics } from './StatusBarMetrics'

export const metricsPlugin: PluginManifest = {
  id: 'metrics',
  name: 'Metrics',
  activityBar: { icon: 'graph-line', tooltip: 'Metrics (metrics-server / Prometheus)', order: 4 },
  sidebar: { id: 'metrics-explorer', title: 'Metrics', component: MetricsSidebar },
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
      title: 'Metrics: Open Cluster Overview',
      icon: 'dashboard',
      run: () =>
        useLayoutStore.getState().openTab({
          id: 'metrics-overview',
          title: 'Cluster Overview',
          icon: 'dashboard',
          kind: 'metrics-overview',
        }),
    },
  ],
}
