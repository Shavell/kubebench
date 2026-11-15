import { dump, load } from 'js-yaml'
import { ApiError } from '@/data/apiClient'
import { delay, delayReject } from '@/data/mock/delay'
import {
  defaultValues,
  initialReleases,
  mockCharts,
  mockRepos,
  type MockChart,
  type MockRelease,
  type MockRepo,
  type MockRevision,
} from './mockData'
import type { HelmData } from './provider'
import type {
  HelmChartInfo,
  HelmChartRef,
  HelmChartResult,
  HelmRelease,
  HelmReleaseDetail,
  HelmRepo,
} from './types'

type Values = Record<string, unknown>

/** A chart a mock install or upgrade resolved, with its default values. */
interface ResolvedChart {
  info: HelmChartInfo
  image: string
}

const notFound = (message: string) => new ApiError(404, 'not_found', message)
const badRequest = (message: string) => new ApiError(400, 'bad_request', message)

/** Fixed demo clock: every change happens one minute after the previous one. */
function demoClock(): () => string {
  let minutes = 0
  return () => new Date(Date.UTC(2026, 7, 11, 9, 15 + minutes++)).toISOString().replace('.000Z', 'Z')
}

function parseValues(text: string): Values {
  let parsed: unknown
  try {
    parsed = load(text)
  } catch (err) {
    throw badRequest(`values are not valid YAML: ${err instanceof Error ? err.message : String(err)}`)
  }
  if (parsed === undefined || parsed === null) return {}
  if (typeof parsed !== 'object' || Array.isArray(parsed)) throw badRequest('values must be a YAML mapping')
  return parsed as Values
}

/** Helm's value merge: maps merge key by key, everything else is replaced. */
function mergeValues(base: Values, over: Values): Values {
  const out: Values = { ...base }
  for (const [key, value] of Object.entries(over)) {
    const prev = out[key]
    out[key] =
      value && prev && typeof value === 'object' && typeof prev === 'object' && !Array.isArray(value) && !Array.isArray(prev)
        ? mergeValues(prev as Values, value as Values)
        : value
  }
  return out
}

const yamlText = (values: Values) => (Object.keys(values).length === 0 ? '' : dump(values))

function chartInfo(chart: MockChart, version: string, appVersion: string): HelmChartInfo {
  return {
    name: chart.name,
    version,
    appVersion,
    description: chart.description,
    home: chart.home,
    icon: '',
    sources: [chart.home],
    keywords: chart.keywords,
    kubeVersion: '',
    deprecated: false,
  }
}

/** Metadata of a chart that is in no demo repository (an upload). */
function otherChartInfo(name: string, version: string, appVersion: string, description = ''): HelmChartInfo {
  return {
    name,
    version,
    appVersion,
    description,
    home: '',
    icon: '',
    sources: [],
    keywords: [],
    kubeVersion: '',
    deprecated: false,
  }
}

/** Renders the manifest a release revision of the demo charts produces. */
function renderManifest(name: string, namespace: string, image: string, all: Values): string {
  const replicas = typeof all.replicaCount === 'number' ? all.replicaCount : 1
  const img = (all.image ?? {}) as Values
  const service = (all.service ?? {}) as Values
  const labels = { 'app.kubernetes.io/name': name, 'app.kubernetes.io/managed-by': 'Helm' }
  const doc = (source: string, object: Values) => `---\n# Source: ${source}\n${dump(object, { noRefs: true })}`
  return [
    doc(`${name}/templates/service.yaml`, {
      apiVersion: 'v1',
      kind: 'Service',
      metadata: { name, namespace, labels },
      spec: { type: service.type ?? 'ClusterIP', ports: [{ port: service.port ?? 80, targetPort: 'http' }], selector: labels },
    }),
    doc(`${name}/templates/deployment.yaml`, {
      apiVersion: 'apps/v1',
      kind: 'Deployment',
      metadata: { name, namespace, labels },
      spec: {
        replicas,
        selector: { matchLabels: labels },
        template: {
          metadata: { labels },
          spec: {
            containers: [
              {
                name,
                image: `${String(img.repository ?? image)}:${String(img.tag ?? 'latest')}`,
                ports: [{ name: 'http', containerPort: 8080 }],
              },
            ],
          },
        },
      },
    }),
  ].join('')
}

