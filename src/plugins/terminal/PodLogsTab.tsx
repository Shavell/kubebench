import { useState } from 'react'
import type { TabDescriptor } from '@/core/plugin/types'
import { useProviderQuery } from '@/data/DataContext'
import { Icon } from '@/shared/Icon'
import './PodLogsTab.css'

const LEVEL_PATTERNS: [RegExp, string][] = [
  [/\b(ERROR|FATAL)\b|level=error/, 'podlogs__line--error'],
  [/\bWARN(ING)?\b|level=warn/, 'podlogs__line--warn'],
]

/** Read-only log viewer for one container. kind: "pod-logs". */
export function PodLogsTab({ tab }: { tab: TabDescriptor }) {
  const namespace = tab.props?.namespace as string
  const pod = tab.props?.pod as string
  const container = tab.props?.container as string
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
          {namespace}/{pod} · {container}
        </span>
        <input
          className="podlogs__filter"
          placeholder="Filter…"
          value={filter}
          onChange={(e) => setFilter(e.target.value)}
          spellCheck={false}
        />
        <span className="podlogs__count">
          {loading ? 'loading…' : `${lines.length} lines`}
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
        {!loading && lines.length === 0 && <div className="podlogs__empty">No matching log lines</div>}
      </pre>
    </div>
  )
}
