import { create } from 'zustand'
import type { ResourceRef } from '@/core/plugin/types'
import type { ObjectRef } from '@/data/types'

/** A resource dialog to show (design §7.2). */
export type ResourceDialog =
  | { type: 'delete'; target: ObjectRef; onDeleted?: () => void }
  | { type: 'metadata'; target: ObjectRef }
  | { type: 'reveal'; target: ObjectRef }

/**
 * The open resource dialog. Row menus and toolbars open one here and the
 * page's <ResourceDialogs/> renders it, so menus stay plain functions.
 */
export const useResourceDialogStore = create<{ dialog: ResourceDialog | null }>(() => ({ dialog: null }))

export const openResourceDialog = (dialog: ResourceDialog) => useResourceDialogStore.setState({ dialog })
export const closeResourceDialog = () => useResourceDialogStore.setState({ dialog: null })

/** The ObjectRef of a resource whose apiVersion is known, else null. */
export function targetOf(ref: ResourceRef): ObjectRef | null {
  if (!ref.apiVersion) return null
  return { apiVersion: ref.apiVersion, kind: ref.kind, namespace: ref.namespace || null, name: ref.name }
}

export const isSecret = (target: ObjectRef) => target.apiVersion === 'v1' && target.kind === 'Secret'

/** Namespaces are deleted through the namespace operations, which protect the system namespaces. */
export const deletable = (target: ObjectRef) => !(target.apiVersion === 'v1' && target.kind === 'Namespace')
