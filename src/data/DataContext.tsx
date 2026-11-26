import { createContext, useContext, useEffect, useState, type DependencyList } from 'react'
import { errorMessage, isApiError } from './apiClient'
import type { DataProviders } from './providers'
import * as invalidation from './invalidation'
import type { QueryScope } from './invalidation'
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
 * Re-renders with a new version whenever the invalidation bus refreshes a
 * query of `scope` (see ./invalidation).
 */
function useRefreshVersion(scope: QueryScope | undefined): number {
  const [version, setVersion] = useState(0)
  const topics = scope?.topics?.join(',') ?? ''
  const namespace = scope?.namespace
  useEffect(() => {
    const refresh = () => setVersion((v) => v + 1)
    return invalidation.registerQuery({ topics: topics ? topics.split(',') : undefined, namespace }, refresh)
  }, [topics, namespace])
  return version
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
  const version = useRefreshVersion(scope)
  // `deps` names the inputs the state was loaded for.
  const [state, setState] = useState<{ deps: DependencyList; value: QueryState<T> }>({ deps, value: LOADING })

  useEffect(() => {
    let cancelled = false
    setState((s) => ({
      deps,
      value: sameDeps(s.deps, deps) ? { ...s.value, loading: true, error: null, errorCode: null, errorReason: null } : LOADING,
    }))
    query()
      .then((data) => {
        if (!cancelled) setState({ deps, value: { data, loading: false, error: null, errorCode: null, errorReason: null } })
      })
      .catch((err: unknown) => {
        if (cancelled) return
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
