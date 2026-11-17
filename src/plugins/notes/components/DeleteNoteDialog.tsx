import { refreshData } from '@/data/DataContext'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { notify } from '@/shared/notify'
import { notesData } from '../data'
import type { Note } from '../data/types'
import { useNotesTranslation } from '../i18n'

/** Asks before deleting a note (the title is typed on production clusters). */
export function DeleteNoteDialog({
  note,
  onClose,
  onDeleted,
}: {
  note: Note | null
  onClose: () => void
  onDeleted?: () => void
}) {
  const { t } = useNotesTranslation()
  if (!note) return null
  return (
    <ConfirmDialog
      open
      danger
      title={t('delete.title')}
      message={t(note.visibility === 'shared' ? 'delete.messageShared' : 'delete.messagePersonal', { title: note.title })}
      confirmLabel={t('delete.confirm')}
      confirmPhrase={note.title}
      onClose={onClose}
      onConfirm={async () => {
        await notesData().remove(note)
        notify.success(t('delete.done', { title: note.title }))
        onDeleted?.()
        refreshData()
      }}
    />
  )
}
