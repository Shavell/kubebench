import { usePermissions } from '@/core/permissions'
import { useDataMode } from '@/data/mode'
import type { Note } from '../data/types'
import { useNotesTranslation } from '../i18n'

/**
 * What the signed-in user may do with notes (design §7.13): personal notes
 * always; shared notes of the active cluster need the operator or admin
 * role — a read-only cluster does not matter, notes are KubeBench data.
 * Deleting a shared note needs its author or an admin. The server enforces
 * the same rules; this only explains disabled controls.
 */
export interface NoteAccess {
  /** Why shared notes cannot be written, or null when they can. */
  shareDenied: string | null
  /** Why `note` cannot be edited or pinned, or null. */
  editDenied: (note: Note) => string | null
  /** Why `note` cannot be deleted, or null. */
  deleteDenied: (note: Note) => string | null
}

export function useNoteAccess(): NoteAccess {
  const { t } = useNotesTranslation()
  const { role } = usePermissions()
  const { user } = useDataMode()
  const shareDenied = role === 'admin' || role === 'operator' ? null : t('access.viewer')
  const editDenied = (note: Note) => (note.visibility === 'shared' ? shareDenied : null)
  return {
    shareDenied,
    editDenied,
    deleteDenied: (note) =>
      editDenied(note) ??
      (note.visibility === 'shared' && role !== 'admin' && note.createdBy !== user ? t('access.notAuthor') : null),
  }
}
