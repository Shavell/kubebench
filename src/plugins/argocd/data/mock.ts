import { ApiError } from '@/data/apiClient'
import { reasonError } from '@/data/reasonError'
import { delay, delayReject } from '@/data/mock/delay'
import { mulberry32 } from '@/data/mock/rng'
import type { ArgoCdProvider } from './provider'
import type {
  ArgoApplicationDetail,
  ArgoApplicationSet,
  ArgoApplicationSummary,
  ArgoHistoryEntry,
  ArgoProject,
  ArgoResource,
  ArgoSource,
} from './types'

/**
 * Deterministic demo Argo CD (design §0): three applications in mixed
 * states (guestbook synced and healthy, helm-guestbook mid-sync and out of
 * sync, kustomize-app degraded with automated sync), two projects and an
 * ApplicationSet. Times count from a fixed instant, revisions come from a
 * seeded RNG. Mutations change the in-memory state; a requested operation
 * runs until the application's detail has been read three more times, then
 * succeeds, so the Running → Succeeded transition (and the 409 meanwhile)
 * can be seen.
 */

const BASE_TIME = Date.parse('2026-11-14T10:00:00Z')
const REPO = 'https://github.com/argoproj/argocd-example-apps.git'
const IN_CLUSTER = 'https://kubernetes.default.svc'

const at = (minutes: number) => new Date(BASE_TIME + minutes * 60_000).toISOString()

function shas(seed: number, count: number): string[] {
  const rand = mulberry32(seed)
  return Array.from({ length: count }, () =>
    Array.from({ length: 40 }, () => Math.floor(rand() * 16).toString(16)).join(''),
  )
}

function resource(
  group: string,
  kind: string,
  namespace: string,
  name: string,
  status: string,
  health = '',
  catalogKey = '',
): ArgoResource {
  return {
    group,
    version: 'v1',
    kind,
    namespace,
    name,
    status,
    health,
    healthMessage: '',
    hook: false,
    requiresPruning: false,
    catalogKey,
  }
}

function history(source: ArgoSource, revisions: string[], startMinute: number): ArgoHistoryEntry[] {
  return revisions
    .map((revision, id) => ({
      id,
      revision,
      revisions: [],
      deployStartedAt: at(startMinute + id * 90),
      deployedAt: at(startMinute + id * 90 + 1),
      sources: [{ ...source }],
      initiatedBy: id === 0 ? 'admin' : 'kubebench:admin',
      automated: false,
    }))
    .reverse()
}

function baseApp(
  name: string,
  project: string,
  source: ArgoSource,
  destinationNamespace: string,
): ArgoApplicationDetail {
  return {
    name,
    namespace: 'argocd',
    project,
    repo: source.repoURL,
    path: source.path,
    chart: source.chart,
    targetRevision: source.targetRevision,
    sourceCount: 1,
    destination: { server: IN_CLUSTER, name: '', namespace: destinationNamespace },
    sync: 'Synced',
    health: 'Healthy',
    operationPhase: '',
    operating: false,
    lastSyncedAt: '',
    revision: '',
    autoSync: false,
    deleting: false,
    sources: [source],
    syncPolicy: { automated: false, prune: false, selfHeal: false, syncOptions: ['CreateNamespace=true'] },
    healthMessage: '',
    reconciledAt: at(600),
    resources: [],
    conditions: [],
    history: [],
    operation: null,
    images: [],
    urls: [],
  }
}

