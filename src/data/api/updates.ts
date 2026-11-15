import { apiGet, apiPatch, apiPost, apiPut } from '../apiClient'
import type { UpdatesProvider } from '../providers'
import type { UpdateStatus } from '../types'

/** Release checks (GET /api/updates, admin mode and check) and the user's What's new state (/api/auth/me). */
export function createApiUpdates(): UpdatesProvider {
  return {
    status: () => apiGet<UpdateStatus>('/updates'),
    setMode: (mode) => apiPut<UpdateStatus>('/settings/updates', { mode }),
    checkNow: () => apiPost<UpdateStatus>('/updates/check'),
    lastSeenVersion: async () =>
      (await apiGet<{ lastSeenVersion?: string }>('/auth/me')).lastSeenVersion ?? '',
    markSeen: async (version) => {
      await apiPatch('/auth/me', { lastSeenVersion: version })
    },
  }
}
