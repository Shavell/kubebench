import type { PanelRequest } from '@/core/plugin/panelRequests'

/** The container a `pod-logs` or `pod-exec` panel request points at. */
export interface PodTarget {
  namespace: string
  pod: string
  container: string
}

/** The request's target, or null without a (well-formed) request. */
export function podTarget(request: PanelRequest | undefined): PodTarget | null {
  const { namespace, pod, container } = request?.props ?? {}
  if (typeof namespace !== 'string' || typeof pod !== 'string' || typeof container !== 'string') return null
  return { namespace, pod, container }
}
