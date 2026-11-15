import { useTranslation } from 'react-i18next'
import { useProviderQuery, type QueryState } from '@/data/DataContext'
import { useSettingsStore } from '@/core/store/settingsStore'
import type { TabDescriptor } from '@/core/plugin/types'
import { chartColor } from '@/shared/charts/chartTheme'
import { MetricChart } from '@/shared/charts/MetricChart'
import { Toolbar } from '@/shared/Toolbar'
import { NS } from '../i18n'
import '@/shared/tabPage.css'
import './metrics.css'

/** A query has finished: it produced data or failed with a reason. */
const settled = (q: QueryState<unknown>) => q.data !== null || q.error !== null

export function NodeMetricsPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const node = tab.props?.node as string
  const minutes = useSettingsStore((s) => s.metricsWindow)
  const cpu = useProviderQuery((p) => p.metrics.nodeSeries(node, 'cpu', minutes), [node, minutes])
  const memory = useProviderQuery((p) => p.metrics.nodeSeries(node, 'memory', minutes), [node, minutes])
  const network = useProviderQuery((p) => p.metrics.nodeSeries(node, 'network', minutes), [node, minutes])
  const fs = useProviderQuery((p) => p.metrics.nodeSeries(node, 'fs', minutes), [node, minutes])

  if (!settled(cpu) || !settled(memory)) {
    return <div className="page-loading">{t('node.loading', { node })}</div>
  }

  return (
    <div className="page">
      <Toolbar title={t('node.title', { node })} />
      <div className="metrics-page">
        <div className="metrics-charts">
          <MetricChart
            title={t('charts.cpu', { minutes })}
            series={cpu}
            color={chartColor('blue')}
            height={220}
          />
          <MetricChart
            title={t('charts.memory', { minutes })}
            series={memory}
            color={chartColor('purple')}
            height={220}
          />
          <MetricChart
            title={t('charts.network', { minutes })}
            series={network}
            color={chartColor('green')}
            unit={` ${t('units.megabytesPerSecond')}`}
            domain={[0, 60]}
            height={220}
          />
          <MetricChart
            title={t('charts.filesystem', { minutes })}
            series={fs}
            color={chartColor('yellow')}
            height={220}
          />
        </div>
      </div>
    </div>
  )
}
