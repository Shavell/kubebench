/** Shapes of /api/plugins/notes (design §7.13; backend internal/plugins/notes/types.go). */

export type NoteVisibility = 'personal' | 'shared'

/** The Kubernetes object a note is about. */
export interface NoteObjectRef {
  apiVersion: string
  kind: string
  /** Empty for cluster-scoped objects. */
  namespace: string
  name: string
}

export interface Note {
  id: number
  visibility: NoteVisibility
  /** null for global (personal) notes. */
  clusterId: number | null
  clusterName: string
  resource: NoteObjectRef | null
  title: string
  /** Markdown; empty in lists unless they asked for bodies. */
  body: string
  tags: string[]
  pinned: boolean
  /** Counts edits from 1; an update names the version it is based on. */
  version: number
  /** Usernames; empty once that user was deleted. */
  createdBy: string
  createdAt: string
  updatedBy: string
  updatedAt: string
}

export interface NoteList {
  items: Note[]
  /** More notes match than the limit returned. */
  truncated: boolean
}

/** An object to filter or count notes by; without apiVersion any API group matches. */
export interface NoteObjectFilter {
  apiVersion?: string
  kind?: string
  namespace?: string
  name?: string
}

export interface NoteQuery {
  scope?: 'all' | NoteVisibility
  /** `all` adds the caller's personal notes on other clusters. */
  clusters?: 'current' | 'all'
  resource?: NoteObjectFilter
  tag?: string
  /** Search terms, each matched in title or body. */
  q?: string
  /** Include bodies (default false). */
  body?: boolean
  limit?: number
}

export interface TagCount {
  tag: string
  count: number
}

export interface ObjectNoteCount extends NoteObjectRef {
  personal: number
  shared: number
}

/** The editable content of a note. */
export interface NoteContent {
  title: string
  body: string
  tags: string[]
}

export interface NewNote extends NoteContent {
  visibility: NoteVisibility
  pinned: boolean
  /** A personal note on no cluster (never with `resource`). */
  global: boolean
  resource?: NoteObjectRef
}
