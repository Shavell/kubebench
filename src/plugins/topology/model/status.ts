import type { RawConfig, RawDaemonSet, RawPod, RawPv, RawPvc, RawReplicated, RawService } from './k8s'
import type { NodeStatus, NodeTone } from './types'

/**
 * Status of each kind, from the fields its controller maintains. Keys are
 * i18n keys of this plugin (`status.*`); Kubernetes values (phases,
 * waiting reasons) are shown as the API reports them.
 */

const status = (tone: NodeTone, key: string, values?: NodeStatus['values']): NodeStatus => ({ tone, key, values })

export const UNKNOWN: NodeStatus = status('neutral', 'status.unknown')
export const MISSING: NodeStatus = status('missing', 'status.missing')

/** Waiting reasons a container does not get out of by itself. */
const FAILING_REASONS = new Set([
  'CrashLoopBackOff',
  'ImagePullBackOff',
  'ErrImagePull',
  'InvalidImageName',
  'CreateContainerConfigError',
  'CreateContainerError',
  'RunContainerError',
])

export function podStatus(pod: RawPod): NodeStatus {
  if (pod.metadata?.deletionTimestamp) return status('warning', 'status.terminating')
  const containers = pod.status?.containerStatuses ?? []
  const failing = containers.find((c) => FAILING_REASONS.has(c.state?.waiting?.reason ?? ''))
  if (failing) return status('error', 'status.waiting', { reason: failing.state?.waiting?.reason ?? '' })
  const phase = pod.status?.phase ?? 'Unknown'
  switch (phase) {
    case 'Running': {
      const ready = containers.filter((c) => c.ready).length
      const total = containers.length || ready
      return status(ready === total ? 'ok' : 'warning', 'status.ready', { ready, total })
    }
    case 'Pending':
      return status('warning', 'status.phase', { phase })
    case 'Failed':
      return status('error', 'status.phase', { phase: pod.status?.reason || phase })
    default:
      return status('neutral', 'status.phase', { phase })
  }
}

/** Ready against desired replicas, for Deployments, ReplicaSets, StatefulSets and DaemonSets. */
function replicaStatus(ready: number, desired: number): NodeStatus {
  if (desired === 0) return status('neutral', 'status.scaledToZero')
  const tone: NodeTone = ready >= desired ? 'ok' : ready === 0 ? 'error' : 'warning'
  return status(tone, 'status.replicas', { ready, desired })
}

/** Deployment, ReplicaSet or StatefulSet; `spec.replicas` defaults to 1 as in the API. */
export function replicatedStatus(obj: RawReplicated): NodeStatus {
  return replicaStatus(obj.status?.readyReplicas ?? 0, obj.spec?.replicas ?? 1)
}

export function daemonSetStatus(ds: RawDaemonSet): NodeStatus {
  return replicaStatus(ds.status?.numberReady ?? 0, ds.status?.desiredNumberScheduled ?? 0)
}

/**
 * A Service by the Pods it selects (`selected` undefined while Pods are
 * not loaded): no ready Pod is an outage, some is degraded.
 */
export function serviceStatus(svc: RawService, selected: NodeTone[] | undefined): NodeStatus {
  if (svc.spec?.type === 'ExternalName') {
    return status('neutral', 'status.externalName', { target: svc.spec.externalName ?? '' })
  }
  if (!svc.spec?.selector || Object.keys(svc.spec.selector).length === 0) return status('neutral', 'status.noSelector')
  if (!selected) return UNKNOWN
  if (selected.length === 0) return status('warning', 'status.noPods')
  const ready = selected.filter((tone) => tone === 'ok').length
  const tone: NodeTone = ready === selected.length ? 'ok' : ready === 0 ? 'error' : 'warning'
  return status(tone, 'status.endpoints', { ready, total: selected.length })
}

/** An Ingress by its backend Services (`missing` of `total` not found). */
export function ingressStatus(total: number, missing: number): NodeStatus {
  if (total === 0) return status('neutral', 'status.noBackends')
  if (missing > 0) return status('warning', 'status.missingBackends', { missing, total })
  return status('ok', 'status.backends', { total })
}

const PVC_TONES: Record<string, NodeTone> = { Bound: 'ok', Pending: 'warning', Lost: 'error' }
const PV_TONES: Record<string, NodeTone> = { Bound: 'ok', Available: 'neutral', Released: 'warning', Failed: 'error' }

export function pvcStatus(pvc: RawPvc): NodeStatus {
  const phase = pvc.status?.phase ?? 'Pending'
  return status(PVC_TONES[phase] ?? 'neutral', 'status.phase', { phase })
}

export function pvStatus(pv: RawPv): NodeStatus {
  const phase = pv.status?.phase ?? 'Pending'
  return status(PV_TONES[phase] ?? 'neutral', 'status.phase', { phase })
}

/** ConfigMaps and Secrets: how many keys they hold (Secret values stay redacted, keys do not). */
export function configStatus(obj: RawConfig): NodeStatus {
  const keys = Object.keys(obj.data ?? {}).length + Object.keys(obj.binaryData ?? {}).length
  return status('neutral', 'status.keys', { total: keys })
}
