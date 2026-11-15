import type { PluginApi } from '@/data/pluginData'
import type { AccessProvider } from './provider'
import type { ReviewResult } from './types'

/** AccessProvider over /api/plugins/access. */
export function createLiveAccess(api: PluginApi): AccessProvider {
  return {
    listSubjects: () => api.get('/subjects'),
    getSubject: (subject) =>
      api.get(`/subjects/${encodeURIComponent(subject.kind)}/${encodeURIComponent(subject.name)}`, {
        namespace: subject.namespace,
      }),
    whoCan: (request) => api.get('/who-can', { ...request }),
    listRiskyRoles: () => api.get('/risky'),
    graph: (namespace, includeSystem) =>
      api.get('/graph', { namespace, system: includeSystem ? 'true' : undefined }),
    review: (request) => api.post<ReviewResult>('/review', request),
  }
}
