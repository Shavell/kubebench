import type { WatchChange } from './types'

/**
 * The invalidation bus (design §7.10): every mounted query registers here
 * with the scope it reads. Mutations call refreshData(), which re-runs
 * every query; live updates call refreshTopics(), which re-runs only the
 * queries of the changed topics and namespaces. The union of the mounted
 * queries' topics is what the live-update stream subscribes to.
 */

/** What a query reads, for scoped refreshes. */
export interface QueryScope {
  /**
   * The resources the query reads, as watch topics: a resource name
   * (`pods`, `deployments`) or `<resource>.<group>` for one of a group
   * (`applications.argoproj.io`). Without topics a query re-runs on
   * refreshData() and auto-refresh only.
   */
  topics?: readonly string[]
  /** The one namespace the query reads; absent = all namespaces (or cluster-scoped). */
  namespace?: string
}

interface QueryEntry {
  scope: QueryScope
  refresh: () => void
}

const queries = new Set<QueryEntry>()
const topicListeners = new Set<() => void>()
let watchedTopics: readonly string[] = []

function updateWatchedTopics(): void {
  const union = new Set<string>()
  for (const q of queries) q.scope.topics?.forEach((t) => union.add(t))
  const next = [...union].sort()
  if (next.length === watchedTopics.length && next.every((t, i) => t === watchedTopics[i])) return
  watchedTopics = next
  topicListeners.forEach((listener) => listener())
}

/** Adds a mounted query; returns its removal. */
export function registerQuery(scope: QueryScope, refresh: () => void): () => void {
  const entry: QueryEntry = { scope, refresh }
  queries.add(entry)
  updateWatchedTopics()
  return () => {
    queries.delete(entry)
    updateWatchedTopics()
  }
}

/** Re-runs every mounted query, e.g. after a change the UI made. */
export function refreshData(): void {
  queries.forEach((q) => q.refresh())
}

function concerns(scope: QueryScope, change: WatchChange): boolean {
  if (!scope.topics?.includes(change.topic)) return false
  return change.namespace === '' || scope.namespace === undefined || scope.namespace === change.namespace
}

/**
 * Re-runs the queries that read a changed topic in a changed namespace
 * (namespace '' = all of the topic's queries).
 */
export function refreshTopics(changes: readonly WatchChange[]): void {
  refreshWhere((scope) => changes.some((change) => concerns(scope, change)))
}

/** Re-runs the queries whose scope matches. */
export function refreshWhere(match: (scope: QueryScope) => boolean): void {
  queries.forEach((q) => {
    if (match(q.scope)) q.refresh()
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
