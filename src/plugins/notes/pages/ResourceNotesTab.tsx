import type { TabDescriptor } from '@/core/plugin/types'
import { refreshData } from '@/data/DataContext'
import { useDataMode } from '@/data/mode'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { Page } from '@/shared/Page'
import { Unavailable } from '@/shared/Unavailable'
import { NoteCard } from '../components/NoteCard'
import { NotePlacement } from '../components/NoteMeta'
import type { NoteObjectRef } from '../data/types'
import { useNotesQuery } from '../data/useNotesQuery'
import { useNotesTranslation } from '../i18n'
import { useNoteAccess } from '../model/access'
import { objectLabel } from '../model/rules'
import { openNewNote } from '../tabs'
import '../notes.css'

/** The `notes-resource` tab: every note on one object, and adding new ones. */
export function ResourceNotesTab({ tab }: { tab: TabDescriptor }) {
  const { t } = useNotesTranslation()
  const access = useNoteAccess()
  const { cluster } = useDataMode()
  const object = tab.props?.object as NoteObjectRef
  // Without an apiVersion (a host that did not know it) any API group matches.
  const filter = { apiVersion: object.apiVersion || undefined, kind: object.kind, namespace: object.namespace, name: object.name }
  const notes = useNotesQuery((p) => p.list({ resource: filter, body: true }), [
    object.apiVersion,
    object.kind,
    object.namespace,
    object.name,
  ])

  const actions = (
    <>
      <Button variant="secondary" icon="lock" onClick={() => openNewNote({ visibility: 'personal', resource: object })}>
        {t('resource.addPersonal')}
      </Button>
      <Button
        icon="organization"
        disabled={access.shareDenied !== null}
        title={access.shareDenied ?? undefined}
        onClick={() => openNewNote({ visibility: 'shared', resource: object })}
      >
        {t('resource.addShared')}
      </Button>
    </>
  )

  return (
    <Page title={t('resource.title', { object: objectLabel(object) })} actions={actions} className="kb-notes-page">
      <div className="kb-notes-page__placement">
        <NotePlacement resource={object} clusterName={cluster?.name ?? ''} global={false} />
      </div>
      {notes.error ? (
        <Unavailable code={notes.errorCode} reason={notes.error} onRetry={refreshData} />
      ) : notes.data === null ? (
        <EmptyState icon="loading" title={t('resource.loading')} />
      ) : notes.data.items.length === 0 ? (
        <EmptyState icon="note" title={t('resource.empty')} hint={t('resource.emptyHint')} />
      ) : (
        <div className="kb-notes-cards">
          {notes.data.items.map((note) => (
            <NoteCard key={note.id} note={note} />
          ))}
        </div>
      )}
    </Page>
  )
}
