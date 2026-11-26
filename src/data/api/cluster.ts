import { apiDelete, apiGet, apiGetList, apiPost, type QueryParams } from '../apiClient'
import type { ClusterProvider } from '../providers'
import type { LiveCapabilities } from '../types'
import { apiEditing } from './editing'
import { streamLogs } from './logStream'
import { openTerminal, refusedTerminal } from './terminal'
import { capabilityUnavailable, rejectUnavailable } from './unavailable'

/**
 * GET /api/k8s/<path> (design §4.5). A query string written into `path`
 * (`/api/v1/pods?labelSelector=app%3Dweb`) is merged into `params`, so
 * every parameter is encoded exactly once by qs().
 */
function k8sGet<T>(path: string, params?: QueryParams): Promise<T> {
  const at = path.indexOf('?')
  const bare = at < 0 ? path : path.slice(0, at)
  const inline = at < 0 ? {} : Object.fromEntries(new URLSearchParams(path.slice(at + 1)))
  return apiGet<T>(`/k8s${bare.startsWith('/') ? bare : `/${bare}`}`, { ...inline, ...params })
}

export function createApiCluster(capabilities: LiveCapabilities): ClusterProvider {
  return {
    listNodes: (options) => apiGetList('/nodes', { limit: options?.limit }),
    listNamespaces: (options) => apiGetList('/namespaces', { limit: options?.limit }),
    listPods: (ns, options) => apiGetList('/pods', { ns, limit: options?.limit }),
    listDeployments: (ns, options) => apiGetList('/deployments', { ns, limit: options?.limit }),
    listServices: (ns, options) => apiGetList('/services', { ns, limit: options?.limit }),
    countResource: async (topic, ns) => (await apiGet<{ count: number }>('/count', { topic, ns })).count,
    getResourceYaml: async (kind, ns, name) =>
      (await apiGet<{ yaml: string }>('/yaml', { kind, ns, name })).yaml,
    // kind keeps objects of different kinds with the same name apart (§4.4).
    listEvents: (kind, ns, name) => apiGet('/events', { kind, ns, name }),
    getPodLogs: (ns, pod, container, options = {}) =>
      apiGet('/logs', {
        ns,
        pod,
        container,
        tail: options.tail,
        previous: options.previous,
        sinceSeconds: options.sinceSeconds,
      }),
    listResource: (key, ns, options) => apiGetList('/resource', { key, ns, limit: options?.limit }),
    getResource: (key, ns, name) => apiGet('/resource/get', { key, ns, name }),
    execCommand: (ns, pod, container, cmd) =>
      capabilities.exec
        ? apiPost('/exec', undefined, { ns, pod, container, cmd })
        : rejectUnavailable('exec'),
    streamLogs,
    openTerminal: (target, handlers) =>
      capabilities.exec
        ? openTerminal(target, handlers)
        : refusedTerminal(handlers, capabilityUnavailable('exec').message),
    k8sGet,
    deletePod: async (ns, name) => {
      await apiDelete('/pods', { ns, name })
    },
    scaleDeployment: async (ns, name, replicas) => {
      await apiPost('/deployments/scale', undefined, { ns, name, replicas })
    },
    restartDeployment: async (ns, name) => {
      await apiPost('/deployments/restart', undefined, { ns, name })
    },
    ...apiEditing,
  }
}
