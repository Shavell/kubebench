import { useProviderQuery } from '@/data/DataContext'
import { StatTile } from '@/shared/charts/StatTile'
import { TimeSeriesChart } from '@/shared/charts/TimeSeriesChart'
import { chartColor } from '@/shared/charts/chartTheme'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'
import './metrics.css'

const WINDOW_MINUTES = 60

export function ClusterOverviewPage() {
  const { data: stats } = useProviderQuery((p) => p.metrics.clusterOverview(), [])
  const { data: cpu } = useProviderQuery(
    (p) => p.metrics.clusterSeries('cpu', WINDOW_MINUTES),
    [],
  )
  const { data: memory } = useProviderQuery(
    (p) => p.metrics.clusterSeries('memory', WINDOW_MINUTES),
    [],
  )
  const { data: network } = useProviderQuery(
    (p) => p.metrics.clusterSeries('network', WINDOW_MINUTES),
    [],
  )
  const { data: fs } = useProviderQuery(
    (p) => p.metrics.clusterSeries('fs', WINDOW_MINUTES),
    [],
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
        </div>
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