function seedApplications(): ArgoApplicationDetail[] {
  const guestbookSource: ArgoSource = { repoURL: REPO, path: 'guestbook', chart: '', targetRevision: 'HEAD', ref: '' }
  const guestbookRevs = shas(0xa1, 3)
  const guestbook = baseApp('guestbook', 'default', guestbookSource, 'guestbook')
  guestbook.history = history(guestbookSource, guestbookRevs, 10)
  guestbook.revision = guestbookRevs[2]
  guestbook.lastSyncedAt = guestbook.history[0].deployedAt
  guestbook.resources = [
    resource('', 'Service', 'guestbook', 'guestbook-ui', 'Synced', 'Healthy'),
    resource('apps', 'Deployment', 'guestbook', 'guestbook-ui', 'Synced', 'Healthy'),
  ]
  guestbook.images = ['gcr.io/google-samples/gb-frontend:v5']
  guestbook.operation = {
    phase: 'Succeeded',
    message: 'successfully synced (all tasks run)',
    startedAt: guestbook.history[0].deployStartedAt,
    finishedAt: guestbook.history[0].deployedAt,
    initiatedBy: 'kubebench:admin',
    automated: false,
    revision: guestbook.revision,
    dryRun: false,
    prune: false,
    retryCount: 0,
    resources: [
      { group: '', kind: 'Service', namespace: 'guestbook', name: 'guestbook-ui', status: 'Synced', message: 'service/guestbook-ui unchanged', hookPhase: 'Running', syncPhase: 'Sync' },
      { group: 'apps', kind: 'Deployment', namespace: 'guestbook', name: 'guestbook-ui', status: 'Synced', message: 'deployment.apps/guestbook-ui configured', hookPhase: 'Running', syncPhase: 'Sync' },
    ],
  }
  guestbook.operationPhase = 'Succeeded'

  const helmSource: ArgoSource = { repoURL: REPO, path: 'helm-guestbook', chart: '', targetRevision: 'HEAD', ref: '' }
  const helmRevs = shas(0xb2, 2)
  const helm = baseApp('helm-guestbook', 'team-web', helmSource, 'helm-guestbook')
  helm.history = history(helmSource, helmRevs.slice(0, 1), 200)
  helm.revision = helmRevs[1]
  helm.lastSyncedAt = helm.history[0].deployedAt
  helm.sync = 'OutOfSync'
  helm.health = 'Progressing'
  helm.healthMessage = 'Waiting for rollout to finish: 1 old replicas are pending termination'
  helm.resources = [
    resource('', 'Service', 'helm-guestbook', 'helm-guestbook', 'Synced', 'Healthy'),
    resource('apps', 'Deployment', 'helm-guestbook', 'helm-guestbook', 'OutOfSync', 'Progressing'),
    resource('', 'ConfigMap', 'helm-guestbook', 'helm-guestbook-config', 'OutOfSync', '', 'configmaps'),
  ]
  helm.images = ['gcr.io/heptio-images/ks-guestbook-demo:0.1']
  helm.operating = true
  helm.operationPhase = 'Running'
  helm.operation = {
    phase: 'Running',
    message: 'waiting for healthy state of apps/Deployment/helm-guestbook',
    startedAt: at(620),
    finishedAt: '',
    initiatedBy: 'kubebench:admin',
    automated: false,
    revision: helm.revision,
    dryRun: false,
    prune: false,
    retryCount: 0,
    resources: [],
  }

  const kustomizeSource: ArgoSource = { repoURL: REPO, path: 'kustomize-guestbook', chart: '', targetRevision: 'main', ref: '' }
  const kustomizeRevs = shas(0xc3, 2)
  const kustomize = baseApp('kustomize-app', 'team-web', kustomizeSource, 'shop')
  kustomize.history = history(kustomizeSource, kustomizeRevs, 300)
  kustomize.revision = kustomizeRevs[1]
  kustomize.lastSyncedAt = kustomize.history[0].deployedAt
  kustomize.health = 'Degraded'
  kustomize.healthMessage = 'Deployment "shop-payments" exceeded its progress deadline'
  kustomize.autoSync = true
  kustomize.syncPolicy = { automated: true, prune: true, selfHeal: true, syncOptions: ['CreateNamespace=true', 'ServerSideApply=true'] }
  kustomize.resources = [
    resource('', 'Service', 'shop', 'shop-frontend', 'Synced', 'Healthy'),
    resource('apps', 'Deployment', 'shop', 'shop-frontend', 'Synced', 'Healthy'),
    resource('apps', 'Deployment', 'shop', 'shop-payments', 'Synced', 'Degraded'),
    resource('', 'ConfigMap', 'shop', 'shop-config', 'Synced', '', 'configmaps'),
  ]
  kustomize.conditions = [
    {
      type: 'SyncError',
      status: '',
      reason: '',
      message: 'Failed sync attempt to ' + kustomizeRevs[1].slice(0, 7) + ': one or more objects failed to apply',
      lastTransitionTime: at(450),
    },
  ]
  kustomize.images = ['ghcr.io/acme/shop-frontend:2.4.1', 'ghcr.io/acme/shop-payments:3.1.0-rc.2']
  kustomize.urls = ['http://203.0.113.42/']
  kustomize.operationPhase = 'Failed'
  kustomize.operation = {
    phase: 'Failed',
    message: 'one or more objects failed to apply (dry run)',
    startedAt: at(450),
    finishedAt: at(451),
    initiatedBy: '',
    automated: true,
    revision: kustomize.revision,
    dryRun: false,
    prune: true,
    retryCount: 2,
    resources: [
      { group: 'apps', kind: 'Deployment', namespace: 'shop', name: 'shop-payments', status: 'SyncFailed', message: 'admission webhook denied the request', hookPhase: 'Failed', syncPhase: 'Sync' },
    ],
  }
  return [guestbook, helm, kustomize]
}

