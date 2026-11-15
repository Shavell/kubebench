import { can } from '@/core/permissions'
import type { ResourceRef } from '@/core/plugin/types'
import type { DataProviders } from '@/data/providers'
import type { Deployment, K8sNode, KubeObject, Pod } from '@/data/types'
import type { ContextMenuItem } from '@/shared/contextMenu'
import { tw } from '../i18n'
import * as act from './actions'
import { isRestartableObject, isScalableObject, isSuspended, isSystemNamespace } from './targets'

/**
 * Row context menu entries of the workload and node operations (design
 * §7.3), for rowActions.tsx. Like the other row actions, entries the user
 * may not run are left out.
 */

const SEPARATOR: ContextMenuItem = { type: 'separator' }

/** The permitted items behind a separator; nothing when none is permitted. */
function section(items: (ContextMenuItem | false)[]): ContextMenuItem[] {
  const kept = items.filter((item): item is ContextMenuItem => item !== false)
  return kept.length > 0 ? [SEPARATOR, ...kept] : []
}

export function podOpsItems(pod: Pod): ContextMenuItem[] {
  return section([
    can('exec') && { label: tw('workloadOps.debug.menu'), icon: 'debug', onClick: () => act.debugPod(pod) },
    can('write') && {
      label: tw('workloadOps.pods.forceDelete'),
      icon: 'trash',
      danger: true,
      onClick: () => act.forceDeletePod(pod),
    },
  ])
}

export function deploymentOpsItems(d: Deployment, { workloadOps }: DataProviders): ContextMenuItem[] {
  return section([
    { label: tw('workloadOps.rollout.historyMenu'), icon: 'history', onClick: () => act.showRolloutHistory(d) },
    can('write') && {
      label: tw(d.paused ? 'workloadOps.rollout.resume' : 'workloadOps.rollout.pause'),
      icon: d.paused ? 'debug-continue' : 'debug-pause',
      onClick: () => void act.togglePause(workloadOps, d),
    },
  ])
}

function nodeItems(node: K8sNode, { workloadOps }: DataProviders): ContextMenuItem[] {
  const write = can('write')
  return section([
    write && {
      label: tw(node.unschedulable ? 'workloadOps.node.uncordon' : 'workloadOps.node.cordon'),
      icon: node.unschedulable ? 'pass' : 'circle-slash',
      onClick: () => void act.toggleCordon(workloadOps, node),
    },
    write && { label: tw('workloadOps.node.drain'), icon: 'debug-step-out', danger: true, onClick: () => act.drainNode(node) },
    write && { label: tw('workloadOps.node.editTaints'), icon: 'tag', onClick: () => act.editTaints(node) },
  ])
}

function namespaceItems(name: string): ContextMenuItem[] {
  const system = isSystemNamespace(name)
  return section([
    can('write') && {
      label: tw('workloadOps.namespace.delete'),
      icon: 'trash',
      danger: true,
      disabled: system,
      title: system ? tw('workloadOps.namespace.systemReason') : undefined,
      onClick: () => act.deleteNamespace(name),
    },
  ])
}

function objectItems(kind: string, obj: KubeObject, { workloadOps }: DataProviders): ContextMenuItem[] {
  const write = can('write')
  if (kind === 'CronJob') {
    return section([
      write && { label: tw('workloadOps.cronjob.trigger'), icon: 'play', onClick: () => void act.triggerCronJob(workloadOps, obj) },
      write && {
        label: tw(isSuspended(obj) ? 'workloadOps.cronjob.resume' : 'workloadOps.cronjob.suspend'),
        icon: isSuspended(obj) ? 'debug-continue' : 'debug-pause',
        onClick: () => void act.toggleSuspend(workloadOps, obj),
      },
    ])
  }
  return section([
    write &&
      isRestartableObject(kind) && {
        label: tw('common.restart'),
        icon: 'sync',
        onClick: () => void act.restartWorkload(workloadOps, kind, obj),
      },
    write &&
      isScalableObject(kind) && { label: tw('workloadOps.workload.scale'), icon: 'arrow-both', onClick: () => act.scaleWorkload(kind, obj) },
  ])
}

/**
 * Items for a row of the node, namespace or generic lists; `row` is the
 * row's object (K8sNode, Namespace or KubeObject). Kinds without
 * operations get none.
 */
export function objectOpsItems(ref: ResourceRef, row: unknown, providers: DataProviders): ContextMenuItem[] {
  if (!row) return []
  switch (ref.kind) {
    case 'Node':
      return nodeItems(row as K8sNode, providers)
    case 'Namespace':
      return namespaceItems(ref.name)
    default:
      return objectItems(ref.kind, row as KubeObject, providers)
  }
}
