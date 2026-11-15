import { ApiError } from '../apiClient'
import type { WorkloadOpsProvider } from '../providers'
import { MAX_BULK_DELETE, type KubeObject, type Pod, type PodDeleteResult } from '../types'
import { delay, delayReject } from './delay'
import { deployments, namespaces, nodes, pods, services, syncDeploymentPods } from './mockCluster'
import { drainMockNode } from './mockDrain'
import { genericMock } from './mockGeneric'
import { mockPauseDeployment, mockRollback, mockRolloutHistory } from './mockRollouts'
import { mulberry32 } from './rng'

/**
 * Workload and node operations on the in-memory mock dataset. Errors use
 * the codes the backend answers with; generated names come from a seeded
 * RNG and timestamps are fixed, so runs are reproducible.
 */

/** The namespace the demo's KubeBench runs in (the backend's KUBEBENCH_NAMESPACE). */
const MOCK_OWN_NAMESPACE = 'kubebench'
const SYSTEM_NAMESPACES = ['default', 'kube-system', 'kube-public', 'kube-node-lease']
/** "Now" of the mock dataset, for restartedAt stamps. */
const MOCK_NOW = '2026-08-11T09:15:00Z'
const DNS1123_LABEL = /^[a-z0-9]([-a-z0-9]*[a-z0-9])?$/

const GENERIC_KEYS: Record<string, string> = {
  StatefulSet: 'statefulsets',
  DaemonSet: 'daemonsets',
  ReplicaSet: 'replicasets',
  CronJob: 'cronjobs',
}

const random = mulberry32(0x0b5e)
const ALPHABET = 'bcdfghjklmnpqrstvwxz2456789'

/** Five characters like the random suffixes of Kubernetes names. */
function suffix(): string {
  let out = ''
  for (let i = 0; i < 5; i++) out += ALPHABET[Math.floor(random() * ALPHABET.length)]
  return out
}

/** Runs a mock operation after the simulated latency; a thrown ApiError rejects. */
function run<T>(operation: () => T): Promise<T> {
  try {
    return delay(operation())
  } catch (err) {
    return delayReject(err instanceof Error ? err : new Error(String(err)))
  }
}

const notFound = (resource: string, name: string) => new ApiError(404, 'kubernetes', `${resource} "${name}" not found`)
const badRequest = (message: string) => new ApiError(400, 'bad_request', message)

function findNode(name: string) {
  const node = nodes.find((n) => n.name === name)
  if (!node) throw notFound('nodes', name)
  return node
}

function findPod(namespace: string, name: string): Pod {
  const pod = pods.find((p) => p.namespace === namespace && p.name === name)
  if (!pod) throw notFound('pods', name)
  return pod
}

function findObject(kind: string, namespace: string, name: string): KubeObject {
  const key = GENERIC_KEYS[kind]
  const obj = (genericMock[key] ?? []).find((o) => o.metadata?.namespace === namespace && o.metadata?.name === name)
  if (!obj) throw notFound(key ?? kind, name)
  return obj
}

/** obj[key] as an object, created when missing. */
function child(obj: Record<string, unknown>, key: string): Record<string, unknown> {
  const value = obj[key]
  if (value && typeof value === 'object') return value as Record<string, unknown>
  const created: Record<string, unknown> = {}
  obj[key] = created
  return created
}

function removeWhere<T>(list: T[], drop: (item: T) => boolean): void {
  for (let i = list.length - 1; i >= 0; i--) if (drop(list[i])) list.splice(i, 1)
}

/** Restarted Pods look fresh; only scheduled ones run. */
function refresh(owned: Pod[]): void {
  for (const p of owned) {
    p.age = '5s'
    p.restarts = 0
    if (p.node) {
      p.status = 'Running'
      p.readyContainers = p.totalContainers
    }
  }
}

/** StatefulSet Pods are <name>-<ordinal>; scaling adds or removes the highest ordinals. */
function syncStatefulSetPods(namespace: string, name: string, replicas: number): void {
  const owned = pods.filter((p) => p.namespace === namespace && p.controlledBy === `StatefulSet/${name}`)
  const template = owned[0]
  if (!template) return
  for (let i = owned.length; i < replicas; i++) {
    pods.push({ ...template, name: `${name}-${i}`, age: '0s', restarts: 0 })
  }
  removeWhere(pods, (p) => owned.includes(p) && Number(p.name.slice(name.length + 1)) >= replicas)
}

