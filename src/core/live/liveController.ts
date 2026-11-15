import type { ApiError } from '@/data/apiClient'
import { getWatchedTopics, onWatchedTopicsChange, refreshData, refreshTopics } from '@/data/invalidation'
import type { LiveProvider } from '@/data/providers'
import type { WatchStream } from '@/data/streams'
import type { WatchTopicStatus } from '@/data/types'
import { setLiveStatus, type PolledTopic } from './liveStore'

/** The server's limit of topics per stream (backend maxWatchTopics). */
export const MAX_STREAM_TOPICS = 32
/** Mounting and unmounting views changes topics in bursts; wait for them to settle. */
const TOPICS_SETTLE_MS = 250
/** Reconnect backoff while the server is unreachable. */
const RECONNECT_MIN_MS = 1_000
const RECONNECT_MAX_MS = 15_000
/** After the server ended a stream (e.g. its cluster connection was rebuilt). */
const ENDED_RECONNECT_MS = 500
/** After the server refused a stream. */
const REFUSED_RETRY_MS = 60_000

/** Refusals that retrying cannot fix: the session is gone (the session guard signs out). */
const FINAL_CODES = new Set(['unauthenticated', 'session_expired', 'password_change_required'])

interface Connection {
  topics: readonly string[]
  stream: WatchStream | null
  /** The server sent the first status: the stream is open. */
  opened: boolean
  statuses: Map<string, WatchTopicStatus>
}

/**
 * Keeps one live-update stream open for the topics of the mounted queries
 * and turns its changes into scoped refreshes (design §7.10):
 * - when views need topics the stream lacks, a new stream opens and
 *   replaces the old one once it is open (no gap without a stream);
 *   unneeded topics are dropped at the next replacement;
 * - a topic whose informer just synced, and every view after a reconnect,
 *   refresh once: changes may have been missed meanwhile;
 * - an unreachable server is retried with backoff (`offline`), a refused
 *   stream after a minute (`polling`, with the reason).
 * Returns the function that stops it.
 */
export function startLiveUpdates(live: LiveProvider): () => void {
  let stopped = false
  let current: Connection | null = null
  let next: Connection | null = null
  let settleTimer: ReturnType<typeof setTimeout> | undefined
  let retryTimer: ReturnType<typeof setTimeout> | undefined
  let failures = 0
  let failure: ApiError | null = null
  /** Views may be stale: refresh them all once a stream is open again. */
  let missedChanges = false

  const publish = () => {
    if (stopped) return
    const wanted = getWatchedTopics()
    const overflow: PolledTopic[] = wanted
      .slice(MAX_STREAM_TOPICS)
      .map((topic) => ({ topic, reason: '', overflow: true }))
    if (!current?.opened) {
      const state = failure ? (failure.status === 0 ? 'offline' : 'polling') : wanted.length ? 'connecting' : 'live'
      setLiveStatus({ state, reason: failure?.message ?? null, polled: overflow, watched: [] })
      return
    }
    const polled: PolledTopic[] = []
    const watched: string[] = []
    for (const topic of current.topics) {
      const status = current.statuses.get(topic)
      if (status?.mode === 'polling') polled.push({ topic, reason: status.reason ?? '', overflow: false })
      else watched.push(topic)
    }
    setLiveStatus({
      state: watched.length ? 'live' : 'polling',
      reason: null,
      polled: [...polled, ...overflow],
      watched,
    })
  }

  const close = (conn: Connection | null) => {
    conn?.stream?.close()
  }

  const retry = (ms: number) => {
    clearTimeout(retryTimer)
    retryTimer = setTimeout(() => {
      retryTimer = undefined
      sync()
    }, ms)
  }

  const open = (topics: readonly string[]) => {
    const conn: Connection = { topics, stream: null, opened: false, statuses: new Map() }
    if (current) {
      close(next)
      next = conn
    } else {
      current = conn
    }
    conn.stream = live.watch(topics, {
      onStatus: (status) => {
        if (conn === next) {
          // The replacement is open: it takes over.
          close(current)
          current = conn
          next = null
        }
        if (conn !== current) return
        const firstOpen = !conn.opened
        conn.opened = true
        failures = 0
        failure = null
        const previous = conn.statuses.get(status.topic)
        conn.statuses.set(status.topic, status)
        if (firstOpen && missedChanges) {
          missedChanges = false
          refreshData()
        } else if (previous && previous.mode !== 'live' && status.mode === 'live') {
          // Synced after starting or polling: changes meanwhile were not reported.
          refreshTopics([{ topic: status.topic, namespace: '' }])
        }
        publish()
      },
      onChange: (change) => {
        if (conn === current) refreshTopics([change])
      },
      onEnd: () => {
        if (conn === next) {
          next = null
        } else if (conn === current) {
          current = null
          missedChanges = true
          publish()
        }
        retry(ENDED_RECONNECT_MS)
      },
      onFailure: (error) => {
        if (conn === next) {
          // The open stream goes on; try the new topics again later.
          next = null
          retry(error.status === 0 ? RECONNECT_MIN_MS : REFUSED_RETRY_MS)
          return
        }
        if (conn !== current) return
        current = null
        failure = error
        missedChanges = true
        publish()
        if (error.status === 0) {
          retry(Math.min(RECONNECT_MIN_MS * 2 ** failures, RECONNECT_MAX_MS))
          failures++
        } else if (!FINAL_CODES.has(error.code)) {
          retry(REFUSED_RETRY_MS)
        }
      },
    })
  }

  /** Opens a stream when the mounted views need topics the stream lacks. */
  const sync = () => {
    if (stopped || retryTimer) return // the pending retry syncs
    const topics = getWatchedTopics().slice(0, MAX_STREAM_TOPICS)
    const target = next ?? current
    if (topics.length === 0) {
      close(next)
      close(current)
      next = current = null
    } else if (!target || !topics.every((t) => target.topics.includes(t))) {
      open(topics)
    }
    publish()
  }

  const onTopics = () => {
    clearTimeout(settleTimer)
    settleTimer = setTimeout(sync, TOPICS_SETTLE_MS)
  }
  // Back online: reconnect now rather than at the end of the backoff.
  const onOnline = () => {
    if (failure?.status !== 0) return
    clearTimeout(retryTimer)
    retryTimer = undefined
    sync()
  }

  const unsubscribe = onWatchedTopicsChange(onTopics)
  window.addEventListener('online', onOnline)
  sync()

  return () => {
    stopped = true
    unsubscribe()
    window.removeEventListener('online', onOnline)
    clearTimeout(settleTimer)
    clearTimeout(retryTimer)
    close(next)
    close(current)
    next = current = null
  }
}
