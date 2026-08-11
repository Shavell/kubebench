import type { DataProviders } from '../providers'
import type { MetricKind, MetricPoint } from '../types'
import { createMockProviders } from '../mock'
import { generateSeries } from '../mock/mockMetrics'

/**
 * Real providers backed by the local proxy server (server/index.mjs),
 * which talks to the cluster from the kubeconfig.
 *
 * Not everything is live yet — limited by the current kubeconfig's RBAC:
 * - helm & crd: mock (secrets/CRD list are forbidden)
 * - network/fs metrics: mock series (metrics-server has no such data)
 * Swap those pieces as broader credentials arrive.
 */

async function api<T>(path: string, init?: RequestInit): Promise<T> {
  const res = await fetch(`/api${path}`, init)
  const body = await res.json().catch(() => null)
  if (!res.ok) {
    throw new Error(body?.error ?? `${res.status} ${res.statusText}`)
  }
  return body as T
}

function liveOrMockSeries(
  metric: MetricKind,
  mockKey: string,
  minutes: number,
  livePath: string,
): Promise<MetricPoint[]> {
  if (metric === 'network' || metric === 'fs') {
    return Promise.resolve(generateSeries(mockKey, metric, minutes))
  }
  return api<MetricPoint[]>(livePath)
}

export function createApiProviders(): DataProviders {
  const mock = createMockProviders()

  return {
    cluster: {
      listNodes: () => api('/nodes'),
      listNamespaces: () => api('/namespaces'),
      listPods: (ns) => api(`/pods${ns ? `?ns=${ns}` : ''}`),
      listDeployments: (ns) => api(`/deployments${ns ? `?ns=${ns}` : ''}`),
      listServices: (ns) => api(`/services${ns ? `?ns=${ns}` : ''}`),
      getResourceYaml: async (kind, ns, name) => {
        const { yaml } = await api<{ yaml: string }>(
          `/yaml?kind=${kind}&ns=${ns ?? ''}&name=${name}`,
        )
        return yaml
      },
      listEvents: (_kind, ns, name) => api(`/events?ns=${ns ?? ''}&name=${name}`),
      getPodLogs: (ns, pod, container) =>
        api(`/logs?ns=${ns}&pod=${pod}&container=${container}`),
      deletePod: async (ns, name) => {
        await api(`/pods?ns=${ns}&name=${name}`, { method: 'DELETE' })
      },
      scaleDeployment: async (ns, name, replicas) => {
        await api(`/deployments/scale?ns=${ns}&name=${name}&replicas=${replicas}`, {
          method: 'POST',
        })
      },
      restartDeployment: async (ns, name) => {
        await api(`/deployments/restart?ns=${ns}&name=${name}`, { method: 'POST' })
      },
    },
    crd: mock.crd,
    helm: mock.helm,
    inventory: {
      listClusters: () => api('/clusters'),
      addCluster: async ({ name, kubeconfigPath, address }) => {
        const params = new URLSearchParams({ name })
        if (kubeconfigPath) params.set('path', kubeconfigPath)
        if (address) params.set('address', address)
        await api(`/clusters?${params}`, { method: 'POST' })
      },
      removeCluster: async (name) => {
        await api(`/clusters?name=${encodeURIComponent(name)}`, { method: 'DELETE' })
      },
      activateCluster: async (name) => {
        await api(`/clusters/activate?name=${encodeURIComponent(name)}`, { method: 'POST' })
      },
    },
    metrics: {
      clusterOverview: () => api('/metrics/overview'),
      clusterSeries: (metric, minutes) =>
        liveOrMockSeries(metric, 'cluster', minutes, `/metrics/cluster?metric=${metric}&minutes=${minutes}`),
      nodeSeries: (node, metric, minutes) =>
        liveOrMockSeries(metric, node, minutes, `/metrics/node?node=${node}&metric=${metric}&minutes=${minutes}`),
      podSeries: (ns, pod, metric, minutes) =>
        liveOrMockSeries(
          metric,
          `pod:${ns}/${pod}`,
          minutes,
          `/metrics/pod?ns=${ns}&pod=${pod}&metric=${metric}&minutes=${minutes}`,
        ),
    },
  }
}
