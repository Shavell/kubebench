import { useProviderQuery } from '@/data/DataContext'
import { useSettingsStore } from '@/core/store/settingsStore'
import { StatTile } from '@/shared/charts/StatTile'
import { UsageBar } from '@/shared/charts/UsageBar'
import { TimeSeriesChart } from '@/shared/charts/TimeSeriesChart'
import { chartColor } from '@/shared/charts/chartTheme'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'
import './metrics.css'

export function ClusterOverviewPage() {
  const WINDOW_MINUTES = useSettingsStore((s) => s.metricsWindow)
  const { data: stats } = useProviderQuery((p) => p.metrics.clusterOverview(), [])
  const { data: allocation } = useProviderQuery((p) => p.metrics.clusterAllocation(), [])
  const { data: cpu } = useProviderQuery(
    (p) => p.metrics.clusterSeries('cpu', WINDOW_MINUTES),
    [WINDOW_MINUTES],
  )
  const { data: memory } = useProviderQuery(
    (p) => p.metrics.clusterSeries('memory', WINDOW_MINUTES),
    [WINDOW_MINUTES],
  )
  const { data: network } = useProviderQuery(
    (p) => p.metrics.clusterSeries('network', WINDOW_MINUTES),
    [WINDOW_MINUTES],
  )
  const { data: fs } = useProviderQuery(
    (p) => p.metrics.clusterSeries('fs', WINDOW_MINUTES),
    [WINDOW_MINUTES],
  )

  if (!stats) return <div className="page-loading">Loading cluster metrics…</div>

  return (
    <div className="page">
      <Toolbar title="Cluster Overview" />
      <div className="metrics-page">
        <div className="metrics-tiles">
          <StatTile
            icon="server"
            label="Nodes"
            value={`${stats.readyNodes}/${stats.nodeCount}`}
            detail="ready"
          />
          <StatTile
            icon="package"
            label="Pods"
            value={`${stats.runningPods}/${stats.podCount}`}
            detail="running"
          />
          <StatTile
            icon="dashboard"
            label="CPU"
            value={`${stats.cpuUsagePercent.toFixed(0)}%`}
            detail={`of ${stats.cpuCapacityCores} cores`}
            accent={chartColor('blue')}
          />
          <StatTile
            icon="database"
            label="Memory"
            value={`${stats.memoryUsagePercent.toFixed(0)}%`}
            detail={`of ${stats.memoryCapacityGi} Gi`}
            accent={chartColor('purple')}
          />
          <StatTile
            icon="save"
            label="Disk"
            value={`${(fs?.[fs.length - 1]?.value ?? 0).toFixed(0)}%`}
            detail="filesystem usage"
            accent={chartColor('yellow')}
          />
        </div>

        {allocation && (
          <div className="metrics-allocation">
            <div className="metrics-allocation__col">
              <div className="metrics-allocation__title">CPU Allocation</div>
              <UsageBar
                label="Usage"
                value={allocation.cpu.usage}
                total={allocation.cpu.allocatable}
                unit={allocation.cpu.unit}
                color={chartColor('blue')}
              />
              <UsageBar
                label="Requests"
                value={allocation.cpu.requests}
                total={allocation.cpu.allocatable}
                unit={allocation.cpu.unit}
                color={chartColor('blue')}
              />
              <UsageBar
                label="Limits"
                value={allocation.cpu.limits}
                total={allocation.cpu.allocatable}
                unit={allocation.cpu.unit}
                color={chartColor('blue')}
              />
            </div>
            <div className="metrics-allocation__col">
              <div className="metrics-allocation__title">Memory Allocation</div>
              <UsageBar
                label="Usage"
                value={allocation.memory.usage}
                total={allocation.memory.allocatable}
                unit={allocation.memory.unit}
                color={chartColor('purple')}
              />
              <UsageBar
                label="Requests"
                value={allocation.memory.requests}
                total={allocation.memory.allocatable}
                unit={allocation.memory.unit}
                color={chartColor('purple')}
              />
              <UsageBar
                label="Limits"
                value={allocation.memory.limits}
                total={allocation.memory.allocatable}
                unit={allocation.memory.unit}
                color={chartColor('purple')}
              />
            </div>
          </div>
        )}
        <div className="metrics-charts">
          <TimeSeriesChart
            title={`Cluster CPU · last ${WINDOW_MINUTES}m`}
            data={cpu ?? []}
            color={chartColor('blue')}
          />
          <TimeSeriesChart
            title={`Cluster Memory · last ${WINDOW_MINUTES}m`}
            data={memory ?? []}
            color={chartColor('purple')}
          />
          <TimeSeriesChart
            title={`Network I/O · last ${WINDOW_MINUTES}m`}
            data={network ?? []}
            color={chartColor('green')}
            unit=" MB/s"
            domain={[0, 60]}
          />
          <TimeSeriesChart
            title={`Filesystem · last ${WINDOW_MINUTES}m`}
            data={fs ?? []}
            color={chartColor('yellow')}
          />
        </div>
      </div>
    </div>
  )
}
