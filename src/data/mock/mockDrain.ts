import { ApiError } from '../apiClient'
import type { DrainOptions, DrainResult, K8sNode, Pod } from '../types'
import { nodes, pods } from './mockCluster'
import { genericMock } from './mockGeneric'

/**
 * A drain of a mock node, following the rules of kubectl drain: DaemonSet
 * Pods are skipped (or stop the drain), unmanaged Pods need `force`, and
 * PodDisruptionBudgets keep Pods whose eviction would leave too few healthy
 * ones. Evicted Pods of a controller come back on another schedulable node.
 */

const PDB_MESSAGE = "Cannot evict pod as it would violate the pod's disruption budget."

const keyOf = (p: Pod) => `${p.namespace}/${p.name}`
const ownerKind = (p: Pod) => p.controlledBy?.split('/')[0]

/** The mock's control-plane components have no controller: they stand for static (mirror) Pods. */
const isMirror = (p: Pod) => !p.controlledBy && p.namespace === 'kube-system'

/** Disruptions each mock PodDisruptionBudget still allows (budgets select Pods by their name = app label). */
function disruptionBudgets(): Map<string, number> {
  const allowed = new Map<string, number>()
  for (const pdb of genericMock.poddisruptionbudgets ?? []) {
    const namespace = pdb.metadata?.namespace ?? ''
    const app = pdb.metadata?.name ?? ''
    const minAvailable = (pdb.spec as { minAvailable?: unknown } | undefined)?.minAvailable
    if (typeof minAvailable !== 'number') continue
    const healthy = pods.filter((p) => p.namespace === namespace && p.labels.app === app && p.status === 'Running').length
    allowed.set(`${namespace}/${app}`, healthy - minAvailable)
  }
  return allowed
}

function schedulable(n: K8sNode, drained: string): boolean {
  return (
    n.name !== drained &&
    !n.unschedulable &&
    n.status === 'Ready' &&
    !(n.taints ?? []).some((t) => t.effect === 'NoSchedule' || t.effect === 'NoExecute')
  )
}

/** What the Pod's controller creates in its place: Pending when no node can take it. */
function replacement(pod: Pod, drained: string, suffix: () => string): Pod {
  const target = nodes.find((n) => schedulable(n, drained))
  const statefulSet = ownerKind(pod) === 'StatefulSet'
  return {
    ...pod,
    name: statefulSet ? pod.name : `${pod.name.replace(/-[^-]+$/, '')}-${suffix()}`,
    node: target?.name ?? '',
    status: target ? 'Running' : 'Pending',
    readyContainers: target ? pod.totalContainers : 0,
    restarts: 0,
    age: '0s',
    podIP: target ? pod.podIP : '',
  }
}

/** Cordons and drains the mock node `name`; `suffix` names the replacement Pods. */
export function drainMockNode(name: string, o: DrainOptions, suffix: () => string): DrainResult {
  const node = nodes.find((n) => n.name === name)
  if (!node) throw new ApiError(404, 'kubernetes', `nodes "${name}" not found`)
  node.unschedulable = true
  const result: DrainResult = {
    node: name,
    cordoned: true,
    completed: false,
    evicted: [],
    failed: [],
    blockedByPDB: [],
    skipped: [],
    errors: [],
  }

  const refusals = new Map<string, string[]>()
  const refuse = (reason: string, pod: Pod) => refusals.set(reason, [...(refusals.get(reason) ?? []), keyOf(pod)])
  const leaving: Pod[] = []
  for (const pod of pods.filter((p) => p.node === name)) {
    if (isMirror(pod)) {
      result.skipped.push({ pod: keyOf(pod), reason: 'mirror' })
    } else if (ownerKind(pod) === 'DaemonSet') {
      if (o.ignoreDaemonSets) result.skipped.push({ pod: keyOf(pod), reason: 'daemonSet' })
      else refuse('DaemonSet-managed Pods (use --ignore-daemonsets to ignore)', pod)
    } else if (!pod.controlledBy && !o.force) {
      refuse('Pods that declare no controller (use --force to override)', pod)
    } else {
      leaving.push(pod)
    }
  }
  result.skipped.sort((a, b) => a.pod.localeCompare(b.pod))
  if (refusals.size > 0) {
    // Like kubectl drain: refuse before evicting anything.
    result.errors = [...refusals].map(([reason, list]) => `cannot delete ${reason}: ${list.join(', ')}`).sort()
    return result
  }

  const budgets = disruptionBudgets()
  for (const pod of leaving) {
    const budget = `${pod.namespace}/${pod.labels.app}`
    const allowed = budgets.get(budget)
    if (allowed !== undefined && allowed <= 0) {
      result.failed.push({ pod: keyOf(pod), error: PDB_MESSAGE })
      result.blockedByPDB.push(keyOf(pod))
      continue
    }
    if (allowed !== undefined) budgets.set(budget, allowed - 1)
    pods.splice(pods.indexOf(pod), 1)
    if (pod.controlledBy) pods.push(replacement(pod, name, suffix))
    result.evicted.push(keyOf(pod))
  }
  result.evicted.sort()
  result.blockedByPDB.sort()
  result.failed.sort((a, b) => a.pod.localeCompare(b.pod))
  result.completed = result.failed.length === 0
  return result
}
