import { useTranslation } from 'react-i18next'
import { reasonMessageText } from '@/core/reasons'
import { useProviderQuery } from '@/data/DataContext'
import { formatPercent } from '@/i18n/format'
import { Icon } from '@/shared/Icon'
import { NS } from './i18n'

/**
 * "CPU 42% · Mem 61%" item contributed to the right of the status bar;
 * "No metrics" with the reason as tooltip when nothing measured the use.
 */
export function StatusBarMetrics() {
  const { t } = useTranslation(NS)
  const { data } = useProviderQuery((p) => p.metrics.clusterOverview(), [])
  if (!data) return null

  const { cpuUsagePercent: cpu, memoryUsagePercent: memory } = data
  if (cpu === null || memory === null) {
    const reason = data.usageUnavailable ? reasonMessageText(data.usageUnavailable) : ''
    return (
      <div className="status-bar__item" title={t('statusBar.unavailableTooltip', { reason })}>
        <Icon name="circle-slash" size={13} />
        <span>{t('statusBar.unavailable')}</span>
      </div>
    )
  }
  return (
    <div className="status-bar__item" title={t('statusBar.tooltip')}>
      <Icon name="pulse" size={13} />
      <span>
        {t('statusBar.usage', {
          cpu: formatPercent(cpu / 100),
          memory: formatPercent(memory / 100),
        })}
      </span>
    </div>
  )
}
