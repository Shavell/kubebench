import type { PluginApi } from '@/data/pluginData'
import type { CreateOptions, StorageProvider } from './provider'
import type { CreateResult, PvcSummary } from './types'

const dryRunParams = (options?: CreateOptions) => (options?.dryRun ? { dryRun: true } : undefined)

/** StorageProvider over /api/plugins/storage. */
export function createLiveStorage(api: PluginApi): StorageProvider {
  return {
    overview: () => api.get('/overview'),
    listPvcs: (namespace) => api.get('/pvcs', { ns: namespace }),
    listPvs: () => api.get('/pvs'),
    listCsiDrivers: () => api.get('/csidrivers'),
    listSnapshots: (namespace) => api.get('/snapshots', { ns: namespace }),
    createPvc: (request, options) => api.post<CreateResult>('/pvcs', request, dryRunParams(options)),
    createPv: (request, options) => api.post<CreateResult>('/pvs', request, dryRunParams(options)),
    createStorageClass: (request, options) =>
      api.post<CreateResult>('/storageclasses', request, dryRunParams(options)),
    resizePvc: (namespace, name, size) =>
      api.patch<PvcSummary>(
        `/pvcs/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}/resize`,
        { size },
      ),
  }
}
