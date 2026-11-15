import type { KubeObject } from '@/data/types'

/**
 * Facts about rows of the generic lists that the operations need, read
 * from the raw objects.
 */

/** Kubernetes creates and needs these; KubeBench never offers to delete them. */
export const SYSTEM_NAMESPACES = ['default', 'kube-system', 'kube-public', 'kube-node-lease']

export const isSystemNamespace = (name: string) => SYSTEM_NAMESPACES.includes(name)

/** Generic-list kinds KubeBench restarts (Deployments have their own page). */
export const isRestartableObject = (kind: string): kind is 'StatefulSet' | 'DaemonSet' =>
  kind === 'StatefulSet' || kind === 'DaemonSet'

/** Generic-list kinds KubeBench scales. */
export const isScalableObject = (kind: string): kind is 'StatefulSet' | 'ReplicaSet' =>
  kind === 'StatefulSet' || kind === 'ReplicaSet'

function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' ? (value as Record<string, unknown>) : {}
}

/** spec.replicas, which defaults to 1. */
export function desiredReplicas(obj: KubeObject): number {
  const replicas = record(obj.spec).replicas
  return typeof replicas === 'number' ? replicas : 1
}

export function isSuspended(obj: KubeObject): boolean {
  return record(obj.spec).suspend === true
}

/** "<Kind>/<name>" of the object's controller, e.g. the Deployment of a ReplicaSet. */
export function controllerOf(obj: KubeObject): string | undefined {
  const refs = obj.metadata?.ownerReferences
  if (!Array.isArray(refs)) return undefined
  const controller = refs.map(record).find((ref) => ref.controller === true)
  return controller ? `${String(controller.kind)}/${String(controller.name)}` : undefined
}
