import type { QueryState } from '@/data/DataContext'
import type { MetricPoint } from '@/data/types'
import { Icon } from '@/shared/Icon'
import { TimeSeriesChart } from '@/shared/charts/TimeSeriesChart'

interface MetricChartProps {
  title: string
  series: QueryState<MetricPoint[]>
  color: string
  unit?: string
  domain?: [number, number]
  height?: number
}

/**
 * Chart for one metric series query. When the series cannot be loaded
 * (live mode, e.g. the metrics source keeps no history for this metric)
 * the chart frame shows the reason instead of an empty plot.
 */
export function MetricChart({ title, series, ...chart }: MetricChartProps) {
  if (series.error) {
    return (
      <div className="kb-chart metrics-chart--unavailable">
        <div className="kb-chart__title">{title}</div>
        <div className="metrics-chart__reason">
          <Icon name="warning" size={14} />
          <span>{series.error}</span>
        </div>
      </div>
    )
  }
  return <TimeSeriesChart title={title} data={series.data ?? []} {...chart} />
}
