import { useProviderQuery } from '@/data/DataContext'
import { Icon } from '@/shared/Icon'

/** "CPU 42% | Mem 61%" item contributed to the right of the status bar. */
export function StatusBarMetrics() {
  const { data } = useProviderQuery((p) => p.metrics.clusterOverview(), [])
  if (!data) return null

  return (
    <div className="status-bar__item" title="Cluster resource usage (metrics-server)">
      <Icon name="pulse" size={13} />
      <span>
        CPU {data.cpuUsagePercent.toFixed(0)}% · Mem {data.memoryUsagePercent.toFixed(0)}%
      </span>
    </div>
  )
}
