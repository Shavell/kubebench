/** The container a `pod-logs` or `pod-exec` request or tab points at. */
export interface PodTarget {
  namespace: string
  pod: string
  container: string
}

/** The target in `props` (`{namespace, pod, container}`), or null when they do not name one. */
export function podTarget(props: Record<string, unknown> | undefined): PodTarget | null {
  const { namespace, pod, container } = props ?? {}
  if (typeof namespace !== 'string' || typeof pod !== 'string' || typeof container !== 'string') return null
  if (!namespace || !pod || !container) return null
  return { namespace, pod, container }
}