const PROJECTS: ArgoProject[] = [
  {
    name: 'default',
    namespace: 'argocd',
    description: '',
    sourceRepos: ['*'],
    destinations: [{ server: '*', name: '', namespace: '*' }],
    clusterResources: ['*/*'],
    roles: [],
    syncWindows: 0,
  },
  {
    name: 'team-web',
    namespace: 'argocd',
    description: 'Web storefront and demo apps',
    sourceRepos: [REPO],
    destinations: [
      { server: IN_CLUSTER, name: '', namespace: 'shop' },
      { server: IN_CLUSTER, name: '', namespace: 'helm-guestbook' },
    ],
    clusterResources: ['/Namespace'],
    roles: ['deployer', 'read-only'],
    syncWindows: 1,
  },
]

const APPLICATION_SETS: ArgoApplicationSet[] = [
  {
    name: 'guestbook-envs',
    namespace: 'argocd',
    generators: ['list'],
    templateName: '{{.env}}-guestbook',
    project: 'default',
    applications: [
      { name: 'dev-guestbook', status: 'Synced', health: 'Healthy', message: '' },
      { name: 'staging-guestbook', status: 'OutOfSync', health: 'Missing', message: '' },
    ],
    conditions: [
      { type: 'ResourcesUpToDate', status: 'True', reason: 'ApplicationSetUpToDate', message: 'All applications have been generated successfully', lastTransitionTime: at(500) },
    ],
  },
]

function summary(app: ArgoApplicationDetail): ArgoApplicationSummary {
  return {
    name: app.name,
    namespace: app.namespace,
    project: app.project,
    repo: app.repo,
    path: app.path,
    chart: app.chart,
    targetRevision: app.targetRevision,
    sourceCount: app.sourceCount,
    destination: app.destination,
    sync: app.sync,
    health: app.health,
    operationPhase: app.operationPhase,
    operating: app.operating,
    lastSyncedAt: app.lastSyncedAt,
    revision: app.revision,
    autoSync: app.autoSync,
    deleting: app.deleting,
  }
}

const clone = <T>(value: T): T => structuredClone(value)