const notesFor = (name: string, namespace: string) =>
  `Release ${name} is deployed in namespace ${namespace}.\n\nRun kubectl -n ${namespace} get pods -l app.kubernetes.io/name=${name} to see its pods.`

/** The Helm capability in demo mode: an in-memory, deterministic model. */
export function createMockHelm(): HelmData {
  const releases: MockRelease[] = initialReleases()
  const repos: MockRepo[] = mockRepos.map((r) => ({ ...r }))
  /** Uploaded archives: id → chart. */
  const uploads = new Map<string, ResolvedChart>()
  const now = demoClock()

  const findChart = (name: string, repo?: string) =>
    mockCharts.find((c) => c.name === name && (repo === undefined || c.repo === repo))

  const findRelease = (namespace: string, name: string) => {
    const release = releases.find((r) => r.namespace === namespace && r.name === name)
    if (!release) throw notFound(`release: not found (${namespace}/${name})`)
    return release
  }

  const revisionOf = (release: MockRelease, revision = 0) => {
    const found = revision ? release.revisions.find((r) => r.revision === revision) : release.revisions[0]
    if (!found) throw notFound(`release: not found (revision ${revision})`)
    return found
  }

  /** Defaults of a revision's chart (an uploaded chart's are the generic ones). */
  const defaultsOf = (r: Pick<MockRevision, 'chart' | 'appVersion'>) =>
    parseValues(defaultValues({ name: r.chart, image: findChart(r.chart)?.image ?? `example.com/${r.chart}` }, r.appVersion))

  const allValues = (r: MockRevision) => mergeValues(defaultsOf(r), parseValues(r.values))

  const manifestOf = (release: MockRelease, r: MockRevision) =>
    renderManifest(release.name, release.namespace, findChart(r.chart)?.image ?? `example.com/${r.chart}`, allValues(r))

  const summary = (release: MockRelease): HelmRelease => {
    const r = release.revisions[0]
    return {
      name: release.name,
      namespace: release.namespace,
      chart: r.chart,
      chartVersion: r.chartVersion,
      appVersion: r.appVersion,
      revision: r.revision,
      status: r.status,
      updated: r.updated,
      description: r.description,
    }
  }

  const resolve = (ref: HelmChartRef): ResolvedChart => {
    if ('uploadId' in ref) {
      const upload = uploads.get(ref.uploadId)
      if (!upload) throw notFound(`upload ${ref.uploadId} does not exist or has expired; upload the chart again`)
      return upload
    }
    const name = 'ref' in ref ? (ref.ref.split('/').pop() ?? '') : ref.name
    if ('repo' in ref && !repos.some((r) => r.name === ref.repo)) throw notFound(`repository "${ref.repo}" is not added`)
    const chart = 'repo' in ref ? findChart(name, ref.repo) : findChart(name)
    if (!chart) throw badRequest(`could not download chart ${name}: not found`)
    const version = ref.version ? chart.versions.find((x) => x.version === ref.version) : chart.versions[0]
    if (!version) throw badRequest(`could not download chart ${name}: version "${ref.version}" not found`)
    return { info: chartInfo(chart, version.version, version.appVersion), image: chart.image }
  }

  /** Marks every revision but the newest as superseded. */
  const supersede = (release: MockRelease) => {
    for (const r of release.revisions.slice(1)) {
      if (r.status === 'deployed' || r.status === 'failed') r.status = 'superseded'
    }
  }

  return {
    listReleases: (namespace) =>
      delay(releases.filter((r) => !namespace || r.namespace === namespace).map(summary)),

    getRelease: async (namespace, name) => {
      const release = findRelease(namespace, name)
      const latest = release.revisions[0]
      const chart = findChart(latest.chart)
      const detail: HelmReleaseDetail = {
        ...summary(release),
        firstDeployed: release.revisions[release.revisions.length - 1].updated,
        notes: notesFor(name, namespace),
        chartInfo: chart
          ? chartInfo(chart, latest.chartVersion, latest.appVersion)
          : otherChartInfo(latest.chart, latest.chartVersion, latest.appVersion),
        hooks:
          latest.chart === 'shop-app'
            ? [
                {
                  name: 'shop-app-db-migrate',
                  kind: 'Job',
                  events: ['post-upgrade'],
                  weight: 0,
                  phase: latest.status === 'failed' ? 'Failed' : 'Succeeded',
                  startedAt: latest.updated,
                  completedAt: latest.updated,
                },
              ]
            : [],
        history: release.revisions.map((r) => ({
          revision: r.revision,
          status: r.status,
          updated: r.updated,
          description: r.description,
          chart: r.chart,
          chartVersion: r.chartVersion,
          appVersion: r.appVersion,
        })),
      }
      return delay(detail)
    },

    getValues: async (namespace, name, query = {}) => {
      const r = revisionOf(findRelease(namespace, name), query.revision)
      return delay(query.all ? yamlText(allValues(r)) : r.values)
    },

    getManifest: async (namespace, name, revision) => {
      const release = findRelease(namespace, name)
      return delay(manifestOf(release, revisionOf(release, revision)))
    },

    rollback: async (namespace, name, revision) => {
      const release = findRelease(namespace, name)
      const target = revision ? revisionOf(release, revision) : release.revisions[1]
      if (!target) throw badRequest('no revision to roll back to')
      release.revisions.unshift({
        ...target,
        revision: release.revisions[0].revision + 1,
        status: 'deployed',
        updated: now(),
        description: `Rollback to ${target.revision}`,
      })
      supersede(release)
      return delay(summary(release))
    },

    uninstall: async (namespace, name, keepHistory) => {
      const release = findRelease(namespace, name)
      if (keepHistory) {
        release.revisions[0] = { ...release.revisions[0], status: 'uninstalled', description: 'Uninstallation complete', updated: now() }
      } else {
        releases.splice(releases.indexOf(release), 1)
      }
      return delay(undefined)
    },

    install: async (request) => {
      if (releases.some((r) => r.namespace === request.namespace && r.name === request.name)) {
        return delayReject(new ApiError(409, 'conflict', 'cannot re-use a name that is still in use'))
      }
      const chart = resolve(request.chart)
      const values = parseValues(request.values)
      const revision: MockRevision = {
        revision: 1,
        status: 'deployed',
        updated: request.dryRun ? '' : now(),
        description: request.dryRun ? 'Dry run complete' : 'Install complete',
        chart: chart.info.name,
        chartVersion: chart.info.version,
        appVersion: chart.info.appVersion,
        values: yamlText(values),
      }
      const release: MockRelease = { name: request.name, namespace: request.namespace, revisions: [revision] }
      if (!request.dryRun) releases.push(release)
      return delay({
        release: summary(release),
        manifest: renderManifest(request.name, request.namespace, chart.image, mergeValues(defaultsOf(revision), values)),
        notes: notesFor(request.name, request.namespace),
      })
    },

    upgrade: async (namespace, name, request) => {
      const release = findRelease(namespace, name)
      const current = release.revisions[0]
      if (current.status === 'uninstalled') throw badRequest(`"${name}" has no deployed releases`)
      const chart = request.chart ? resolve(request.chart) : null
      const given = parseValues(request.values)
      const values = request.reuseValues ? mergeValues(parseValues(current.values), given) : given
      const next: MockRevision = {
        revision: current.revision + 1,
        status: 'deployed',
        updated: request.dryRun ? '' : now(),
        description: request.dryRun ? 'Dry run complete' : 'Upgrade complete',
        chart: chart?.info.name ?? current.chart,
        chartVersion: chart?.info.version ?? current.chartVersion,
        appVersion: chart?.info.appVersion ?? current.appVersion,
        values: yamlText(values),
      }
      const upgraded: MockRelease = { ...release, revisions: [next, ...release.revisions] }
      const result = {
        release: summary(upgraded),
        manifest: manifestOf(upgraded, next),
        notes: notesFor(name, namespace),
        currentManifest: manifestOf(release, current),
      }
      if (!request.dryRun) {
        release.revisions.unshift(next)
        supersede(release)
      }
      return delay(result)
    },

    listRepos: () =>
      delay(
        repos.map(
          (r): HelmRepo => ({
            name: r.name,
            url: r.url,
            authenticated: r.authenticated,
            chartCount: mockCharts.filter((c) => c.repo === r.name).length,
            updated: r.updated,
          }),
        ),
      ),

    addRepo: async (input) => {
      if (repos.some((r) => r.name === input.name)) {
        return delayReject(new ApiError(409, 'conflict', `repository "${input.name}" already exists`))
      }
      if (!/^https?:\/\/[^/@]+/.test(input.url)) {
        return delayReject(badRequest('url must be an http or https URL without credentials'))
      }
      const repo: MockRepo = { name: input.name, url: input.url, authenticated: Boolean(input.username), updated: now() }
      repos.push(repo)
      return delay({ name: repo.name, url: repo.url, authenticated: repo.authenticated, chartCount: 0, updated: repo.updated })
    },

    removeRepo: async (name) => {
      const index = repos.findIndex((r) => r.name === name)
      if (index < 0) return delayReject(notFound(`repository "${name}" does not exist`))
      repos.splice(index, 1)
      return delay(undefined)
    },

    updateRepos: () => {
      const updated = now()
      for (const r of repos) r.updated = updated
      return delay(repos.map((r) => ({ name: r.name, ok: true })))
    },

    searchCharts: (query) => {
      const q = query.trim().toLowerCase()
      const results: HelmChartResult[] = mockCharts
        .filter((c) => repos.some((r) => r.name === c.repo))
        .filter(
          (c) =>
            !q ||
            c.name.toLowerCase().includes(q) ||
            c.description.toLowerCase().includes(q) ||
            c.keywords.some((k) => k.includes(q)),
        )
        .map((c) => ({
          repo: c.repo,
          name: c.name,
          version: c.versions[0].version,
          appVersion: c.versions[0].appVersion,
          description: c.description,
          icon: '',
          deprecated: false,
          versions: c.versions.map((x) => ({ ...x, deprecated: false })),
        }))
      return delay(results)
    },

    chartValues: async (ref) => {
      const chart = resolve(ref)
      return delay({ chart: chart.info, values: defaultValues({ name: chart.info.name, image: chart.image }, chart.info.appVersion) })
    },

    uploadChart: async (file) => {
      // The demo cannot unpack archives: the chart is named by the file, e.g. app-1.2.3.tgz.
      const match = /^(.+?)-(\d+\.\d+\.\d+[^/]*?)\.tgz$/.exec(file.name)
      if (!match) return delayReject(badRequest('not a chart archive: name it <chart>-<version>.tgz'))
      const id = Array.from(file.name, (ch) => ch.charCodeAt(0).toString(16).padStart(2, '0'))
        .join('')
        .padEnd(32, '0')
        .slice(0, 32)
      const info = otherChartInfo(match[1], match[2], match[2], `Uploaded chart ${match[1]}`)
      uploads.set(id, { info, image: `example.com/${match[1]}` })
      return delay({ uploadId: id, chart: info, expiresAt: '2026-08-12T09:15:00Z' })
    },
  }
}
