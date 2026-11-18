import { useSettingsStore } from '@/core/store/settingsStore'
import { apiGet } from '../apiClient'
import type { MetricsProvider } from '../providers'
import type { MetricsSourceInfo, UnavailableReason } from '../types'

interface SourceStatus {
  available: boolean
  location: string | null
  unavailable?: UnavailableReason
}

interface SourcesResponse {
  metricsServer: SourceStatus
  prometheus: SourceStatus
}

/**
 * Series come from the source selected in Settings (auto | metrics-server
 * | prometheus). When that source cannot provide a metric (metrics-server
 * has no network/fs history) the request fails with the server's reason;
 * charts show it instead of invented data.
 */
export function createApiMetrics(): MetricsProvider {
  const source = () => useSettingsStore.getState().metricsSource
  return {
    listSources: async (): Promise<MetricsSourceInfo[]> => {
      const s = await apiGet<SourcesResponse>('/metrics/sources')
      return [
        {
          id: 'metrics-server',
          name: 'metrics-server',
          available: s.metricsServer.available,
          unavailable: s.metricsServer.unavailable,
        },
        {
          id: 'prometheus',
          name: 'Prometheus',
          available: s.prometheus.available,
          detail: s.prometheus.location ?? undefined,
          unavailable: s.prometheus.unavailable,
        },
      ]
    },
    clusterOverview: () => apiGet('/metrics/overview'),
    clusterAllocation: () => apiGet('/metrics/allocation'),
    clusterSeries: (metric, minutes) =>
      apiGet('/metrics/cluster', { metric, minutes, source: source() }),
    nodeSeries: (node, metric, minutes) =>
      apiGet('/metrics/node', { node, metric, minutes, source: source() }),
    podSeries: (ns, pod, metric, minutes) =>
      apiGet('/metrics/pod', { ns, pod, metric, minutes, source: source() }),
  }
}
