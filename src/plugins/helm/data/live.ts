import type { PluginApi } from '@/data/pluginData'
import type { HelmData } from './provider'
import type { HelmChartRef, HelmRelease, HelmRepoUpdate } from './types'

const path = (namespace: string, name: string, suffix = '') =>
  `/releases/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}${suffix}`

/**
 * The default-values route and query of a chart reference: OCI charts
 * have their own route, which needs write (the caller names the host).
 */
function chartValuesQuery(chart: HelmChartRef): [string, Record<string, string | undefined>] {
  if ('uploadId' in chart) return ['/charts/values', { uploadId: chart.uploadId }]
  if ('ref' in chart) return ['/charts/oci/values', { ref: chart.ref, version: chart.version }]
  return ['/charts/values', { repo: chart.repo, chart: chart.name, version: chart.version }]
}

/** The Helm plugin's backend routes (`/api/plugins/helm`). */
export function createLiveHelm(api: PluginApi): HelmData {
  return {
    listReleases: (namespace) => api.get('/releases', { ns: namespace }),
    getRelease: (namespace, name) => api.get(path(namespace, name)),
    getValues: async (namespace, name, query = {}) =>
      (
        await api.get<{ values: string }>(path(namespace, name, '/values'), {
          revision: query.revision || undefined,
          all: query.all || undefined,
        })
      ).values,
    getManifest: async (namespace, name, revision) =>
      (await api.get<{ manifest: string }>(path(namespace, name, '/manifest'), { revision: revision || undefined }))
        .manifest,
    rollback: async (namespace, name, revision) =>
      (await api.post<{ release: HelmRelease }>(path(namespace, name, '/rollback'), { revision })).release,
    uninstall: async (namespace, name, keepHistory) => {
      await api.del(path(namespace, name), { keepHistory: keepHistory || undefined })
    },
    // dryRun goes in the query as well as the body; either asks for one.
    install: (request) => api.post('/install', request, { dryRun: request.dryRun || undefined }),
    upgrade: (namespace, name, request) =>
      api.post(path(namespace, name, '/upgrade'), request, { dryRun: request.dryRun || undefined }),

    listRepos: () => api.get('/repos'),
    addRepo: (input) => api.post('/repos', input),
    removeRepo: async (name) => {
      await api.del(`/repos/${encodeURIComponent(name)}`)
    },
    updateRepos: async () => (await api.post<{ repos: HelmRepoUpdate[] }>('/repos/update')).repos,

    searchCharts: (query) => api.get('/charts/search', { q: query }),
    chartValues: (chart) => api.get(...chartValuesQuery(chart)),
    uploadChart: (file) => {
      const form = new FormData()
      form.append('chart', file, file.name)
      return api.post('/charts/upload', form)
    },
  }
}
