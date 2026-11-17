import type { TabDescriptor } from '@/core/plugin/types'
import { EmptyState } from '@/shared/EmptyState'
import { Unavailable } from '@/shared/Unavailable'
import { refreshData } from '@/data/DataContext'
import { useNotesQuery } from '../data/useNotesQuery'
import { useNotesTranslation } from '../i18n'
import type { NoteDraft } from '../tabs'
import { NoteEditor } from '../components/NoteEditor'
import '../notes.css'

/**
 * The `notes-editor` tab (design §7.13): props `{id}` edit a stored note,
 * `{draft}` write a new one.
 */
export function NoteEditorTab({ tab }: { tab: TabDescriptor }) {
  const id = typeof tab.props?.id === 'number' ? tab.props.id : null
  const draft = (tab.props?.draft ?? null) as NoteDraft | null
  if (id !== null) return <StoredNoteEditor tabId={tab.id} id={id} />
  if (draft) return <NoteEditor tabId={tab.id} draft={draft} />
  return null
}

function StoredNoteEditor({ tabId, id }: { tabId: string; id: number }) {
  const { t } = useNotesTranslation()
  const note = useNotesQuery((notes) => notes.get(id), [id])
  if (note.data) return <NoteEditor tabId={tabId} note={note.data} />
  if (note.error) return <Unavailable code={note.errorCode} reason={note.error} onRetry={refreshData} />
  return <EmptyState icon="loading" title={t('editor.loading')} />
}
