import { create } from 'zustand'
import { useLayoutStore } from '@/core/store/layoutStore'

/**
 * State of an editor tab that must outlive its component: the editor area
 * renders only the active tab, so switching tabs unmounts the editor while
 * its unsaved text has to stay. A session ends when its tab closes.
 */
const useSessionStore = create<{ sessions: Partial<Record<string, object>> }>(() => ({ sessions: {} }))

useLayoutStore.subscribe((layout) => {
  const { sessions } = useSessionStore.getState()
  const open = new Set(layout.tabs.map((t) => t.id))
  if (Object.keys(sessions).some((id) => !open.has(id))) {
    useSessionStore.setState({ sessions: Object.fromEntries(Object.entries(sessions).filter(([id]) => open.has(id))) })
  }
})

/**
 * The session of tab `tabId`, `initial()` until the first update (so it
 * must be cheap and depend only on the tab's props), and an updater that
 * merges a patch into it.
 */
export function useTabSession<T extends object>(tabId: string, initial: () => T): [T, (patch: Partial<T>) => void] {
  const stored = useSessionStore((s) => s.sessions[tabId]) as T | undefined
  const session = stored ?? initial()
  const update = (patch: Partial<T>) =>
    useSessionStore.setState((s) => ({
      sessions: { ...s.sessions, [tabId]: { ...((s.sessions[tabId] as T | undefined) ?? session), ...patch } },
    }))
  return [session, update]
}
