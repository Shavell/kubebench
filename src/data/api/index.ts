import type { DataProviders } from '../providers'
import type { MetricKind, MetricPoint, MetricsSourceInfo } from '../types'
import { createMockProviders } from '../mock'
import { generateSeries } from '../mock/mockMetrics'
import { useSettingsStore } from '@/core/store/settingsStore'

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

/**
 * Fetches a series from the selected source (auto | metrics-server |
 * prometheus). When the source can't provide a metric (metrics-server
 * has no network/fs), falls back to the seeded mock series so charts
 * stay populated.
 */
async function liveOrMockSeries(
  metric: MetricKind,
  mockKey: string,
  minutes: number,
  livePath: string,
): Promise<MetricPoint[]> {
  const source = useSettingsStore.getState().metricsSource
  try {
    return await api<MetricPoint[]>(`${livePath}&source=${source}`)
  } catch {
    return generateSeries(mockKey, metric, minutes)
  }
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
      listSources: async (): Promise<MetricsSourceInfo[]> => {
        const s = await api<{
          metricsServer: { available: boolean }
          prometheus: { available: boolean; location: string | null }
        }>('/metrics/sources')
        return [
          { id: 'metrics-server', name: 'metrics-server', available: s.metricsServer.available },
          {
            id: 'prometheus',
            name: 'Prometheus',
            available: s.prometheus.available,
            detail: s.prometheus.location ?? 'not discovered',
          },
        ]
      },
      clusterOverview: () => api('/metrics/overview'),
      clusterAllocation: () => api('/metrics/allocation'),
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
