import type { WatchChange } from './types.ts'

/**
 * The invalidation bus (design §7.10): every mounted query registers here
 * with the scope it reads. Mutations call refreshData(), which re-runs
 * every query; live updates call refreshTopics(), which re-runs only the
 * queries of the changed topics and namespaces. The union of the mounted
 * queries' topics is what the live-update stream subscribes to.
 *
 * Refreshes are paced so that a busy cluster does not keep views
 * re-reading their lists:
 * - a query that cannot be seen (its page is hidden, or the browser tab
 *   is in the background) is only marked stale, and refreshes once when it
 *   is shown again;
 * - a live refresh never overlaps a running one: changes that arrive
 *   meanwhile make one more refresh after it;
 * - a query whose live refreshes follow each other within BURST_WINDOW_MS
 *   waits longer each time (1 s, 2 s, 4 s, then every MAX_GAP_MS, and at
 *   least as long as its last read took) until changes calm down.
 * refreshData() (a change the UI made) refreshes visible queries at once.
 */

/** What a query reads, for scoped refreshes. */
export interface QueryScope {
  /**
   * The resources the query reads, as watch topics: a resource name
   * (`pods`, `deployments`) or `<resource>.<group>` for one of a group
   * (`applications.argoproj.io`), or a KubeBench-internal topic
   * (`kubebench:notes`). Without topics a query re-runs on refreshData()
   * and auto-refresh only.
   */
  topics?: readonly string[]
  /** The one namespace the query reads; absent = all namespaces (or cluster-scoped). */
  namespace?: string
}

/** Live refreshes closer together than this count as one burst. */
export const BURST_WINDOW_MS = 10_000
/** The first wait within a burst; it doubles with each refresh. */
export const MIN_GAP_MS = 1_000
/** The longest wait between live refreshes of one query. */
export const MAX_GAP_MS = 5_000

/** A registered query: what the bus needs from it, and how it reports its reads. */
export interface QueryHandle {
  /** The query's page is shown (true) or hidden (false); queries start shown. */
  setActive(active: boolean): void
  /** The query started reading (after a refresh, or for new inputs). */
  begin(): void
  /** The latest read finished. */
  end(): void
  /** Removes the query. */
  unregister(): void
}

interface QueryEntry {
  scope: QueryScope
  refresh: () => void
  active: boolean
  /** A refresh was skipped while the query could not be seen. */
  stale: boolean
  /** A read is running. */
  running: boolean
  startedAt: number
  /** How long the last read took. */
  lastDuration: number
  /** Changes arrived while reading: refresh once more afterwards. */
  pending: boolean
  timer: ReturnType<typeof setTimeout> | undefined
  /** When the latest live refresh ran (0: never). */
  lastLiveAt: number
  /** Live refreshes in the current burst, beyond the first. */
  burst: number
}

const queries = new Set<QueryEntry>()
const topicListeners = new Set<() => void>()
let watchedTopics: readonly string[] = []
let documentListening = false

function updateWatchedTopics(): void {
  const union = new Set<string>()
  for (const q of queries) q.scope.topics?.forEach((t) => union.add(t))
  const next = [...union].sort()
  if (next.length === watchedTopics.length && next.every((t, i) => t === watchedTopics[i])) return
  watchedTopics = next
  topicListeners.forEach((listener) => listener())
}

function documentHidden(): boolean {
  return typeof document !== 'undefined' && document.visibilityState === 'hidden'
}

function visible(entry: QueryEntry): boolean {
  return entry.active && !documentHidden()
}

/** Shows what a query missed while it could not be seen. */
function catchUp(entry: QueryEntry): void {
  if (!entry.stale || !visible(entry)) return
  entry.stale = false
  runLive(entry)
}

function listenToDocument(): void {
  if (documentListening || typeof document === 'undefined') return
  documentListening = true
  document.addEventListener('visibilitychange', () => {
    if (!documentHidden()) queries.forEach(catchUp)
  })
}

