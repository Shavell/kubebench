import clsx from 'clsx'
import { useDeferredValue } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { Button } from '@/shared/Button'
import { Checkbox, Select, TextInput } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { SidebarErrors } from '@/shared/SidebarErrors'
import type { Note, NoteQuery } from './data/types'
import { useNotesQuery } from './data/useNotesQuery'
import { useNotesTranslation } from './i18n'
import { useNoteAccess } from './model/access'
import { useNotesFilter } from './model/filterStore'
import { objectLabel } from './model/rules'
import { noteTabId, openNewNote, openNote, VISIBILITY_ICONS } from './tabs'
import './notes.css'

/** The Notes sidebar: new note, search and filters, tags, and the list. */
export function NotesSidebar() {
  const { t } = useNotesTranslation()
  const access = useNoteAccess()
  const filter = useNotesFilter()
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const search = useDeferredValue(filter.search)

  const query: NoteQuery = {
    scope: filter.scope,
    clusters: filter.allClusters ? 'all' : 'current',
    tag: filter.tag || undefined,
    q: search || undefined,
  }
  const notes = useNotesQuery((p) => p.list(query), [filter.scope, filter.allClusters, filter.tag, search])
  const tags = useNotesQuery((p) => p.tags({ scope: filter.scope }), [filter.scope])

  const scopeOptions = (['all', 'personal', 'shared'] as const).map((scope) => ({ value: scope, label: t(`filters.scope.${scope}`) }))

  return (
    <div className="kb-notes-sidebar">
      <div className="kb-notes-sidebar__controls">
        <div className="kb-notes-sidebar__new">
          <Button icon="add" onClick={() => openNewNote({ visibility: 'personal' })}>
            {t('actions.newPersonal')}
          </Button>
          <Button
            variant="secondary"
            icon="organization"
            disabled={access.shareDenied !== null}
            title={access.shareDenied ?? t('actions.newShared')}
            aria-label={t('actions.newShared')}
            onClick={() => openNewNote({ visibility: 'shared' })}
          />
        </div>
        <TextInput
          type="search"
          value={filter.search}
          onChange={(value) => filter.set({ search: value })}
          placeholder={t('filters.search')}
          aria-label={t('filters.search')}
        />
        <Select
          value={filter.scope}
          options={scopeOptions}
          onChange={(scope) => filter.set({ scope })}
          aria-label={t('filters.scope.label')}
        />
        <Checkbox
          checked={filter.allClusters}
          onChange={(allClusters) => filter.set({ allClusters })}
          label={t('filters.allClusters')}
        />
        {(tags.data?.length ?? 0) > 0 && (
          <div className="kb-notes-sidebar__tags" role="group" aria-label={t('filters.tags')}>
            {tags.data!.map(({ tag, count }) => (
              <button
                key={tag}
                type="button"
                className={clsx('kb-notes-tag', 'kb-notes-tag--button', filter.tag === tag && 'kb-notes-tag--active')}
                aria-pressed={filter.tag === tag}
                onClick={() => filter.set({ tag: filter.tag === tag ? '' : tag })}
              >
                #{tag} <span className="kb-notes-tag__count">{count}</span>
              </button>
            ))}
          </div>
        )}
      </div>

      <ul className="kb-notes-list" aria-label={t('manifest.sidebarTitle')}>
        {(notes.data?.items ?? []).map((note) => (
          <NoteRow key={note.id} note={note} selected={activeTabId === noteTabId(note.id)} />
        ))}
      </ul>
      {notes.data && notes.data.items.length === 0 && (
        <p className="kb-notes-sidebar__empty">{filter.search || filter.tag ? t('sidebar.noMatches') : t('sidebar.empty')}</p>
      )}
      {notes.data?.truncated && <p className="kb-notes-sidebar__empty">{t('sidebar.truncated')}</p>}
      <SidebarErrors errors={[notes.error, tags.error]} />
    </div>
  )
}

function NoteRow({ note, selected }: { note: Note; selected: boolean }) {
  const { t } = useNotesTranslation()
  const place = note.resource
    ? objectLabel(note.resource)
    : note.clusterId === null
      ? t('placement.global')
      : note.clusterName
  return (
    <li>
      <button
        type="button"
        className={clsx('kb-notes-row', selected && 'kb-notes-row--selected')}
        onClick={() => openNote(note)}
        title={t(`visibility.${note.visibility}`)}
      >
        <Icon name={VISIBILITY_ICONS[note.visibility]} size={14} className="kb-notes-row__icon" />
        <span className="kb-notes-row__text">
          <span className="kb-notes-row__title">
            {note.title}
            {note.pinned && <Icon name="pinned" size={12} className="kb-notes-row__pin" />}
          </span>
          <span className="kb-notes-row__place">
            {place}
            {note.resource && note.clusterId !== null && note.clusterName && ` · ${note.clusterName}`}
          </span>
          {note.tags.length > 0 && <span className="kb-notes-row__tags">{note.tags.map((tag) => `#${tag}`).join(' ')}</span>}
        </span>
      </button>
    </li>
  )
}
