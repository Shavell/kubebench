import { openYamlTab } from '@/core/plugin/editorTabs'
import type { ResourceRef } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { Note, NoteObjectRef, NoteVisibility } from './data/types'
import { NS, translate } from './i18n'

/**
 * Editor tabs of the notes plugin: one editor tab per note, drafts of new
 * notes, and the notes of one Kubernetes object.
 */
export const TAB_KINDS = {
  editor: 'notes-editor',
  resource: 'notes-resource',
} as const

export const NOTE_ICON = 'note'

export const VISIBILITY_ICONS: Record<NoteVisibility, string> = { personal: 'lock', shared: 'organization' }

/** A note not saved yet: where it goes and who sees it. */
export interface NoteDraft {
  visibility: NoteVisibility
  /** A personal note on no cluster. */
  global?: boolean
  resource?: NoteObjectRef
}

export const noteTabId = (id: number) => `${TAB_KINDS.editor}:${id}`

export function openNote(note: Pick<Note, 'id' | 'title'>): void {
  useLayoutStore.getState().openTab({
    id: noteTabId(note.id),
    title: note.title,
    icon: NOTE_ICON,
    kind: TAB_KINDS.editor,
    props: { id: note.id },
  })
}

let draftSeq = 0

/** Opens a new draft in its own tab (several drafts may be open at once). */
export function openNewNote(draft: NoteDraft): void {
  draftSeq += 1
  useLayoutStore.getState().openTab({
    id: `${TAB_KINDS.editor}:new:${draftSeq}`,
    title: translate('editor.newTitle'),
    titleKey: 'editor.newTitle',
    titleNs: NS,
    icon: NOTE_ICON,
    kind: TAB_KINDS.editor,
    props: { draft },
  })
}

/** The object of a resource action as a note reference (no namespace = cluster-scoped). */
export function noteObject(ref: ResourceRef): NoteObjectRef {
  return { apiVersion: ref.apiVersion ?? '', kind: ref.kind, namespace: ref.namespace ?? '', name: ref.name }
}

export function openResourceNotes(object: NoteObjectRef): void {
  useLayoutStore.getState().openTab({
    id: `${TAB_KINDS.resource}:${object.apiVersion}/${object.kind}:${object.namespace || '-'}/${object.name}`,
    title: translate('resource.tabTitle', { name: object.name }),
    titleKey: 'resource.tabTitle',
    titleNs: NS,
    titleValues: { name: object.name },
    icon: NOTE_ICON,
    kind: TAB_KINDS.resource,
    props: { object },
  })
}

/** Kinds with a rich detail page in the workloads plugin (`workloads-detail`). */
const RICH_KINDS = new Set(['Pod', 'Deployment'])

/** Whether openObject can open `object` (its YAML needs the apiVersion). */
export function canOpenObject(object: NoteObjectRef): boolean {
  return RICH_KINDS.has(object.kind) || object.apiVersion !== ''
}

/**
 * Opens the object a note is about: the workloads detail page for Pods and
 * Deployments, the object's YAML otherwise. Through tab kinds, so this
 * plugin imports neither view.
 */
export function openObject(object: NoteObjectRef): void {
  const namespace = object.namespace || null
  if (RICH_KINDS.has(object.kind)) {
    useLayoutStore.getState().openTab({
      id: `${object.kind.toLowerCase()}:${namespace ?? '-'}/${object.name}`,
      title: object.name,
      icon: 'symbol-field',
      kind: 'workloads-detail',
      props: { kind: object.kind, namespace, name: object.name },
    })
  } else if (object.apiVersion) {
    openYamlTab({ apiVersion: object.apiVersion, kind: object.kind, namespace: object.namespace || undefined, name: object.name })
  }
}
