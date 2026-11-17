import { useEffect, useState } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { isApiError } from '@/data/apiClient'
import { refreshData } from '@/data/DataContext'
import { useDataMode } from '@/data/mode'
import { Button } from '@/shared/Button'
import { CodeEditor } from '@/shared/editor/CodeEditor'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Checkbox, Field, Select, TextInput } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { Markdown } from '@/shared/markdown/Markdown'
import { notify } from '@/shared/notify'
import { Toolbar } from '@/shared/Toolbar'
import { notesData } from '../data'
import type { Note, NoteVisibility } from '../data/types'
import { useNotesTranslation } from '../i18n'
import { useNoteAccess } from '../model/access'
import { sameText, useEditorSession, type EditorSession, type EditorText, type EditorView } from '../model/editorSessions'
import { contentProblems, parseTags } from '../model/rules'
import { openNote, type NoteDraft } from '../tabs'
import { DeleteNoteDialog } from './DeleteNoteDialog'
import { NoteAuthors, NotePlacement, VisibilityLabel } from './NoteMeta'
import { ViewSwitch } from './ViewSwitch'

const EMPTY: EditorText = { title: '', body: '', tags: '' }

function textOf(note: Note): EditorText {
  return { title: note.title, body: note.body, tags: note.tags.join(', ') }
}

function sessionOf(note: Note, view: EditorView): EditorSession {
  const text = textOf(note)
  return { text, baseline: text, version: note.version, visibility: note.visibility, global: note.clusterId === null, view, conflict: false }
}

/**
 * Edits a stored note (`note`) or writes a new one (`draft`): title, tags,
 * visibility and placement of new notes, and the Markdown body with a
 * preview through the shared safe renderer. Unsaved changes mark the tab
 * dirty, so closing it asks first; a save that meets a newer version
 * offers to reload or to overwrite it.
 */
