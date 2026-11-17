import { useState } from 'react'
import { refreshData } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { Icon } from '@/shared/Icon'
import { Markdown } from '@/shared/markdown/Markdown'
import { notify } from '@/shared/notify'
import { notesData } from '../data'
import type { Note } from '../data/types'
import { useNotesTranslation } from '../i18n'
import { useNoteAccess } from '../model/access'
import { openNote } from '../tabs'
import { DeleteNoteDialog } from './DeleteNoteDialog'
import { NoteAuthors, NoteTags, VisibilityLabel } from './NoteMeta'

/** A note rendered in full, with pin, edit and delete. */
export function NoteCard({ note }: { note: Note }) {
  const { t } = useNotesTranslation()
  const access = useNoteAccess()
  const [deleting, setDeleting] = useState(false)
  const [pinning, setPinning] = useState(false)
  const editDenied = access.editDenied(note)
  const deleteDenied = access.deleteDenied(note)

  const togglePin = async () => {
    setPinning(true)
    try {
      await notesData().pin(note, !note.pinned)
      refreshData()
    } catch (err) {
      notify.error(err)
    } finally {
      setPinning(false)
    }
  }

  return (
    <article className="kb-notes-card" aria-label={note.title}>
      <header className="kb-notes-card__header">
        {note.pinned && <Icon name="pinned" size={14} className="kb-notes-card__pin" />}
        <h3 className="kb-notes-card__title">{note.title}</h3>
        <VisibilityLabel visibility={note.visibility} />
        <NoteTags tags={note.tags} />
        <span className="kb-notes-card__actions">
          <Button
            variant="secondary"
            icon={note.pinned ? 'pinned' : 'pin'}
            busy={pinning}
            disabled={editDenied !== null}
            title={editDenied ?? (note.pinned ? t('actions.unpin') : t('actions.pin'))}
            aria-label={note.pinned ? t('actions.unpin') : t('actions.pin')}
            onClick={togglePin}
          />
          <Button variant="secondary" icon={editDenied ? 'eye' : 'edit'} onClick={() => openNote(note)}>
            {editDenied ? t('actions.open') : t('actions.edit')}
          </Button>
          <Button
            variant="secondary"
            icon="trash"
            disabled={deleteDenied !== null}
            title={deleteDenied ?? t('actions.delete')}
            aria-label={t('actions.delete')}
            onClick={() => setDeleting(true)}
          />
        </span>
      </header>
      <div className="kb-notes-card__body">
        {note.body.trim() ? <Markdown source={note.body} headingOffset={2} /> : <p className="kb-notes-muted">{t('card.empty')}</p>}
      </div>
      <footer className="kb-notes-card__footer">
        <NoteAuthors note={note} />
      </footer>
      <DeleteNoteDialog note={deleting ? note : null} onClose={() => setDeleting(false)} />
    </article>
  )
}
