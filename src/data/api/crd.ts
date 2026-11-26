import { apiGet, apiGetList } from '../apiClient'
import type { CrdProvider } from '../providers'
import { rejectUnavailable } from './unavailable'

export function createApiCrd(available: boolean): CrdProvider {
  if (!available) {
    return {
      listCrds: () => rejectUnavailable('crds'),
      listCustomResources: () => rejectUnavailable('crds'),
    }
  }
  return {
    listCrds: () => apiGet('/crds'),
    listCustomResources: (crd, options) => apiGetList('/crds/instances', { crd, limit: options?.limit }),
  }
}