export function NoteEditor({ tabId, note, draft }: { tabId: string; note?: Note; draft?: NoteDraft }) {
  const { t } = useNotesTranslation()
  const access = useNoteAccess()
  const { cluster } = useDataMode()
  const setTabDirty = useLayoutStore((s) => s.setTabDirty)
  const [stored, update] = useEditorSession(tabId)
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [deleting, setDeleting] = useState(false)

  const readOnlyReason = note ? access.editDenied(note) : null
  const session: EditorSession = stored ?? (note
    ? sessionOf(note, readOnlyReason ? 'preview' : 'split')
    : { text: EMPTY, baseline: EMPTY, version: 0, visibility: draft?.visibility ?? 'personal', global: Boolean(draft?.global), view: 'split', conflict: false })
  const { text, visibility, view } = session
  const dirty = !sameText(text, session.baseline)

  useEffect(() => setTabDirty(tabId, dirty), [tabId, dirty, setTabDirty])

  // Another user's (or tab's) save arrived while nothing here was edited: show it.
  useEffect(() => {
    if (note && stored && !dirty && note.version !== stored.version) update(sessionOf(note, stored.view))
  }, [note, stored, dirty, update])

  const edit = (patch: Partial<EditorText>) => update({ ...session, text: { ...text, ...patch } })
  const problems = contentProblems(text.title, text.body, text.tags)
  const valid = !problems.title && !problems.body && !problems.tags
  const resource = note ? note.resource : (draft?.resource ?? null)
  const shareDenied = !note && visibility === 'shared' ? access.shareDenied : null
  const content = () => ({ title: text.title.trim(), body: text.body, tags: parseTags(text.tags).tags })

  const run = async (action: () => Promise<void>) => {
    setBusy(true)
    setError(null)
    try {
      await action()
    } catch (err) {
      if (note && isApiError(err, 'conflict')) update({ ...session, conflict: true })
      else setError(err)
    } finally {
      setBusy(false)
    }
  }

  const save = (version = session.version) =>
    run(async () => {
      if (!note) {
        const created = await notesData().create({
          ...content(),
          visibility,
          pinned: false,
          global: visibility === 'personal' && !resource && session.global,
          resource: resource ?? undefined,
        })
        setTabDirty(tabId, false)
        useLayoutStore.getState().closeTab(tabId)
        openNote(created)
        notify.success(t('editor.created', { title: created.title }))
      } else {
        const saved = await notesData().update(note, content(), version)
        update(sessionOf(saved, view))
        notify.success(t('editor.saved'))
      }
      refreshData()
    })

  /** Takes the stored note, dropping the edits. */
  const reload = () =>
    run(async () => {
      if (note) update(sessionOf(await notesData().get(note.id), view))
    })

  /** Saves the edits over the newer stored version. */
  const overwrite = () =>
    run(async () => {
      if (!note) return
      const latest = await notesData().get(note.id)
      update({ ...session, version: latest.version, conflict: false })
      const saved = await notesData().update(note, content(), latest.version)
      update(sessionOf(saved, view))
      notify.success(t('editor.saved'))
      refreshData()
    })

  const togglePin = () =>
    run(async () => {
      if (!note) return
      await notesData().pin(note, !note.pinned)
      refreshData()
    })

  const visibilityOptions: { value: NoteVisibility; label: string; disabled?: boolean }[] = [
    { value: 'personal', label: t('visibility.personal') },
    { value: 'shared', label: t('visibility.shared'), disabled: access.shareDenied !== null },
  ]

  return (
    <div className="kb-notes-editor">
      <Toolbar title={note ? note.title : t('editor.newTitle')}>
        <ViewSwitch view={view} onChange={(next) => update({ ...session, view: next })} />
        {note && (
          <>
            <Button
              variant="secondary"
              icon={note.pinned ? 'pinned' : 'pin'}
              disabled={busy || readOnlyReason !== null}
              title={readOnlyReason ?? undefined}
              onClick={togglePin}
            >
              {note.pinned ? t('actions.unpin') : t('actions.pin')}
            </Button>
            <Button
              variant="secondary"
              icon="trash"
              disabled={busy || access.deleteDenied(note) !== null}
              title={access.deleteDenied(note) ?? undefined}
              onClick={() => setDeleting(true)}
            >
              {t('actions.delete')}
            </Button>
          </>
        )}
        {!readOnlyReason && (
          <Button
            icon="save"
            busy={busy}
            disabled={!valid || shareDenied !== null || (note !== undefined && !dirty)}
            title={shareDenied ?? undefined}
            onClick={() => void save()}
          >
            {note ? t('actions.save') : t('actions.create')}
          </Button>
        )}
      </Toolbar>

      <div className="kb-notes-editor__meta">
        <div className="kb-notes-editor__line">
          {note && <VisibilityLabel visibility={note.visibility} />}
          <NotePlacement
            resource={resource}
            clusterName={note ? note.clusterName : (cluster?.name ?? '')}
            global={note ? note.clusterId === null : visibility === 'personal' && !resource && session.global}
          />
          {note && <NoteAuthors note={note} />}
        </div>
        {readOnlyReason && (
          <p className="kb-notes-notice">
            <Icon name="lock" size={13} />
            {readOnlyReason}
          </p>
        )}
        <div className="kb-notes-editor__fields">
          <Field label={t('editor.title')} error={problems.title && text.title ? t(problems.title.key, problems.title.values) : undefined} required>
            <TextInput
              value={text.title}
              onChange={(title) => edit({ title })}
              disabled={readOnlyReason !== null}
              placeholder={t('editor.titlePlaceholder')}
              autoFocus={!note}
            />
          </Field>
          <Field label={t('editor.tags')} help={t('editor.tagsHelp')} error={problems.tags && t(problems.tags.key, problems.tags.values)}>
            <TextInput value={text.tags} onChange={(tags) => edit({ tags })} disabled={readOnlyReason !== null} monospace placeholder="runbook, incident" />
          </Field>
          {!note && (
            <Field label={t('editor.visibility')} help={visibility === 'shared' ? undefined : (access.shareDenied ?? undefined)}>
              <Select value={visibility} options={visibilityOptions} onChange={(next) => update({ ...session, visibility: next })} />
            </Field>
          )}
        </div>
        {!note && visibility === 'personal' && !resource && (
          <Checkbox
            checked={session.global}
            onChange={(global) => update({ ...session, global })}
            label={t('editor.global')}
            description={t('editor.globalHelp')}
          />
        )}
        {visibility === 'shared' && (
          <p className="kb-notes-notice kb-notes-notice--warning">
            <Icon name="warning" size={13} />
            {t('editor.sharedWarning', { cluster: note ? note.clusterName : (cluster?.name ?? '') })}
          </p>
        )}
        {session.conflict && (
          <div className="kb-notes-conflict" role="alert">
            <Icon name="warning" size={14} />
            <span>{t('editor.conflict')}</span>
            <Button variant="secondary" disabled={busy} onClick={reload}>
              {t('editor.reload')}
            </Button>
            <Button variant="danger" disabled={busy} onClick={overwrite}>
              {t('editor.overwrite')}
            </Button>
          </div>
        )}
        {problems.body && <ErrorNotice error={new Error(t(problems.body.key, problems.body.values))} />}
        <ErrorNotice error={error} />
      </div>

      <div className={`kb-notes-editor__work kb-notes-editor__work--${view}`}>
        {view !== 'preview' && (
          <CodeEditor
            value={text.body}
            onChange={(body) => edit({ body })}
            language="markdown"
            readOnly={readOnlyReason !== null}
            wordWrap
            lineNumbers={false}
            ariaLabel={t('editor.body')}
            placeholder={t('editor.bodyPlaceholder')}
          />
        )}
        {view !== 'edit' && (
          <div className="kb-notes-preview" aria-label={t('editor.preview')}>
            {text.body.trim() ? <Markdown source={text.body} headingOffset={1} /> : <p className="kb-notes-muted">{t('editor.emptyPreview')}</p>}
          </div>
        )}
      </div>

      <DeleteNoteDialog
        note={deleting && note ? note : null}
        onClose={() => setDeleting(false)}
        onDeleted={() => {
          setTabDirty(tabId, false)
          useLayoutStore.getState().closeTab(tabId)
        }}
      />
    </div>
  )
}
