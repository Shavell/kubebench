import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useProviderQuery } from '@/data/DataContext'
import { CORE_NS } from '@/i18n'
import { formatNumber } from '@/i18n/format'
import { Icon } from '@/shared/Icon'
import { NS } from './i18n'
import './PodLogsTab.css'

const LEVEL_PATTERNS: [RegExp, string][] = [
  [/\b(ERROR|FATAL)\b|level=error/, 'podlogs__line--error'],
  [/\bWARN(ING)?\b|level=warn/, 'podlogs__line--warn'],
]

/** Reusable log viewer for one container; used by the editor tab and the
 *  bottom panel. `compact` trims the toolbar for the panel. */
export function LogsView({
  namespace,
  pod,
  container,
  compact,
}: {
  namespace: string
  pod: string
  container: string
  compact?: boolean
}) {
  const { t } = useTranslation(NS)
  const [filter, setFilter] = useState('')
  const { data, loading, error } = useProviderQuery(
    (p) => p.cluster.getPodLogs(namespace, pod, container),
    [namespace, pod, container],
  )

  const lines = (data ?? []).filter(
    (line) => !filter || line.toLowerCase().includes(filter.toLowerCase()),
  )

  return (
    <div className="podlogs">
      <div className="podlogs__toolbar">
        <Icon name="output" size={14} />
        <span className="podlogs__target">
          {compact ? `${pod} · ${container}` : `${namespace}/${pod} · ${container}`}
        </span>
        <input
          className="podlogs__filter"
          placeholder={t('common.filter', { ns: CORE_NS })}
          aria-label={t('logs.filterLabel')}
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          spellCheck={false}
        />
        <span className="podlogs__count">
          {loading
            ? t('logs.loading')
            : t('logs.lines', { count: lines.length, lines: formatNumber(lines.length) })}
        </span>
      </div>
      <pre className="podlogs__body">
        {lines.map((line, i) => {
          const cls = LEVEL_PATTERNS.find(([re]) => re.test(line))?.[1]
          return (
            <div key={i} className={cls}>
              {line}
            </div>
          )
        })}
        {!loading && lines.length === 0 && (
          <div className="podlogs__empty">
            {error ? t('logs.error', { message: error }) : t('logs.empty')}
          </div>
        )}
      </pre>
    </div>
  )
}
