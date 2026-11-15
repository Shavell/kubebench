import type { WorkloadOpsProvider } from '@/data/providers'
import type { Deployment, K8sNode, KubeObject, Pod, RestartableKind, ScalableKind } from '@/data/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { notify } from '@/shared/notify'
import { tw } from '../i18n'
import { openOpsDialog } from './opsDialogs'
import { runOp } from './runOp'
import { controllerOf, desiredReplicas, isSuspended } from './targets'

/**
 * The workload and node operations behind row menu items and detail
 * toolbar buttons. Both run the same functions, so they behave alike.
 */

const meta = (obj: KubeObject) => ({ namespace: obj.metadata?.namespace ?? '', name: obj.metadata?.name ?? '' })

export function toggleCordon(ops: WorkloadOpsProvider, node: K8sNode): Promise<void> {
  const cordon = !node.unschedulable
  return runOp(
    () => ops.cordonNode(node.name, cordon),
    tw(cordon ? 'workloadOps.node.cordoned' : 'workloadOps.node.uncordoned', { name: node.name }),
  )
}

export const drainNode = (node: K8sNode) => openOpsDialog({ type: 'drain', node: node.name })
export const editTaints = (node: K8sNode) => openOpsDialog({ type: 'taints', node })

export function togglePause(ops: WorkloadOpsProvider, d: Deployment): Promise<void> {
  const pause = !d.paused
  return runOp(
    () => ops.pauseDeployment(d.namespace, d.name, pause),
    tw(pause ? 'workloadOps.rollout.paused' : 'workloadOps.rollout.resumed', { name: d.name }),
  )
}

export const showRolloutHistory = (d: Deployment) =>
  openOpsDialog({ type: 'rolloutHistory', deployment: { namespace: d.namespace, name: d.name } })

export function restartWorkload(ops: WorkloadOpsProvider, kind: RestartableKind, obj: KubeObject): Promise<void> {
  const { namespace, name } = meta(obj)
  return runOp(() => ops.restartWorkload(kind, namespace, name), tw('workloadOps.workload.restarted', { kind, name }))
}

export function scaleWorkload(kind: ScalableKind, obj: KubeObject) {
  openOpsDialog({ type: 'scale', kind, target: meta(obj), replicas: desiredReplicas(obj), managedBy: controllerOf(obj) })
}

/** Runs a CronJob now; the toast offers to open the new Job. */
export function triggerCronJob(ops: WorkloadOpsProvider, obj: KubeObject): Promise<void> {
  const { namespace, name } = meta(obj)
  return runOp(
    () => ops.triggerCronJob(namespace, name),
    (run) =>
      notify.success(tw('workloadOps.cronjob.triggered', { job: run.job, name }), {
        action: { label: tw('workloadOps.cronjob.openJob'), run: () => openJob(run.namespace, run.job) },
      }),
  )
}

function openJob(namespace: string, name: string) {
  useLayoutStore.getState().openTab({
    id: `res:jobs:${namespace}/${name}`,
    title: name,
    icon: 'checklist',
    kind: 'generic-detail',
    props: { resourceKey: 'jobs', namespace, name },
  })
}

export function toggleSuspend(ops: WorkloadOpsProvider, obj: KubeObject): Promise<void> {
  const { namespace, name } = meta(obj)
  const suspend = !isSuspended(obj)
  return runOp(
    () => ops.suspendCronJob(namespace, name, suspend),
    tw(suspend ? 'workloadOps.cronjob.suspended' : 'workloadOps.cronjob.resumed', { name }),
  )
}

export const debugPod = (pod: Pod) => openOpsDialog({ type: 'debug', pod })

export const forceDeletePod = (pod: Pod, onDeleted?: () => void) =>
  openOpsDialog({ type: 'forceDeletePod', pod: { namespace: pod.namespace, name: pod.name }, onDeleted })

export const createNamespace = () => openOpsDialog({ type: 'createNamespace' })
export const deleteNamespace = (name: string) => openOpsDialog({ type: 'deleteNamespace', name })
