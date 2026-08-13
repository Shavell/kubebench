import { useState } from 'react'
import { useProviderQuery } from '@/data/DataContext'
import { Icon } from '@/shared/Icon'
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
  const [filter, setFilter] = useState('')
  const { data, loading } = useProviderQuery(
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
          placeholder="Filter…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          spellCheck={false}
        />
        <span className="podlogs__count">{loading ? 'loading…' : `${lines.length} lines`}</span>
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
        {!loading && lines.length === 0 && <div className="podlogs__empty">No matching log lines</div>}
      </pre>
    </div>
  )
}
