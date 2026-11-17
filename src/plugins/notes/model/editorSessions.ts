import { create } from 'zustand'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { NoteVisibility } from '../data/types'

/** How the editor shows a note: the Markdown source, its rendering, or both. */
export type EditorView = 'edit' | 'preview' | 'split'

/** The text fields of a note as the editor holds them. */
export interface EditorText {
  title: string
  body: string
  /** Tags as typed: comma or space separated. */
  tags: string
}

/**
 * The state of one note editor tab. It must outlive the tab's component:
 * the editor area renders only the active tab, and unsaved text has to
 * survive switching tabs. A session ends when its tab closes.
 */
export interface EditorSession {
  text: EditorText
  /** The text as last loaded or saved: equal means nothing to lose. */
  baseline: EditorText
  /** The version the edit is based on; 0 for a draft. */
  version: number
  visibility: NoteVisibility
  global: boolean
  view: EditorView
  /** The last save found a newer version on the server. */
  conflict: boolean
}

const useSessions = create<{ sessions: Partial<Record<string, EditorSession>> }>(() => ({ sessions: {} }))

useLayoutStore.subscribe((layout) => {
  const { sessions } = useSessions.getState()
  const open = new Set(layout.tabs.map((t) => t.id))
  if (Object.keys(sessions).some((id) => !open.has(id))) {
    useSessions.setState({ sessions: Object.fromEntries(Object.entries(sessions).filter(([id]) => open.has(id))) })
  }
})

/** The session of tab `tabId` (undefined until set) and its setter, which merges a patch. */
export function useEditorSession(tabId: string): [EditorSession | undefined, (patch: Partial<EditorSession>) => void] {
  const session = useSessions((s) => s.sessions[tabId])
  const update = (patch: Partial<EditorSession>) =>
    useSessions.setState((s) => {
      const current = s.sessions[tabId]
      return { sessions: { ...s.sessions, [tabId]: { ...(current as EditorSession), ...patch } } }
    })
  return [session, update]
}

export function sameText(a: EditorText, b: EditorText): boolean {
  return a.title === b.title && a.body === b.body && a.tags === b.tags
}
