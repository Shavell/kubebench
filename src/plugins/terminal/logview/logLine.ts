export type LogLevel = 'error' | 'warn' | 'info' | 'debug'

/** One row of the log view. */
export interface LogEntry {
  /** Grows by one per entry for the lifetime of a view; React key and chunking. */
  seq: number
  /** A log line, an error the server reported, or a marker of the view (stream ended …). */
  kind: 'line' | 'error' | 'notice'
  /** `pod/container`, for streams that follow several pods. */
  source?: string
  /** The line's RFC 3339 timestamp as sent. */
  ts?: string
  /** The message; for a notice, its i18n key. */
  text: string
  level: LogLevel | null
}

const SOURCE_PREFIX = /^\[([^\]]+)\] /
const RFC3339 = /^(\d{4}-\d\d-\d\dT\d\d:\d\d:\d\d)(?:\.(\d{1,9}))?(Z|[+-]\d\d:\d\d)$/

/**
 * Splits a line of the log stream into its `[pod/container] ` prefix
 * (streams of several pods), its timestamp (the stream always asks for
 * them) and the message, and detects the level of the message.
 */
export function parseLine(raw: string, prefixed: boolean): Omit<LogEntry, 'seq' | 'kind'> {
  let rest = raw
  let source: string | undefined
  if (prefixed) {
    const match = SOURCE_PREFIX.exec(rest)
    if (match) {
      source = match[1]
      rest = rest.slice(match[0].length)
    }
  }
  let ts: string | undefined
  const space = rest.indexOf(' ')
  const head = space < 0 ? rest : rest.slice(0, space)
  if (RFC3339.test(head)) {
    ts = head
    rest = space < 0 ? '' : rest.slice(space + 1)
  }
  return { source, ts, text: rest, level: detectLevel(rest) }
}

/** structured `level=warn`, `"level":"error"`, `severity: …` */
const STRUCTURED = /\b(?:level|lvl|severity)["']?\s*[:=]\s*["']?([a-z]+)/i
/** klog's `E1115 10:00:00.000000 …` */
const KLOG = /^([IWEF])\d{4} \d\d:\d\d:\d\d/
/** Upper-case level words (`ERROR`, `[WARNING]`, `LOG:` …) and bracketed lower-case ones (`[error]`). */
const WORDS: [LogLevel, RegExp][] = [
  ['error', /\b(?:ERROR|ERR|FATAL|CRIT|CRITICAL|PANIC|SEVERE|EMERG|ALERT)\b|\[(?:error|crit|alert|emerg)\]/],
  ['warn', /\b(?:WARN|WARNING)\b|\[warn(?:ing)?\]/],
  ['info', /\b(?:INFO|NOTICE)\b|\[(?:info|notice)\]/],
  ['debug', /\b(?:DEBUG|TRACE)\b|\[(?:debug|trace)\]/],
]
const NAMED: Record<string, LogLevel> = {
  error: 'error', err: 'error', fatal: 'error', panic: 'error', crit: 'error', critical: 'error',
  warn: 'warn', warning: 'warn',
  info: 'info', notice: 'info',
  debug: 'debug', trace: 'debug',
}
const KLOG_LEVELS: Record<string, LogLevel> = { I: 'info', W: 'warn', E: 'error', F: 'error' }

/** The level a message states, if any (structured fields first, then klog, then level words). */
export function detectLevel(text: string): LogLevel | null {
  const structured = STRUCTURED.exec(text)
  if (structured) {
    const level = NAMED[structured[1].toLowerCase()]
    if (level) return level
  }
  const klog = KLOG.exec(text)
  if (klog) return KLOG_LEVELS[klog[1]]
  return WORDS.find(([, pattern]) => pattern.test(text))?.[0] ?? null
}

/**
 * A key that orders timestamps correctly whatever their zone offset and
 * fraction length (the API trims trailing zeros): UTC, 9 fraction digits.
 */
export function timestampKey(ts: string): string | null {
  const match = RFC3339.exec(ts)
  if (!match) return null
  const seconds = Date.parse(match[1] + match[3])
  if (Number.isNaN(seconds)) return null
  return `${new Date(seconds).toISOString().slice(0, 19)}.${(match[2] ?? '').padEnd(9, '0')}`
}

/** Milliseconds since the epoch of a timestamp, or NaN. */
export function timestampMillis(ts: string): number {
  const match = RFC3339.exec(ts)
  if (!match) return Number.NaN
  return Date.parse(match[1] + match[3]) + Number(`0.${match[2] ?? '0'}`) * 1000
}
