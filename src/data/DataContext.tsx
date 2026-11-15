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

const LOADING: QueryState<never> = { data: null, loading: true, error: null, errorCode: null }

function sameDeps(a: DependencyList, b: DependencyList): boolean {
  return a.length === b.length && a.every((value, i) => Object.is(value, b[i]))
}

/**
 * Runs an async query, re-running it when `deps` change or when
 * refreshData() is called. A refresh keeps the previous data while
 * reloading; new `deps` never show the data of the old ones (a page reused
 * for another resource must not render the old rows with its new columns).
 */
export function useQuery<T>(query: () => Promise<T>, deps: DependencyList): QueryState<T> {
  const version = useDataVersion()
  // `deps` names the inputs the state was loaded for.
  const [state, setState] = useState<{ deps: DependencyList; value: QueryState<T> }>({ deps, value: LOADING })

  useEffect(() => {
    let cancelled = false
    setState((s) => ({
      deps,
      value: sameDeps(s.deps, deps) ? { ...s.value, loading: true, error: null, errorCode: null } : LOADING,
    }))
    query()
      .then((data) => {
        if (!cancelled) setState({ deps, value: { data, loading: false, error: null, errorCode: null } })
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
): QueryState<T> {
  const providers = useProviders()
  return useQuery(() => query(providers), [providers, ...deps])
}
