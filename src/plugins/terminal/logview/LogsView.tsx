import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Unavailable } from '@/shared/Unavailable'
import { NS } from '../i18n'
import { usePodContainers } from '../usePodContainers'
import { downloadText, logFileName, logText } from './logFile'
import type { LogEntry } from './logLine'
import { LogLines } from './LogLines'
import { useLogPreferences } from './logPreferences'
import { LogsToolbar } from './LogsToolbar'
import { useLogStream, type LogStreamParams } from './useLogStream'
import './logs.css'

/** What a log view follows: one container, or the pods a label selector matches. */
export type LogsTarget =
  | { kind: 'pod'; namespace: string; pod: string; container: string }
  | { kind: 'selector'; namespace: string; selector: string; title: string }

/**
 * Live log viewer (design §7.1): follow and pause, tail size, previous
 * container, timestamps, wrap, filter, level colours, download and clear,
 * on a stream that reconnects by itself. `compact` shortens the label for
 * the bottom panel.
 */
export function LogsView({ target, compact = false }: { target: LogsTarget; compact?: boolean }) {
  const { t } = useTranslation(NS)
  const prefs = useLogPreferences()
  const [container, setContainer] = useState(target.kind === 'pod' ? target.container : '')
  const [previous, setPrevious] = useState(false)
  const [filter, setFilter] = useState('')
  // While paused the view shows this snapshot; the stream goes on behind it.
  const [frozen, setFrozen] = useState<LogEntry[] | null>(null)

  const params: LogStreamParams =
    target.kind === 'pod'
      ? { namespace: target.namespace, pod: target.pod, container, tail: prefs.tail, previous }
      : { namespace: target.namespace, selector: target.selector, tail: prefs.tail, previous }
  const stream = useLogStream(params)
  const containers = usePodContainers(target.namespace, target.kind === 'pod' ? target.pod : null, container)

  const shown = frozen ?? stream.entries
  const needle = filter.trim().toLowerCase()
  const visible = useMemo(
    () =>
      needle
        ? shown.filter(
            (e) => e.kind === 'notice' || e.text.toLowerCase().includes(needle) || e.source?.toLowerCase().includes(needle),
          )
        : shown,
    [shown, needle],
  )
  const lastShown = shown.at(-1)?.seq ?? 0
  const newer = frozen ? stream.entries.filter((e) => e.seq > lastShown && e.kind !== 'notice').length : 0
  const lineCount = (entries: LogEntry[]) => entries.reduce((n, e) => (e.kind === 'notice' ? n : n + 1), 0)

  // Following something else starts over, and follows again.
  const restart = (change: () => void) => {
    setFrozen(null)
    change()
  }

  const label =
    target.kind === 'pod'
      ? compact
        ? `${target.pod} · ${container}`
        : `${target.namespace}/${target.pod} · ${container}`
      : target.title

  const emptyText = () => {
    if (needle && stream.entries.length > 0) return t('logs.noMatch')
    switch (stream.status.state) {
      case 'connecting':
      case 'retrying':
        return t('logs.waiting')
      case 'ended':
        return t('logs.emptyLog')
      default:
        return t('logs.noLinesYet')
    }
  }

  return (
    <div className="kb-logs">
      <LogsToolbar
        label={label}
        labelTitle={target.kind === 'selector' ? `${target.namespace} · ${target.selector}` : undefined}
        pods={target.kind === 'selector' ? { containers: stream.containers, matched: stream.matched } : undefined}
        status={stream.status}
        paused={frozen !== null}
        newer={newer}
        shown={lineCount(visible)}
        total={lineCount(shown)}
        filter={filter}
        onFilter={setFilter}
        container={
          target.kind === 'pod'
            ? { value: container, options: containers, onChange: (c) => restart(() => setContainer(c)) }
            : undefined
        }
        previous={previous}
        onPrevious={(value) => restart(() => setPrevious(value))}
        onTail={(tail) => restart(() => prefs.set('tail', tail))}
        onPause={() => setFrozen(frozen ? null : stream.entries)}
        onDownload={() =>
          downloadText(
            target.kind === 'pod' ? logFileName(target.pod, container) : logFileName(target.title),
            logText(stream.entries, prefs.timestamps),
          )
        }
        onClear={() => {
          stream.clear()
          if (frozen) setFrozen([])
        }}
        onReconnect={() => restart(stream.reconnect)}
      />
      {stream.status.state === 'failed' && stream.entries.length === 0 ? (
        <Unavailable compact error={stream.status.error} onRetry={() => restart(stream.reconnect)} />
      ) : (
        <LogLines
          entries={visible}
          timestamps={prefs.timestamps}
          wrap={prefs.wrap}
          colorize={prefs.colorize}
          follow={frozen === null}
          empty={emptyText()}
        />
      )}
    </div>
  )
}
