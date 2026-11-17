import type { NewNote, Note, NoteContent, NoteList, NoteObjectFilter, NoteQuery, ObjectNoteCount, TagCount } from './types'

/** The note a change applies to: mutations go to the route of its visibility. */
export type NoteKey = Pick<Note, 'id' | 'visibility'>

/**
 * The notes plugin's data capability (design §7.13). The live
 * implementation calls /api/plugins/notes; the mock one keeps notes in
 * memory with the same visibility and permission rules.
 */
export interface NotesProvider {
  list(query?: NoteQuery): Promise<NoteList>
  /** Rejects with `not_found` for notes the caller may not see. */
  get(id: number): Promise<Note>
  tags(query?: NoteQuery): Promise<TagCount[]>
  counts(resource: NoteObjectFilter): Promise<ObjectNoteCount[]>
  create(note: NewNote): Promise<Note>
  /** Rejects with `conflict` when `version` is no longer the stored one. */
  update(note: NoteKey, content: NoteContent, version: number): Promise<Note>
  pin(note: NoteKey, pinned: boolean): Promise<Note>
  remove(note: NoteKey): Promise<void>
}
