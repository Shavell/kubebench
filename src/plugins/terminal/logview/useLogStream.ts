import { useEffect, useRef, useState } from 'react'
import type { ApiError } from '@/data/apiClient'
import { useProviders } from '@/data/DataContext'
import type { LogStream, LogStreamRequest } from '@/data/streams'
import { parseLine, timestampKey, timestampMillis, type LogEntry } from './logLine'

/** Lines a view keeps; older ones are dropped (ring buffer). */
export const MAX_LOG_LINES = 10_000
/** Waits before the 1st, 2nd, … reconnect after a broken stream. */
const RETRY_DELAYS_MS = [1000, 2000, 4000, 8000, 16000, 30000]
/** Extra seconds asked for when resuming, against clock skew; the overlap is dropped. */
const RESUME_MARGIN_SECONDS = 5

export type LogStreamStatus =
  | { state: 'connecting' }
  | { state: 'live' }
  /** Every followed log ended (the container exited, or previous logs were read). */
  | { state: 'ended' }
  /** The connection broke or the server is unavailable; a reconnect is scheduled. */
  | { state: 'retrying'; delayMs: number; error: ApiError }
  /** The server refused the stream; only a manual reconnect retries. */
  | { state: 'failed'; error: ApiError }

/** What the stream follows and how it starts; any change starts over. */
export type LogStreamParams = Omit<LogStreamRequest, 'timestamps' | 'sinceSeconds'>

export interface LogStreamState {
  entries: LogEntry[]
  status: LogStreamStatus
  /** The followed containers (`pod/container`) and the number of matching pods, once open. */
  containers: string[]
  matched: number
  /** Empties the view; the stream goes on. */
  clear: () => void
  /** Starts over with a fresh stream. */
  reconnect: () => void
}

/** Broken connections and unavailable servers heal by themselves; refusals do not. */
const retryable = (error: ApiError) => error.status === 0 || error.status >= 500

/**
 * Follows a log stream into a ring buffer of MAX_LOG_LINES entries. Lines
 * are batched per animation frame. A broken stream reconnects with
 * backoff and resumes after its last line: the stream always carries
 * timestamps, so the resumed request asks for the seconds since then and
 * drops the lines it had already shown.
 */
export function useLogStream(params: LogStreamParams): LogStreamState {
  const providers = useProviders()
  const [entries, setEntries] = useState<LogEntry[]>([])
  const [status, setStatus] = useState<LogStreamStatus>({ state: 'connecting' })
  const [sources, setSources] = useState<{ containers: string[]; matched: number }>({ containers: [], matched: 0 })
  const [generation, setGeneration] = useState(0)
  const seq = useRef(0)
  const key = JSON.stringify(params)

  // A new stream (other params, a reconnect or other providers) starts from
  // an empty view; reset while rendering, before the effect opens it.
  const [streamOf, setStreamOf] = useState({ providers, key, generation })
  if (streamOf.providers !== providers || streamOf.key !== key || streamOf.generation !== generation) {
    setStreamOf({ providers, key, generation })
    setEntries([])
    setSources({ containers: [], matched: 0 })
    setStatus({ state: 'connecting' })
  }

  useEffect(() => {
    const request = JSON.parse(key) as LogStreamParams
    const prefixed = Boolean(request.selector)
    // Per source ("" for one pod): the newest line's timestamp key and time.
    const lastKey = new Map<string, string>()
    const lastMillis = new Map<string, number>()
    // Sources whose resumed stream is still within the overlap.
    let overlapping = new Set<string>()
    let stream: LogStream | null = null
    let retry: ReturnType<typeof setTimeout> | undefined
    let attempt = 0
    let pending: LogEntry[] = []
    let frame = 0

    const flush = () => {
      frame = 0
      const batch = pending
      pending = []
      setEntries((current) => {
        const next = current.concat(batch)
        return next.length > MAX_LOG_LINES ? next.slice(next.length - MAX_LOG_LINES) : next
      })
    }
    const push = (entry: Omit<LogEntry, 'seq'>) => {
      pending.push({ ...entry, seq: ++seq.current })
      // Hidden tabs get no animation frames: keep the backlog bounded too.
      if (pending.length > MAX_LOG_LINES) pending = pending.slice(pending.length - MAX_LOG_LINES)
      frame ||= requestAnimationFrame(flush)
    }

    const connect = (resume: boolean) => {
      // The first connect starts from the 'connecting' state set on render.
      if (resume) setStatus({ state: 'connecting' })
      const oldest = Math.min(...lastMillis.values())
      const since =
        resume && Number.isFinite(oldest)
          ? Math.max(1, Math.ceil((Date.now() - oldest) / 1000) + RESUME_MARGIN_SECONDS)
          : undefined
      overlapping = since ? new Set(lastKey.keys()) : new Set()
      stream = providers.cluster.streamLogs(
        { ...request, timestamps: true, ...(since ? { tail: 0, sinceSeconds: since } : {}) },
        {
          onOpen: (containers, matched) => {
            attempt = 0
            setSources({ containers, matched })
            setStatus({ state: 'live' })
            if (since) push({ kind: 'notice', text: 'logs.notice.resumed', level: null })
          },
          onPods: (containers, matched) => setSources({ containers, matched }),
          onLine: (raw) => {
            const line = parseLine(raw, prefixed)
            const source = line.source ?? ''
            const tsKey = line.ts ? timestampKey(line.ts) : null
            if (tsKey && line.ts) {
              if (overlapping.has(source)) {
                if (tsKey <= (lastKey.get(source) ?? '')) return // shown before the break
                overlapping.delete(source)
              }
              lastKey.set(source, tsKey)
              lastMillis.set(source, timestampMillis(line.ts))
            }
            push({ kind: 'line', ...line })
          },
          onError: (message) => push({ kind: 'error', text: message, level: 'error' }),
          onEnd: () => {
            stream = null
            push({ kind: 'notice', text: 'logs.notice.ended', level: null })
            setStatus({ state: 'ended' })
          },
          onFailure: (error) => {
            stream = null
            if (!retryable(error)) {
              setStatus({ state: 'failed', error })
              return
            }
            const delayMs = RETRY_DELAYS_MS[Math.min(attempt++, RETRY_DELAYS_MS.length - 1)]
            setStatus({ state: 'retrying', delayMs, error })
            retry = setTimeout(() => connect(true), delayMs)
          },
        },
      )
    }
    connect(false)

    return () => {
      stream?.close()
      clearTimeout(retry)
      cancelAnimationFrame(frame)
    }
  }, [providers, key, generation])

  return {
    entries,
    status,
    containers: sources.containers,
    matched: sources.matched,
    clear: () => setEntries([]),
    reconnect: () => setGeneration((n) => n + 1),
  }
}
