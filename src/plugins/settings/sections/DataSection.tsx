import { useTranslation } from 'react-i18next'
import { useSettingsStore } from '@/core/store/settingsStore'
import { useProviderQuery } from '@/data/DataContext'
import { Checkbox, Field, Select, type SelectOption } from '@/shared/form'
import { SETTINGS_NS } from '../namespace'
import { SettingsSection } from '../SettingsSection'

type MetricsSource = 'auto' | 'metrics-server' | 'prometheus'

const METRICS_WINDOWS = [15, 30, 60]
const REFRESH_INTERVALS = [0, 10, 30, 60]

/** Metrics source and history window, auto-refresh of lists, live updates. */
export function DataSection() {
  const { t } = useTranslation(SETTINGS_NS)
  const metricsSource = useSettingsStore((s) => s.metricsSource)
  const metricsWindow = useSettingsStore((s) => s.metricsWindow)
  const autoRefreshSeconds = useSettingsStore((s) => s.autoRefreshSeconds)
  const liveUpdates = useSettingsStore((s) => s.liveUpdates)
  const setSetting = useSettingsStore((s) => s.set)
  const { data: sources } = useProviderQuery((p) => p.metrics.listSources(), [])
  const prometheus = sources?.find((s) => s.id === 'prometheus')
  const prometheusMissing = prometheus?.available === false

  // Depends on whether Prometheus was discovered; unknown while loading.
  const sourceHelp = () => {
    if (!prometheus) return t('data.metricsSource.help')
    if (prometheusMissing) return t('data.metricsSource.helpNoPrometheus')
    return prometheus.detail
      ? t('data.metricsSource.helpPrometheusAt', { location: prometheus.detail })
      : t('data.metricsSource.helpPrometheus')
  }

  const sourceOptions: SelectOption<MetricsSource>[] = [
    { value: 'auto', label: t('data.metricsSource.auto') },
    {
      value: 'prometheus',
      label: prometheusMissing ? t('data.metricsSource.prometheusUnavailable') : t('data.metricsSource.prometheus'),
      disabled: prometheusMissing,
    },
    { value: 'metrics-server', label: t('data.metricsSource.metricsServer') },
  ]

  return (
    <SettingsSection title={t('sections.data')}>
      <Field layout="inline" label={t('data.metricsSource.label')} help={sourceHelp()}>
        <Select
          value={metricsSource}
          options={sourceOptions}
          onChange={(value) => setSetting('metricsSource', value)}
        />
      </Field>
      <Field layout="inline" label={t('data.metricsWindow.label')} help={t('data.metricsWindow.help')}>
        <Select
          value={metricsWindow}
          options={METRICS_WINDOWS.map((count) => ({
            value: count,
            label: t('data.metricsWindow.minutes', { count }),
          }))}
          onChange={(value) => setSetting('metricsWindow', value)}
        />
      </Field>
      <Field layout="inline" label={t('data.autoRefresh.label')} help={t('data.autoRefresh.help')}>
        <Select
          value={autoRefreshSeconds}
          options={REFRESH_INTERVALS.map((count) => ({
            value: count,
            label: count === 0 ? t('data.autoRefresh.off') : t('data.autoRefresh.every', { count }),
          }))}
          onChange={(value) => setSetting('autoRefreshSeconds', value)}
        />
      </Field>
      <Field layout="inline" label={t('data.liveUpdates.label')} help={t('data.liveUpdates.help')}>
        <Checkbox checked={liveUpdates} onChange={(value) => setSetting('liveUpdates', value)} />
      </Field>
    </SettingsSection>
  )
}
