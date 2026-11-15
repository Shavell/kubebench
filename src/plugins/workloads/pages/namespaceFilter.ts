/**
 * The page-local namespace filter of the list pages: '' follows the global
 * status-bar filter, a name scopes the list to that namespace, and
 * LOCAL_ALL_NAMESPACES lists all namespaces whatever the global filter
 * (palette `get <kind> -A`). '*' is never a namespace name.
 */
export const LOCAL_ALL_NAMESPACES = '*'

/** The initial local filter from a list tab's props (`namespace`, `allNamespaces`). */
export function initialLocalNamespace(props: Record<string, unknown> | undefined): string {
  if (props?.allNamespaces === true) return LOCAL_ALL_NAMESPACES
  return typeof props?.namespace === 'string' ? props.namespace : ''
}

/** The namespace to list: undefined = all namespaces. `globalNamespace` '' = all. */
export function listNamespace(localNs: string, globalNamespace: string): string | undefined {
  if (localNs === LOCAL_ALL_NAMESPACES) return undefined
  return localNs || globalNamespace || undefined
}
