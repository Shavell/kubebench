import {
  createContext,
  useContext,
  useEffect,
  useState,
  type DependencyList,
} from 'react'
import type { DataProviders } from './providers'

export const DataContext = createContext<DataProviders | null>(null)

export function useProviders(): DataProviders {
  const providers = useContext(DataContext)
  if (!providers) throw new Error('DataContext.Provider is missing')
  return providers
}

interface QueryState<T> {
  data: T | null
  loading: boolean
  error: string | null
}

/**
 * Minimal async-query hook so plugin UIs never touch fetch mechanics.
 * Can be swapped for TanStack Query later without touching plugins.
 */
export function useProviderQuery<T>(
  query: (providers: DataProviders) => Promise<T>,
  deps: DependencyList,
): QueryState<T> {
  const providers = useProviders()
  const [state, setState] = useState<QueryState<T>>({ data: null, loading: true, error: null })

  useEffect(() => {
    let cancelled = false
    setState((s) => ({ ...s, loading: true, error: null }))
    query(providers)
      .then((data) => {
        if (!cancelled) setState({ data, loading: false, error: null })
      })
      .catch((err: unknown) => {
        if (!cancelled) setState({ data: null, loading: false, error: String(err) })
      })
    return () => {
      cancelled = true
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, deps)

  return state
}
