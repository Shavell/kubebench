import { useTranslation } from 'react-i18next'
import { useProviderQuery } from '@/data/DataContext'
import { formatPercent } from '@/i18n/format'
import { Icon } from '@/shared/Icon'
import { NS } from './i18n'

/** "CPU 42% · Mem 61%" item contributed to the right of the status bar. */
export function StatusBarMetrics() {
  const { t } = useTranslation(NS)
  const { data } = useProviderQuery((p) => p.metrics.clusterOverview(), [])
  if (!data) return null

  return (
    <div className="status-bar__item" title={t('statusBar.tooltip')}>
      <Icon name="pulse" size={13} />
      <span>
        {t('statusBar.usage', {
          cpu: formatPercent(data.cpuUsagePercent / 100),
          memory: formatPercent(data.memoryUsagePercent / 100),
        })}
      </span>
    </div>
  )
}
