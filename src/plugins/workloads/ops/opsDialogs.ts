import { create } from 'zustand'
import type { DrainResult, K8sNode, Pod, PodKey, ScalableKind } from '@/data/types'

/** A namespaced object an operation targets. */
export interface ObjectKey {
  namespace: string
  name: string
}

/**
 * The operation dialogs (design §7.3). Row menus and toolbar buttons ask
 * for one with openOpsDialog(); the <WorkloadOpsDialogs/> of the current
 * page renders it.
 */
export type OpsDialogRequest =
  | { type: 'drain'; node: string }
  | { type: 'drainResult'; result: DrainResult }
  | { type: 'taints'; node: K8sNode }
  | { type: 'rolloutHistory'; deployment: ObjectKey }
  | { type: 'scale'; kind: ScalableKind; target: ObjectKey; replicas: number; managedBy?: string }
  | { type: 'forceDeletePod'; pod: PodKey; onDeleted?: () => void }
  | { type: 'deletePods'; pods: PodKey[]; onDeleted?: () => void }
  | { type: 'debug'; pod: Pod }
  | { type: 'createNamespace' }
  | { type: 'deleteNamespace'; name: string }

export const useOpsDialog = create<{ request: OpsDialogRequest | null }>(() => ({ request: null }))

export function openOpsDialog(request: OpsDialogRequest): void {
  useOpsDialog.setState({ request })
}

export function closeOpsDialog(): void {
  useOpsDialog.setState({ request: null })
}
