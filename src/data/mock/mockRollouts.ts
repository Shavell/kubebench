import { ApiError } from '../apiClient'
import type { Deployment, Pod, RolloutHistory, RolloutRevision } from '../types'
import { deployments, pods } from './mockCluster'

/**
 * Rollout history of the mock Deployments (GET /api/deployments/history),
 * kept in memory so rollbacks show up. Deterministic: fixed timestamps.
 */

interface RevisionSeed {
  image: string
  changeCause: string
  created: string
}

/** Revisions of the demo Deployments, oldest first; the last one is current. */
const SEEDS: Record<string, RevisionSeed[]> = {
  'shop/shop-frontend': [
    { image: 'ghcr.io/acme/shop-frontend:2.3.0', changeCause: '', created: '2026-06-27T08:12:00Z' },
    {
      image: 'ghcr.io/acme/shop-frontend:2.4.0',
      changeCause: 'kubectl set image deployment/shop-frontend shop-frontend=ghcr.io/acme/shop-frontend:2.4.0',
      created: '2026-07-30T14:41:00Z',
    },
    { image: 'ghcr.io/acme/shop-frontend:2.4.1', changeCause: 'Hotfix: checkout rounding', created: '2026-08-08T16:05:00Z' },
  ],
  'shop/shop-payments': [
    { image: 'ghcr.io/acme/shop-payments:3.0.4', changeCause: '', created: '2026-06-27T08:15:00Z' },
    { image: 'ghcr.io/acme/shop-payments:3.1.0-rc.2', changeCause: 'Canary of 3.1.0-rc.2', created: '2026-08-11T08:34:00Z' },
  ],
}

/** Creation time of the single revision of every other Deployment. */
const DEFAULT_CREATED = '2026-05-11T06:00:00Z'

const histories = new Map<string, RolloutRevision[]>()

const keyOf = (d: Deployment) => `${d.namespace}/${d.name}`

/** The Deployment's Pods; the mock links them through controlledBy. */
function podsOf(d: Deployment): Pod[] {
  return pods.filter((p) => p.namespace === d.namespace && p.controlledBy === `Deployment/${d.name}`)
}

/** A stable stand-in for a pod-template-hash. */
function templateHash(text: string): string {
  let h = 0x811c9dc5
  for (let i = 0; i < text.length; i++) {
    h = Math.imul(h ^ text.charCodeAt(i), 0x01000193) >>> 0
  }
  return h.toString(36).padStart(7, '0')
}

function seed(d: Deployment): RolloutRevision[] {
  const owned = podsOf(d)
  // The current ReplicaSet is the one the Pods' names point at.
  const currentReplicaSet = owned[0]?.name.replace(/-[^-]+$/, '') ?? `${d.name}-${templateHash(d.name)}`
  const seeds = SEEDS[keyOf(d)] ?? [
    {
      image: owned[0]?.containers[0]?.image ?? `registry.example.com/${d.name}:latest`,
      changeCause: '',
      created: DEFAULT_CREATED,
    },
  ]
  return seeds.map((s, i) => {
    const current = i === seeds.length - 1
    return {
      revision: i + 1,
      replicaSet: current ? currentReplicaSet : `${d.name}-${templateHash(s.image)}`,
      images: [s.image],
      changeCause: s.changeCause,
      created: s.created,
      replicas: 0,
      readyReplicas: 0,
      current,
    }
  })
}

/** The revisions of d, with the current one's Pod counts following the Deployment. */
function revisionsOf(d: Deployment): RolloutRevision[] {
  let revisions = histories.get(keyOf(d))
  if (!revisions) {
    revisions = seed(d)
    histories.set(keyOf(d), revisions)
  }
  for (const r of revisions) {
    r.replicas = r.current ? d.replicas : 0
    r.readyReplicas = r.current ? d.readyReplicas : 0
  }
  return revisions
}

function findDeployment(namespace: string, name: string): Deployment {
  const d = deployments.find((x) => x.namespace === namespace && x.name === name)
  if (!d) throw new ApiError(404, 'kubernetes', `deployments.apps "${name}" not found`)
  return d
}

export function mockRolloutHistory(namespace: string, name: string): RolloutHistory {
  const d = findDeployment(namespace, name)
  const revisions = revisionsOf(d)
    .map((r) => ({ ...r, images: [...r.images] }))
    .sort((a, b) => b.revision - a.revision)
  return {
    currentRevision: revisions.find((r) => r.current)?.revision ?? 0,
    paused: d.paused ?? false,
    revisions,
  }
}

/**
 * Rolls back like the Deployment controller: the ReplicaSet of `revision`
 * becomes current under a new revision number and the Pods take its image.
 */
export function mockRollback(namespace: string, name: string, revision: number): void {
  const d = findDeployment(namespace, name)
  if (d.paused) {
    throw new ApiError(409, 'conflict', `Deployment ${name} is paused; resume it before rolling back`)
  }
  const revisions = revisionsOf(d)
  if (revisions.find((r) => r.current)?.revision === revision) {
    throw new ApiError(409, 'conflict', `revision ${revision} is the current revision of Deployment ${name}`)
  }
  const target = revisions.find((r) => r.revision === revision)
  if (!target) throw new ApiError(404, 'not_found', `Deployment ${name} has no revision ${revision}`)

  const next = Math.max(...revisions.map((r) => r.revision)) + 1
  for (const r of revisions) r.current = false
  target.revision = next
  target.current = true
  const owned = podsOf(d)
  for (const p of owned) {
    p.containers = p.containers.map((c, i) => (i === 0 ? { ...c, image: target.images[0] } : c))
    p.restarts = 0
    p.age = '5s'
    // Only scheduled Pods start; the others stay Pending.
    if (p.node) {
      p.status = 'Running'
      p.readyContainers = p.totalContainers
    }
  }
  d.readyReplicas = d.available = Math.min(d.replicas, owned.filter((p) => p.status === 'Running').length)
  d.upToDate = d.replicas
}

export function mockPauseDeployment(namespace: string, name: string, paused: boolean): void {
  findDeployment(namespace, name).paused = paused
}