export function createMockWorkloadOps(): WorkloadOpsProvider {
  return {
    cordonNode: (name, unschedulable) =>
      run(() => {
        findNode(name).unschedulable = unschedulable
      }),
    drainNode: (name, options) => run(() => drainMockNode(name, options, suffix)),
    setNodeTaints: (name, taints) =>
      run(() => {
        const node = findNode(name)
        node.taints = taints.map((t) => ({ ...t }))
        return node.taints.map((t) => ({ ...t }))
      }),

    deploymentHistory: (ns, name) => run(() => mockRolloutHistory(ns, name)),
    rollbackDeployment: (ns, name, revision) => run(() => mockRollback(ns, name, revision)),
    pauseDeployment: (ns, name, paused) => run(() => mockPauseDeployment(ns, name, paused)),

    restartWorkload: (kind, ns, name) =>
      run(() => {
        if (kind === 'Deployment') {
          if (!deployments.some((d) => d.namespace === ns && d.name === name)) throw notFound('deployments.apps', name)
        } else {
          const template = child(child(findObject(kind, ns, name), 'spec'), 'template')
          child(child(template, 'metadata'), 'annotations')['kubectl.kubernetes.io/restartedAt'] = MOCK_NOW
        }
        refresh(pods.filter((p) => p.namespace === ns && p.controlledBy === `${kind}/${name}`))
      }),
    scaleWorkload: (kind, ns, name, replicas) =>
      run(() => {
        if (replicas < 0) throw badRequest('replicas must be at least 0')
        if (kind === 'Deployment') {
          const d = deployments.find((x) => x.namespace === ns && x.name === name)
          if (!d) throw notFound('deployments.apps', name)
          syncDeploymentPods(d, replicas)
          d.replicas = d.upToDate = replicas
          d.readyReplicas = d.available = Math.min(replicas, d.readyReplicas)
          return
        }
        const obj = findObject(kind, ns, name)
        child(obj, 'spec').replicas = replicas
        Object.assign(child(obj, 'status'), { replicas, readyReplicas: replicas })
        if (kind === 'StatefulSet') syncStatefulSetPods(ns, name, replicas)
      }),

    forceDeletePod: (ns, name) =>
      run(() => {
        pods.splice(pods.indexOf(findPod(ns, name)), 1)
      }),
    deletePods: (items) =>
      run(() => {
        if (items.length === 0 || items.length > MAX_BULK_DELETE) {
          throw badRequest(`a bulk delete takes 1 to ${MAX_BULK_DELETE} pods, not ${items.length}`)
        }
        if (items.some((item) => !item.namespace || !item.name)) throw badRequest('every pod needs a namespace and a name')
        return items.map((item): PodDeleteResult => {
          const index = pods.findIndex((p) => p.namespace === item.namespace && p.name === item.name)
          if (index < 0) return { ...item, error: `pods "${item.name}" not found` }
          pods.splice(index, 1)
          return { ...item }
        })
      }),
    addDebugContainer: (ns, podName, { image, target }) =>
      run(() => {
        const pod = findPod(ns, podName)
        if (image && /\s/.test(image)) throw badRequest(`the image "${image}" contains white space`)
        if (target && !pod.containers.some((c) => c.name === target)) {
          throw badRequest(`pod ${podName} has no container "${target}" to target`)
        }
        return { namespace: ns, pod: podName, container: `debugger-${suffix()}` }
      }),

    triggerCronJob: (ns, name) =>
      run(() => {
        findObject('CronJob', ns, name)
        const job = `${name}-manual-${suffix()}`
        genericMock.jobs.push({
          kind: 'Job',
          metadata: {
            name: job,
            namespace: ns,
            labels: {},
            annotations: { 'cronjob.kubernetes.io/instantiate': 'manual' },
            ownerReferences: [{ apiVersion: 'batch/v1', kind: 'CronJob', name, controller: true }],
          },
          age: '0s',
          status: { active: 1 },
          spec: { completions: 1 },
        })
        return { namespace: ns, job }
      }),
    suspendCronJob: (ns, name, suspend) =>
      run(() => {
        child(findObject('CronJob', ns, name), 'spec').suspend = suspend
      }),

    createNamespace: (name) =>
      run(() => {
        if (name.length > 63 || !DNS1123_LABEL.test(name)) {
          throw badRequest(`namespace name "${name}" must be a DNS-1123 label`)
        }
        if (namespaces.some((n) => n.name === name)) {
          throw new ApiError(409, 'kubernetes', `namespaces "${name}" already exists`)
        }
        namespaces.push({ name, status: 'Active', age: '0s' })
      }),
    deleteNamespace: (name, force) =>
      run(() => {
        const index = namespaces.findIndex((n) => n.name === name)
        if (index < 0) throw notFound('namespaces', name)
        if (!force && SYSTEM_NAMESPACES.includes(name)) {
          throw new ApiError(403, 'forbidden', `${name} is a system namespace; KubeBench deletes it only when forced`)
        }
        if (!force && name === MOCK_OWN_NAMESPACE) {
          throw new ApiError(403, 'forbidden', `${name} is the namespace KubeBench runs in; KubeBench deletes it only when forced`)
        }
        namespaces.splice(index, 1)
        removeWhere(pods, (p) => p.namespace === name)
        removeWhere(deployments, (d) => d.namespace === name)
        removeWhere(services, (s) => s.namespace === name)
        for (const list of Object.values(genericMock)) removeWhere(list, (o) => o.metadata?.namespace === name)
      }),
  }
}