function clearTimer(entry: QueryEntry): void {
  clearTimeout(entry.timer)
  entry.timer = undefined
}

/** The wait before the next live refresh of entry, after its last one. */
function gap(entry: QueryEntry): number {
  const backoff = entry.burst === 0 ? 0 : Math.min(MIN_GAP_MS * 2 ** (entry.burst - 1), MAX_GAP_MS)
  return Math.max(backoff, Math.min(entry.lastDuration, MAX_GAP_MS))
}

function runLive(entry: QueryEntry): void {
  clearTimer(entry)
  const now = Date.now()
  entry.burst = entry.lastLiveAt && now - entry.lastLiveAt < BURST_WINDOW_MS ? entry.burst + 1 : 0
  entry.lastLiveAt = now
  entry.running = true
  entry.refresh()
}

/** A live refresh of entry, paced as the module comment says. */
function schedule(entry: QueryEntry): void {
  if (!visible(entry)) {
    clearTimer(entry)
    entry.stale = true
    return
  }
  if (entry.running) {
    entry.pending = true
    return
  }
  if (entry.timer) return
  const wait = entry.lastLiveAt ? entry.lastLiveAt + gap(entry) - Date.now() : 0
  if (wait <= 0) {
    runLive(entry)
    return
  }
  entry.timer = setTimeout(() => {
    entry.timer = undefined
    if (queries.has(entry)) schedule(entry)
  }, wait)
}

/** Adds a mounted query; its handle reports its reads and visibility. */
export function registerQuery(scope: QueryScope, refresh: () => void): QueryHandle {
  listenToDocument()
  const entry: QueryEntry = {
    scope,
    refresh,
    active: true,
    stale: false,
    running: false,
    startedAt: 0,
    lastDuration: 0,
    pending: false,
    timer: undefined,
    lastLiveAt: 0,
    burst: 0,
  }
  queries.add(entry)
  updateWatchedTopics()
  return {
    setActive(active) {
      if (entry.active === active) return
      entry.active = active
      if (active) catchUp(entry)
      else clearTimer(entry)
    },
    begin() {
      entry.running = true
      entry.startedAt = Date.now()
    },
    end() {
      entry.running = false
      entry.lastDuration = Date.now() - entry.startedAt
      if (entry.pending) {
        entry.pending = false
        schedule(entry)
      }
    },
    unregister() {
      clearTimer(entry)
      queries.delete(entry)
      updateWatchedTopics()
    },
  }
}

/** Re-runs every mounted query, e.g. after a change the UI made; hidden ones when shown. */
export function refreshData(): void {
  queries.forEach((q) => {
    if (!visible(q)) {
      q.stale = true
      return
    }
    clearTimer(q)
    q.pending = false
    q.running = true
    q.refresh()
  })
}

function concerns(scope: QueryScope, change: WatchChange): boolean {
  if (!scope.topics?.includes(change.topic)) return false
  return change.namespace === '' || scope.namespace === undefined || scope.namespace === change.namespace
}

/**
 * Re-runs the queries that read a changed topic in a changed namespace
 * (namespace '' = all of the topic's queries), paced.
 */
export function refreshTopics(changes: readonly WatchChange[]): void {
  refreshWhere((scope) => changes.some((change) => concerns(scope, change)))
}

/** Re-runs the queries whose scope matches, paced like live updates. */
export function refreshWhere(match: (scope: QueryScope) => boolean): void {
  queries.forEach((q) => {
    if (match(q.scope)) schedule(q)
  })
}

/** The topics of the mounted queries, sorted. */
export function getWatchedTopics(): readonly string[] {
  return watchedTopics
}

/** Calls `listener` whenever getWatchedTopics() changes; returns the unsubscription. */
export function onWatchedTopicsChange(listener: () => void): () => void {
  topicListeners.add(listener)
  return () => {
    topicListeners.delete(listener)
  }
}
