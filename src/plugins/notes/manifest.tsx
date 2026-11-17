import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { localeBundles } from '@/i18n/bundles'
import { NoteCountBadge } from './components/NoteCountBadge'
import { NS } from './i18n'
import { NotesSidebar } from './NotesSidebar'
import { NOTE_ICON, noteObject, openNewNote, openResourceNotes, TAB_KINDS } from './tabs'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const NoteEditorTab = lazyComponent(() => import('./pages/NoteEditorTab').then((m) => m.NoteEditorTab))
const ResourceNotesTab = lazyComponent(() => import('./pages/ResourceNotesTab').then((m) => m.ResourceNotesTab))

/**
 * Notes (design §7.13): personal and shared Markdown notes on the cluster,
 * its objects, or (personal) everywhere. Display strings below are i18n
 * keys in the plugin's namespace.
 */
export const notesPlugin: PluginManifest = {
  id: NS,
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'tools',
  backendPlugin: NS,
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
  activityBar: { icon: NOTE_ICON, tooltip: 'manifest.tooltip' },
  sidebar: { id: 'notes-list', title: 'manifest.sidebarTitle', component: NotesSidebar },
  tabRenderers: [
    { kind: TAB_KINDS.editor, component: NoteEditorTab },
    { kind: TAB_KINDS.resource, component: ResourceNotesTab },
  ],
  commands: [
    { id: 'notes.new', title: 'commands.new', icon: 'add', run: () => openNewNote({ visibility: 'personal' }) },
    { id: 'notes.newShared', title: 'commands.newShared', icon: 'organization', run: () => openNewNote({ visibility: 'shared' }) },
  ],
  resourceActions: [
    {
      id: 'notes.show',
      title: 'resourceActions.show',
      icon: NOTE_ICON,
      kinds: ['*'],
      run: (ref) => openResourceNotes(noteObject(ref)),
      badge: NoteCountBadge,
    },
  ],
}
