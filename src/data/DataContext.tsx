import {
  createContext,
  useContext,
  useEffect,
  useState,
  useSyncExternalStore,
  type DependencyList,
} from 'react'
import { errorMessage, isApiError } from './apiClient'
import type { DataProviders } from './providers'

// Simple invalidation bus: mutations call refreshData() and every query
// hook (core providers and plugin data alike) re-runs.
let dataVersion = 0
const versionListeners = new Set<() => void>()

export function refreshData(): void {
  dataVersion++
  versionListeners.forEach((listener) => listener())
}

function useDataVersion(): number {
  return useSyncExternalStore(
    (onChange) => {
      versionListeners.add(onChange)
      return () => versionListeners.delete(onChange)
    },
    () => dataVersion,
  )
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
}

/**
 * Runs an async query, re-running it when `deps` change or when
 * refreshData() is called. Keeps the previous data while reloading.
 */
export function useQuery<T>(query: () => Promise<T>, deps: DependencyList): QueryState<T> {
  const version = useDataVersion()
  const [state, setState] = useState<QueryState<T>>({
    data: null,
    loading: true,
    error: null,
    errorCode: null,
  })

  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null, errorCode: null }))
    query()
      .then((data) => {
        if (!cancelled) setState({ data, loading: false, error: null, errorCode: null })
      })
      .catch((err: unknown) => {
        if (cancelled) return
        setState({
          data: null,
          loading: false,
          error: errorMessage(err),
          errorCode: isApiError(err) ? err.code : null,
        })
      })
    return () => {
      cancelled = true
    }
    // `query` is a fresh closure every render; callers list its inputs in `deps`.
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [...deps, version])

  return state
}

/** useQuery over the core data providers, so plugin UIs never touch fetch mechanics. */
export function useProviderQuery<T>(
  query: (providers: DataProviders) => Promise<T>,
  deps: DependencyList,
): QueryState<T> {
  const providers = useProviders()
  return useQuery(() => query(providers), [providers, ...deps])
}
