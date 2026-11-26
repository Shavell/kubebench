import { useProviderQuery } from '@/data/DataContext'
import { useDataMode, type DataMode } from '@/data/mode'

/** The namespaces a picker offers on the active cluster (design §7.16). */
export interface NamespaceNames {
  names: string[]
  loading: boolean
  /** Why the namespaces could not be listed, when they could not. */
  error: string | null
  /**
   * False when the cluster's credential may not list namespaces: `names`
   * are then the cluster's allowed (or detected) namespaces, and pickers
   * also accept a typed name.
   */
  listed: boolean
}

/** Whether the session's credential may list namespaces (mock mode: yes). */
export function canListNamespaces(mode: DataMode): boolean {
  return mode.capabilities?.listNamespaces !== false
}

/**
 * The names a namespace picker offers: the cluster's namespaces, or, when
 * the credential may not list them, the namespaces the server reports for
 * it (`capabilities.namespaces`) plus `extra` (e.g. the current filter),
 * without asking the API a question it would refuse.
 */
export function useNamespaceNames(extra: readonly string[] = []): NamespaceNames {
  const mode = useDataMode()
  const listed = canListNamespaces(mode)
  const query = useProviderQuery((p) => (listed ? p.cluster.listNamespaces() : Promise.resolve(null)), [listed], {
    topics: ['namespaces'],
  })
  if (listed) {
    return { names: (query.data ?? []).map((ns) => ns.name), loading: query.loading, error: query.error, listed }
  }
  const names = [...new Set([...(mode.capabilities?.namespaces ?? []), ...extra.filter(Boolean)])]
  return { names, loading: false, error: null, listed }
}
