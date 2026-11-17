import { formatDateTime } from '@/i18n/format'
import { Icon } from '@/shared/Icon'
import type { Note, NoteObjectRef, NoteVisibility } from '../data/types'
import { useNotesTranslation } from '../i18n'
import { objectLabel } from '../model/rules'
import { canOpenObject, openObject, VISIBILITY_ICONS } from '../tabs'

/** Personal / Shared with its icon. */
export function VisibilityLabel({ visibility }: { visibility: NoteVisibility }) {
  const { t } = useNotesTranslation()
  return (
    <span className={`kb-notes-visibility kb-notes-visibility--${visibility}`}>
      <Icon name={VISIBILITY_ICONS[visibility]} size={12} />
      {t(`visibility.${visibility}`)}
    </span>
  )
}

/** Where a note lives: its object (a link), its cluster, or everywhere. */
export function NotePlacement({
  resource,
  clusterName,
  global,
}: {
  resource: NoteObjectRef | null
  clusterName: string
  global: boolean
}) {
  const { t } = useNotesTranslation()
  if (global) {
    return (
      <span className="kb-notes-placement">
        <Icon name="globe" size={12} />
        {t('placement.global')}
      </span>
    )
  }
  return (
    <span className="kb-notes-placement">
      <Icon name="server" size={12} />
      {clusterName}
      {resource && (
        <>
          <Icon name="symbol-field" size={12} />
          {canOpenObject(resource) ? (
            <button type="button" className="kb-notes-link" onClick={() => openObject(resource)}>
              {objectLabel(resource)}
            </button>
          ) : (
            objectLabel(resource)
          )}
        </>
      )}
    </span>
  )
}

/** "Created by ann · 2 Nov 2026, 09:00 · edited by ben · …". */
export function NoteAuthors({ note }: { note: Note }) {
  const { t } = useNotesTranslation()
  const who = (name: string) => name || t('deletedUser')
  const edited = note.version > 1
  return (
    <span className="kb-notes-authors">
      {t('meta.created', { user: who(note.createdBy), date: formatDateTime(note.createdAt) })}
      {edited && <> · {t('meta.updated', { user: who(note.updatedBy), date: formatDateTime(note.updatedAt) })}</>}
    </span>
  )
}

/** The tags of a note as small chips. */
export function NoteTags({ tags }: { tags: string[] }) {
  if (tags.length === 0) return null
  return (
    <span className="kb-notes-tags">
      {tags.map((tag) => (
        <span key={tag} className="kb-notes-tag">
          #{tag}
        </span>
      ))}
    </span>
  )
}
