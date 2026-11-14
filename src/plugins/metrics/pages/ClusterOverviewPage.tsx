import { useTranslation } from 'react-i18next'
import { useProviderQuery } from '@/data/DataContext'
import { useSettingsStore } from '@/core/store/settingsStore'
import type { ResourceAllocation } from '@/data/types'
import { formatNumber, formatPercent } from '@/i18n/format'
import { EmptyState } from '@/shared/EmptyState'
import { StatTile } from '@/shared/charts/StatTile'
import { UsageBar } from '@/shared/charts/UsageBar'
import { chartColor } from '@/shared/charts/chartTheme'
import { Toolbar } from '@/shared/Toolbar'
import { NS } from '../i18n'
import { MetricChart } from './MetricChart'
import '@/shared/tabPage.css'
import './metrics.css'

export function ClusterOverviewPage() {
  const { t } = useTranslation(NS)
  const minutes = useSettingsStore((s) => s.metricsWindow)
  const overview = useProviderQuery((p) => p.metrics.clusterOverview(), [])
  const { data: allocation } = useProviderQuery((p) => p.metrics.clusterAllocation(), [])
  const cpu = useProviderQuery((p) => p.metrics.clusterSeries('cpu', minutes), [minutes])
  const memory = useProviderQuery((p) => p.metrics.clusterSeries('memory', minutes), [minutes])
  const network = useProviderQuery((p) => p.metrics.clusterSeries('network', minutes), [minutes])
  const fs = useProviderQuery((p) => p.metrics.clusterSeries('fs', minutes), [minutes])

  const stats = overview.data
  if (!stats) {
    if (!overview.error) return <div className="page-loading">{t('overview.loading')}</div>
    return (
      <div className="page">
        <Toolbar title={t('overview.title')} />
        <EmptyState icon="warning" title={t('overview.unavailable')} hint={overview.error} />
      </div>
    )
  }

  // Latest filesystem sample; "—" when the source has no filesystem series.
  const diskPercent = fs.data?.at(-1)?.value
  // The backend reports CPU in "cores" (a word to translate) and memory in "Gi" (a symbol).
  const unitLabel = (a: ResourceAllocation) =>
    a.unit === 'cores' ? t('units.cores', { count: a.allocatable }) : a.unit

  return (
    <div className="page">
      <Toolbar title={t('overview.title')} />
      <div className="metrics-page">
        <div className="metrics-tiles">
          <StatTile
            icon="server"
            label={t('overview.tiles.nodes')}
            value={`${formatNumber(stats.readyNodes)}/${formatNumber(stats.nodeCount)}`}
            detail={t('overview.tiles.nodesDetail')}
          />
          <StatTile
            icon="package"
            label={t('overview.tiles.pods')}
            value={`${formatNumber(stats.runningPods)}/${formatNumber(stats.podCount)}`}
            detail={t('overview.tiles.podsDetail')}
          />
          <StatTile
            icon="dashboard"
            label={t('overview.tiles.cpu')}
            value={formatPercent(stats.cpuUsagePercent / 100)}
            detail={t('overview.tiles.cpuDetail', {
              count: stats.cpuCapacityCores,
              cores: formatNumber(stats.cpuCapacityCores),
            })}
            accent={chartColor('blue')}
          />
          <StatTile
            icon="database"
            label={t('overview.tiles.memory')}
            value={formatPercent(stats.memoryUsagePercent / 100)}
            detail={t('overview.tiles.memoryDetail', { amount: formatNumber(stats.memoryCapacityGi) })}
            accent={chartColor('purple')}
          />
          <StatTile
            icon="save"
            label={t('overview.tiles.disk')}
            value={diskPercent === undefined ? '—' : formatPercent(diskPercent / 100)}
            detail={t('overview.tiles.diskDetail')}
            accent={chartColor('yellow')}
          />
        </div>

        {allocation && (
          <div className="metrics-allocation">
            <AllocationColumn
              title={t('overview.allocation.cpu')}
              allocation={allocation.cpu}
              unit={unitLabel(allocation.cpu)}
              color={chartColor('blue')}
            />
            <AllocationColumn
              title={t('overview.allocation.memory')}
              allocation={allocation.memory}
              unit={unitLabel(allocation.memory)}
              color={chartColor('purple')}
            />
          </div>
        )}
        <div className="metrics-charts">
          <MetricChart
            title={t('charts.clusterCpu', { minutes })}
            series={cpu}
            color={chartColor('blue')}
          />
          <MetricChart
            title={t('charts.clusterMemory', { minutes })}
            series={memory}
            color={chartColor('purple')}
          />
          <MetricChart
            title={t('charts.network', { minutes })}
            series={network}
            color={chartColor('green')}
            unit={` ${t('units.megabytesPerSecond')}`}
            domain={[0, 60]}
          />
          <MetricChart
            title={t('charts.filesystem', { minutes })}
            series={fs}
            color={chartColor('yellow')}
          />
        </div>
      </div>
    </div>
  )
}

/** Usage, requests and limits of one resource against its allocatable amount. */
function AllocationColumn({
  title,
  allocation,
  unit,
  color,
}: {
  title: string
  allocation: ResourceAllocation
  unit: string
  color: string
}) {
  const { t } = useTranslation(NS)
  const bars = [
    { label: t('overview.allocation.usage'), value: allocation.usage },
    { label: t('overview.allocation.requests'), value: allocation.requests },
    { label: t('overview.allocation.limits'), value: allocation.limits },
  ]
  return (
    <div className="metrics-allocation__col">
      <div className="metrics-allocation__title">{title}</div>
      {bars.map((bar) => (
        <UsageBar
          key={bar.label}
          label={bar.label}
          value={bar.value}
          total={allocation.allocatable}
          unit={unit}
          color={color}
        />
      ))}
    </div>
  )
}
