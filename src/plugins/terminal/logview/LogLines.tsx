import clsx from 'clsx'
import { memo, useLayoutEffect, useMemo, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { formatLocale } from '@/i18n/format'
import { Icon } from '@/shared/Icon'
import { NS } from '../i18n'
import { timestampMillis, type LogEntry } from './logLine'

/** Entries per rendered chunk; only the chunks whose entries change re-render. */
const CHUNK = 200
/** Distinct colours for the `[pod/container]` prefixes (chart tokens). */
const SOURCE_COLOURS = 6

interface LogLinesProps {
  entries: LogEntry[]
  timestamps: boolean
  wrap: boolean
  colorize: boolean
  /** Keeps the newest line in view, but only while the view is scrolled to the bottom. */
  follow: boolean
  /** Shown instead of lines when there are none. */
  empty: ReactNode
}

/**
 * The lines of a log view. Lines are grouped in chunks that the browser
 * skips while off screen (content-visibility) and that React skips when
 * unchanged, so a full 10 000-line buffer stays cheap to update. The view
 * options are classes on the container: toggling them renders nothing anew.
 */
export function LogLines({ entries, timestamps, wrap, colorize, follow, empty }: LogLinesProps) {
  const { t } = useTranslation(NS)
  const scrollRef = useRef<HTMLDivElement>(null)
  const atBottom = useRef(true)
  const [scrolledUp, setScrolledUp] = useState(false)
  const chunks = useChunks(entries)

  useLayoutEffect(() => {
    const el = scrollRef.current
    if (el && follow && atBottom.current) el.scrollTop = el.scrollHeight
  }, [entries, follow])

  const onScroll = () => {
    const el = scrollRef.current
    if (!el) return
    atBottom.current = el.scrollHeight - el.scrollTop - el.clientHeight < 24
    setScrolledUp(!atBottom.current)
  }
  const toBottom = () => {
    const el = scrollRef.current
    if (!el) return
    atBottom.current = true
    el.scrollTop = el.scrollHeight
  }

  return (
    <div className="kb-logs__frame">
      <div
        ref={scrollRef}
        className={clsx(
          'kb-logs__body',
          wrap && 'kb-logs__body--wrap',
          colorize && 'kb-logs__body--colours',
          !timestamps && 'kb-logs__body--no-timestamps',
        )}
        onScroll={onScroll}
        role="log"
        aria-label={t('logs.linesLabel')}
        tabIndex={0}
      >
        {entries.length === 0 ? (
          <div className="kb-logs__empty">{empty}</div>
        ) : (
          chunks.map((chunk) => <LogChunk key={chunk.id} entries={chunk.entries} />)
        )}
      </div>
      {scrolledUp && entries.length > 0 && (
        <button type="button" className="kb-logs__jump" title={t('logs.toBottom')} onClick={toBottom}>
          <Icon name="arrow-down" size={14} />
          {t('logs.toBottom')}
        </button>
      )}
    </div>
  )
}

/**
 * The entries grouped by chunk. Each render builds new arrays; `LogChunk`
 * compares them by content (`sameChunk`), so an unchanged chunk does not
 * re-render.
 */
function useChunks(entries: LogEntry[]): { id: number; entries: LogEntry[] }[] {
  return useMemo(() => {
    const groups = new Map<number, LogEntry[]>()
    for (const entry of entries) {
      const id = Math.floor(entry.seq / CHUNK)
      const group = groups.get(id)
      if (group) group.push(entry)
      else groups.set(id, [entry])
    }
    return [...groups].map(([id, group]) => ({ id, entries: group }))
  }, [entries])
}

/**
 * Whether two groupings of one chunk hold the same entries. Entries are
 * immutable and numbered consecutively, so the same length, first and last
 * entry mean the same lines.
 */
function sameChunk(a: { entries: LogEntry[] }, b: { entries: LogEntry[] }): boolean {
  const x = a.entries
  const y = b.entries
  return x.length === y.length && x[0] === y[0] && x[x.length - 1] === y[y.length - 1]
}

const LogChunk = memo(function LogChunk({ entries }: { entries: LogEntry[] }) {
  const { t } = useTranslation(NS)
  return (
    <div className="kb-logs__chunk">
      {entries.map((entry) =>
        entry.kind === 'notice' ? (
          <div key={entry.seq} className="logline logline--notice">
            {t(entry.text)}
          </div>
        ) : (
          <div
            key={entry.seq}
            className={clsx('logline', entry.kind === 'error' && 'logline--failure', entry.level && `logline--${entry.level}`)}
          >
            {entry.source && (
              <span className={`logline__source logline__source--${sourceColour(entry.source)}`}>[{entry.source}] </span>
            )}
            {entry.ts && (
              <span className="logline__ts" title={entry.ts}>
                {formatTime(entry.ts)}{' '}
              </span>
            )}
            {entry.text}
          </div>
        ),
      )}
    </div>
  )
}, sameChunk)

function sourceColour(source: string): number {
  let hash = 0
  for (let i = 0; i < source.length; i++) hash = (hash * 31 + source.charCodeAt(i)) | 0
  return Math.abs(hash) % SOURCE_COLOURS
}

const timeFormats = new Map<string, Intl.DateTimeFormat>()

/** A line's time of day in the UI language, to the millisecond. */
function formatTime(ts: string): string {
  const millis = timestampMillis(ts)
  if (Number.isNaN(millis)) return ts
  const locale = formatLocale()
  let format = timeFormats.get(locale)
  if (!format) {
    format = new Intl.DateTimeFormat(locale, {
      hour: '2-digit',
      minute: '2-digit',
      second: '2-digit',
      fractionalSecondDigits: 3,
      hour12: false,
    })
    timeFormats.set(locale, format)
  }
  return format.format(millis)
}
