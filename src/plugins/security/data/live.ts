import type { PluginApi } from '@/data/pluginData'
import type { SecurityProvider } from './provider'
import type { SecurityReport } from './types'

/** SecurityProvider over /api/plugins/security. */
export function createLiveSecurity(api: PluginApi): SecurityProvider {
  return {
    scan: (options) => api.post<SecurityReport>('/scan', options),
    // null before the cluster's first scan.
    report: () => api.get<SecurityReport | null>('/report'),
    checks: () => api.get('/checks'),
    accepted: () => api.get('/accepted'),
    accept: (key, reason, expiresAt) => api.post('/accept', { key, reason, expiresAt }),
    unaccept: async (key) => {
      await api.del(`/accept/${encodeURIComponent(key)}`)
    },
    exportReport: (format) => api.get<string>('/export', { format }),
  }
}