export function mockArgoCd(): ArgoCdProvider {
  const apps = seedApplications()
  /** Reads left before an application's running operation finishes. */
  const pendingReads = new Map<string, number>([['argocd/helm-guestbook', 3]])
  let clock = 700 // minutes after BASE_TIME of the next mock event
  const revisionRand = mulberry32(0xd4)

  const key = (app: ArgoApplicationDetail) => `${app.namespace}/${app.name}`
  const find = (namespace: string, name: string) => apps.find((a) => a.namespace === namespace && a.name === name)
  const notFound = (namespace: string, name: string) =>
    new ApiError(404, 'kubernetes', `applications.argoproj.io "${name}" not found in namespace ${namespace}`)

  /** Advances a running operation by one detail read, finishing it when its time is up. */
  const tick = (app: ArgoApplicationDetail) => {
    const left = pendingReads.get(key(app))
    if (left === undefined) return
    if (left > 1) {
      pendingReads.set(key(app), left - 1)
      return
    }
    pendingReads.delete(key(app))
    finish(app)
  }

  const finish = (app: ArgoApplicationDetail) => {
    const op = app.operation
    if (!op) return
    const now = at(clock++)
    op.phase = 'Succeeded'
    op.finishedAt = now
    op.message = op.dryRun ? 'successfully synced (dry run)' : 'successfully synced (all tasks run)'
    op.resources = app.resources
      .filter((r) => !r.hook)
      .map((r) => ({
        group: r.group,
        kind: r.kind,
        namespace: r.namespace,
        name: r.name,
        status: 'Synced',
        message: `${r.kind.toLowerCase()}/${r.name} ${op.dryRun ? 'configured (dry run)' : 'configured'}`,
        hookPhase: 'Running',
        syncPhase: 'Sync',
      }))
    app.operating = false
    app.operationPhase = 'Succeeded'
    if (op.dryRun) return
    app.revision = op.revision
    app.sync = 'Synced'
    app.resources = app.resources.map((r) => ({ ...r, status: 'Synced' }))
    if (app.health !== 'Degraded') {
      app.health = 'Healthy'
      app.healthMessage = ''
      app.resources = app.resources.map((r) => (r.health ? { ...r, health: 'Healthy' } : r))
    }
    const nextId = Math.max(-1, ...app.history.map((h) => h.id)) + 1
    app.history = [
      {
        id: nextId,
        revision: op.revision,
        revisions: [],
        deployStartedAt: op.startedAt,
        deployedAt: now,
        sources: app.sources.map((s) => ({ ...s })),
        initiatedBy: op.initiatedBy,
        automated: false,
      },
      ...app.history,
    ]
    app.lastSyncedAt = now
  }

  const start = (app: ArgoApplicationDetail, revision: string, options: { dryRun: boolean; prune: boolean }) => {
    if (app.operating) {
      throw reasonError(409, 'conflict', 'argocd.operation_running', { namespace: app.namespace, name: app.name })
    }
    app.operating = true
    app.operationPhase = 'Running'
    app.operation = {
      phase: 'Running',
      message: 'one or more tasks are running',
      startedAt: at(clock++),
      finishedAt: '',
      initiatedBy: 'kubebench:admin',
      automated: false,
      revision,
      ...options,
      retryCount: 0,
      resources: [],
    }
    pendingReads.set(key(app), 3)
  }

  const mutate = (namespace: string, name: string, change: (app: ArgoApplicationDetail) => void) => {
    const app = find(namespace, name)
    if (!app) return delayReject<ArgoApplicationDetail>(notFound(namespace, name))
    try {
      change(app)
    } catch (err) {
      return delayReject<ArgoApplicationDetail>(err as Error)
    }
    return delay(clone(app))
  }

  return {
    listApplications: () => delay(apps.map((a) => clone(summary(a)))),
    getApplication: (namespace, name) => {
      const app = find(namespace, name)
      if (!app) return delayReject(notFound(namespace, name))
      tick(app)
      return delay(clone(app))
    },
    syncApplication: (namespace, name, request) =>
      mutate(namespace, name, (app) => {
        // A symbolic revision resolves to a commit, as the repo server would.
        const revision = request.revision
          ? /^[0-9a-f]{40}$/.test(request.revision)
            ? request.revision
            : shas(Math.floor(revisionRand() * 1e9), 1)[0]
          : app.revision
        start(app, revision, { dryRun: Boolean(request.dryRun), prune: Boolean(request.prune) })
      }),
    refreshApplication: (namespace, name) =>
      mutate(namespace, name, (app) => {
        app.reconciledAt = at(clock++)
      }),
    rollbackApplication: (namespace, name, historyId) =>
      mutate(namespace, name, (app) => {
        if (app.autoSync) {
          throw reasonError(409, 'conflict', 'argocd.rollback_auto_sync')
        }
        const entry = app.history.find((h) => h.id === historyId)
        if (!entry) throw reasonError(404, 'not_found', 'argocd.no_history_entry', { id: historyId })
        start(app, entry.revision, { dryRun: false, prune: false })
        app.sync = 'OutOfSync'
      }),
    deleteApplication: (namespace, name) => {
      const index = apps.findIndex((a) => a.namespace === namespace && a.name === name)
      if (index < 0) return delayReject(notFound(namespace, name))
      apps.splice(index, 1)
      return delay(undefined)
    },
    listProjects: () => delay(clone(PROJECTS)),
    listApplicationSets: () => delay(clone(APPLICATION_SETS)),
  }
}
