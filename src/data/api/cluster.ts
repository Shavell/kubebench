import { apiDelete, apiGet, apiPost, type QueryParams } from '../apiClient'
import type { ClusterProvider } from '../providers'
import type { LiveCapabilities } from '../types'
import { rejectUnavailable } from './unavailable'

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
    listNodes: () => apiGet('/nodes'),
    listNamespaces: () => apiGet('/namespaces'),
    listPods: (ns) => apiGet('/pods', { ns }),
    listDeployments: (ns) => apiGet('/deployments', { ns }),
    listServices: (ns) => apiGet('/services', { ns }),
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
    listResource: (key, ns) => apiGet('/resource', { key, ns }),
    getResource: (key, ns, name) => apiGet('/resource/get', { key, ns, name }),
    execCommand: (ns, pod, container, cmd) =>
      capabilities.exec
        ? apiPost('/exec', undefined, { ns, pod, container, cmd })
        : rejectUnavailable('exec'),
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
  }
}
