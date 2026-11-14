import { apiGet, apiPost } from '../apiClient'
import type { HelmProvider } from '../providers'
import { rejectUnavailable } from './unavailable'

export function createApiHelm(available: boolean): HelmProvider {
  if (!available) {
    return {
      listReleases: () => rejectUnavailable('helm'),
      listRepositories: () => rejectUnavailable('helm'),
      getReleaseValuesYaml: () => rejectUnavailable('helm'),
      rollbackRelease: () => rejectUnavailable('helm'),
      uninstallRelease: () => rejectUnavailable('helm'),
    }
  }
  return {
    listReleases: () => apiGet('/helm/releases'),
    listRepositories: () => apiGet('/helm/repos'),
    getReleaseValuesYaml: async (name, ns) =>
      (await apiGet<{ yaml: string }>('/helm/values', { name, ns })).yaml,
    rollbackRelease: async (name, ns, revision) => {
      await apiPost('/helm/rollback', undefined, { name, ns, revision })
    },
    uninstallRelease: async (name, ns) => {
      await apiPost('/helm/uninstall', undefined, { name, ns })
    },
  }
}
