import type { AuditProvider } from '../providers'
import type { AuditEntry } from '../types'
import { delay } from './delay'
import { mulberry32 } from './rng'

/**
 * Deterministic audit history for mock mode: a seeded mix of workload
 * operations, shell sessions and admin changes, including rejected
 * attempts (viewer role, read-only production cluster).
 */
const BASE_TIME = Date.parse('2026-08-11T09:15:00Z')
const ENTRY_COUNT = 240
const DEFAULT_LIMIT = 100

interface Template {
  action: string
  target: (pick: <T>(items: T[]) => T) => string
  detail?: string
  admin?: boolean
}

const NAMESPACES = ['default', 'shop', 'monitoring', 'ingress-nginx']
const DEPLOYMENTS = ['web', 'api', 'checkout', 'cart', 'grafana']
const PODS = ['web-7d4b9c6f5-x2lq8', 'api-5c9f8b7d6-k9m2p', 'checkout-6b8d7c5f4-q7w3e', 'grafana-7f9c5b6d4-p2k8s']

const TEMPLATES: Template[] = [
  { action: 'POST /api/deployments/scale', target: (p) => `deployment/${p(NAMESPACES)}/${p(DEPLOYMENTS)}`, detail: '{"replicas":3}' },
  { action: 'POST /api/deployments/restart', target: (p) => `deployment/${p(NAMESPACES)}/${p(DEPLOYMENTS)}` },
  { action: 'DELETE /api/pods', target: (p) => `pod/${p(NAMESPACES)}/${p(PODS)}` },
  { action: 'exec.session', target: (p) => `pod/${p(NAMESPACES)}/${p(PODS)}`, detail: '{"container":"app","command":"bash"}' },
  { action: 'POST /api/plugins/helm/releases/{ns}/{name}/rollback', target: (p) => `release/${p(NAMESPACES)}/${p(DEPLOYMENTS)}`, detail: '{"revision":2}' },
  { action: 'POST /api/nodes/cordon', target: (p) => `node/${p(['kb-worker-1', 'kb-worker-2'])}`, detail: '{"unschedulable":true}' },
  { action: 'POST /api/users', target: (p) => `user/${p(['carol', 'dave', 'erin'])}`, detail: '{"role":"user"}', admin: true },
  { action: 'PUT /api/users/{id}/clusters', target: (p) => `user/${p(['alice', 'bob'])}`, admin: true },
  { action: 'PATCH /api/clusters/{id}', target: () => 'cluster/prod-eu-west-1', detail: '{"readOnly":true}', admin: true },
  { action: 'PUT /api/plugins/{id}', target: (p) => `plugin/${p(['argocd', 'storage', 'security'])}`, detail: '{"enabled":true}', admin: true },
]

const CLUSTERS = ['kind-dev-cluster', 'staging', 'prod-eu-west-1']
const OPERATORS = ['admin', 'alice', 'ops-lead']

function generate(): AuditEntry[] {
  const rand = mulberry32(0xa0d17)
  const pick = <T>(items: T[]): T => items[Math.floor(rand() * items.length)]
  const entries: AuditEntry[] = []
  let minutesAgo = 0
  for (let i = 0; i < ENTRY_COUNT; i++) {
    minutesAgo += 3 + Math.floor(rand() * 55)
    const template = pick(TEMPLATES)
    const cluster = template.admin ? '' : pick(CLUSTERS)
    // A viewer occasionally tries a mutation and is rejected by role.
    const viewerAttempt = !template.admin && rand() < 0.08
    const user = viewerAttempt ? 'bob' : template.admin ? pick(['admin', 'ops-lead']) : pick(OPERATORS)
    let status = 200
    let detail = template.detail ?? ''
    if (viewerAttempt) {
      status = 403
      detail = 'forbidden_role: viewer cannot modify resources'
    } else if (cluster === 'prod-eu-west-1') {
      status = 403
      detail = 'cluster_read_only: cluster is read-only'
    } else if (rand() < 0.06) {
      status = 409
      detail = 'conflict: the object has been modified'
    }
    entries.push({
      id: ENTRY_COUNT - i,
      ts: new Date(BASE_TIME - minutesAgo * 60_000).toISOString(),
      user,
      cluster,
      action: template.action,
      target: template.target(pick),
      status,
      detail,
    })
  }
  return entries
}

let history: AuditEntry[] | null = null

export function createMockAudit(): AuditProvider {
  return {
    listAudit: (query) => {
      history ??= generate()
      const action = query.action?.toLowerCase()
      const matches = history.filter(
        (e) =>
          (query.before === undefined || e.id < query.before) &&
          (!query.user || e.user === query.user) &&
          (!query.cluster || e.cluster === query.cluster) &&
          (!action || e.action.toLowerCase().includes(action)),
      )
      const limit = query.limit ?? DEFAULT_LIMIT
      const items = matches.slice(0, limit)
      return delay({
        items,
        nextBefore: matches.length > limit ? items[items.length - 1].id : null,
      })
    },
  }
}
