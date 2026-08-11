import { useProviderQuery } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { TimeSeriesChart } from '@/shared/charts/TimeSeriesChart'
import { chartColor } from '@/shared/charts/chartTheme'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'
import './metrics.css'

const WINDOW_MINUTES = 60

export function NodeMetricsPage({ tab }: { tab: TabDescriptor }) {
  const node = tab.props?.node as string
  const { data: cpu } = useProviderQuery(
    (p) => p.metrics.nodeSeries(node, 'cpu', WINDOW_MINUTES),
    [node],
  )
  const { data: memory } = useProviderQuery(
    (p) => p.metrics.nodeSeries(node, 'memory', WINDOW_MINUTES),
    [node],
  )
  const { data: network } = useProviderQuery(
    (p) => p.metrics.nodeSeries(node, 'network', WINDOW_MINUTES),
    [node],
  )
  const { data: fs } = useProviderQuery(
    (p) => p.metrics.nodeSeries(node, 'fs', WINDOW_MINUTES),
    [node],
  )

  if (!cpu || !memory) return <div className="page-loading">Loading {node} metrics…</div>

  return (
    <div className="page">
      <Toolbar title={`Node Metrics · ${node}`} />
      <div className="metrics-page">
        <div className="metrics-charts">
          <TimeSeriesChart
            title={`CPU · last ${WINDOW_MINUTES}m`}
            data={cpu}
            color={chartColor('blue')}
            height={220}
          />
          <TimeSeriesChart
            title={`Memory · last ${WINDOW_MINUTES}m`}
            data={memory}
            color={chartColor('purple')}
            height={220}
          />
          <TimeSeriesChart
            title={`Network I/O · last ${WINDOW_MINUTES}m`}
            data={network ?? []}
            color={chartColor('green')}
            unit=" MB/s"
            domain={[0, 60]}
            height={220}
          />
          <TimeSeriesChart
            title={`Filesystem · last ${WINDOW_MINUTES}m`}
            data={fs ?? []}
            color={chartColor('yellow')}
            height={220}
          />
        </div>
      </div>
    </div>
  )
}
