import { apiDelete, apiGet, apiPost, type QueryParams } from '../apiClient'
import type { LiveCapabilities } from '../mode'
import type { ClusterProvider } from '../providers'
import { rejectUnavailable } from './unavailable'

export function createApiCluster(capabilities: LiveCapabilities): ClusterProvider {
  return {
    listNodes: () => apiGet('/nodes'),
    listNamespaces: () => apiGet('/namespaces'),
    listPods: (ns) => apiGet('/pods', { ns }),
    listDeployments: (ns) => apiGet('/deployments', { ns }),
    listServices: (ns) => apiGet('/services', { ns }),
    getResourceYaml: async (kind, ns, name) =>
      (await apiGet<{ yaml: string }>('/yaml', { kind, ns, name })).yaml,
    listEvents: (kind, ns, name) => apiGet('/events', { kind, ns, name }),
    getPodLogs: (ns, pod, container) => apiGet('/logs', { ns, pod, container }),
    listResource: (key, ns) => apiGet('/resource', { key, ns }),
    getResource: (key, ns, name) => apiGet('/resource/get', { key, ns, name }),
    execCommand: (ns, pod, container, cmd) =>
      capabilities.exec
        ? apiPost('/exec', undefined, { ns, pod, container, cmd })
        : rejectUnavailable('exec'),
    k8sGet: <T>(path: string, params?: QueryParams) =>
      apiGet<T>(`/k8s${path.startsWith('/') ? path : `/${path}`}`, params),
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
