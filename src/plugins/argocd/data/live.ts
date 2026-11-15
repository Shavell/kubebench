import type { PluginApi } from '@/data/pluginData'
import type { ArgoCdProvider } from './provider'
import type {
  ArgoApplicationDetail,
  ArgoApplicationSet,
  ArgoApplicationSummary,
  ArgoProject,
} from './types'

const appPath = (namespace: string, name: string) =>
  `/applications/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`

/** The Argo CD backend plugin (/api/plugins/argocd). */
export function liveArgoCd(api: PluginApi): ArgoCdProvider {
  return {
    listApplications: () => api.get<ArgoApplicationSummary[]>('/applications'),
    getApplication: (namespace, name) => api.get<ArgoApplicationDetail>(appPath(namespace, name)),
    syncApplication: (namespace, name, request) =>
      api.post<ArgoApplicationDetail>(`${appPath(namespace, name)}/sync`, request),
    refreshApplication: (namespace, name, hard) =>
      api.post<ArgoApplicationDetail>(`${appPath(namespace, name)}/refresh`, undefined, { hard }),
    rollbackApplication: (namespace, name, historyId) =>
      api.post<ArgoApplicationDetail>(`${appPath(namespace, name)}/rollback`, { historyId }),
    deleteApplication: async (namespace, name, cascade) => {
      await api.del(appPath(namespace, name), { cascade })
    },
    listProjects: () => api.get<ArgoProject[]>('/projects'),
    listApplicationSets: () => api.get<ArgoApplicationSet[]>('/applicationsets'),
  }
}
