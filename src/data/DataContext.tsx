import { createContext, useContext, useEffect, useRef, useState, type DependencyList } from 'react'
import { errorMessage, isApiError } from './apiClient'
import type { DataProviders } from './providers'
import * as invalidation from './invalidation'
import type { QueryHandle, QueryScope } from './invalidation'
import type { WatchChange } from './types'

export type { QueryScope } from './invalidation'

/** Re-runs every mounted query, e.g. after a change the UI made. */
export function refreshData(): void {
  invalidation.refreshData()
}

/** Re-runs the queries that read the changed topics (see ./invalidation). */
export function refreshTopics(changes: readonly WatchChange[]): void {
  invalidation.refreshTopics(changes)
}

export const DataContext = createContext<DataProviders | null>(null)

/**
 * Whether the queries below can be seen: a container that keeps hidden
 * pages mounted (the side editor group, the editors behind a maximized
 * panel) provides false for them, so live updates mark them stale instead
 * of re-reading (see ./invalidation). Default: shown.
 */
export const QueryActivityContext = createContext(true)

export function useProviders(): DataProviders {
  const providers = useContext(DataContext)
  if (!providers) throw new Error('DataContext.Provider is missing')
  return providers
}

export interface QueryState<T> {
  data: T | null
  loading: boolean
  /** Message of the last failure; null when the last run succeeded. */
  error: string | null
  /** ApiError code of the last failure (e.g. `capability_unavailable`), when known. */
  errorCode: string | null
  /** Reason code of the last failure (e.g. `kubernetes.forbidden_namespace`), when the server gave one. */
  errorReason: string | null
}

const LOADING: QueryState<never> = { data: null, loading: true, error: null, errorCode: null, errorReason: null }

function sameDeps(a: DependencyList, b: DependencyList): boolean {
  return a.length === b.length && a.every((value, i) => Object.is(value, b[i]))
}

/**
 * Registers the query with the invalidation bus (see ./invalidation) and
 * re-renders with a new version whenever the bus refreshes it. The handle
 * reports the query's reads, which paces live refreshes.
 */
function useRefreshVersion(scope: QueryScope | undefined): { version: number; handle: { current: QueryHandle | null } } {
  const [version, setVersion] = useState(0)
  const handle = useRef<QueryHandle | null>(null)
  const active = useContext(QueryActivityContext)
  const topics = scope?.topics?.join(',') ?? ''
  const namespace = scope?.namespace
  useEffect(() => {
    const refresh = () => setVersion((v) => v + 1)
    const registered = invalidation.registerQuery({ topics: topics ? topics.split(',') : undefined, namespace }, refresh)
    handle.current = registered
    return () => {
      registered.unregister()
      if (handle.current === registered) handle.current = null
    }
  }, [topics, namespace])
  useEffect(() => {
    handle.current?.setActive(active)
  }, [active, topics, namespace])
  return { version, handle }
}

/**
 * Runs an async query, re-running it when `deps` change or when
 * refreshData() is called. `scope` names the resources the query reads
 * (topics like `pods`, optionally one namespace), so that live updates
 * (refreshTopics) re-run only the queries a change concerns; queries
 * without topics re-run on refreshData() and auto-refresh only. A refresh
 * keeps the previous data while reloading; new `deps` never show the data
 * of the old ones (a page reused for another resource must not render the
 * old rows with its new columns).
 */
export function useQuery<T>(query: () => Promise<T>, deps: DependencyList, scope?: QueryScope): QueryState<T> {
  const { version, handle } = useRefreshVersion(scope)
  // `deps` names the inputs the state was loaded for.
  const [state, setState] = useState<{ deps: DependencyList; value: QueryState<T> }>({ deps, value: LOADING })

  useEffect(() => {
    let cancelled = false
    setState((s) => ({
      deps,
      value: sameDeps(s.deps, deps) ? { ...s.value, loading: true, error: null, errorCode: null, errorReason: null } : LOADING,
    }))
    const reads = handle.current
    reads?.begin()
    query()
      .then((data) => {
        if (cancelled) return
        reads?.end()
        setState({ deps, value: { data, loading: false, error: null, errorCode: null, errorReason: null } })
      })
      .catch((err: unknown) => {
        if (cancelled) return
        reads?.end()
        setState({
          deps,
          value: {
            data: null,
            loading: false,
            error: errorMessage(err),
            errorCode: isApiError(err) ? err.code : null,
            errorReason: (isApiError(err) && err.reason) || null,
          },
        })
      })
    return () => {
      cancelled = true
    }
    // `query` is a fresh closure every render; callers list its inputs in `deps`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, version])

  return sameDeps(state.deps, deps) ? state.value : LOADING
}

/** useQuery over the core data providers, so plugin UIs never touch fetch mechanics. */
export function useProviderQuery<T>(
  query: (providers: DataProviders) => Promise<T>,
  deps: DependencyList,
  scope?: QueryScope,
): QueryState<T> {
  const providers = useProviders()
  return useQuery(() => query(providers), [providers, ...deps], scope)
}
