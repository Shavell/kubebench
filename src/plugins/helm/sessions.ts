import { useCallback, useRef } from 'react'
import { create } from 'zustand'
import { useLayoutStore } from '@/core/store/layoutStore'

/**
 * State of the install and upgrade tabs that must outlive their
 * components: the editor area renders only the active tab, so switching
 * tabs unmounts a half-filled wizard. A session ends when its tab closes.
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
 * The session of tab `tabId`: `initial()` until the first update (so it
 * must depend only on the tab's props), and a stable updater that merges a
 * patch into it.
 */
export function useTabSession<T extends object>(tabId: string, initial: () => T): [T, (patch: Partial<T>) => void] {
  const stored = useSessionStore((s) => s.sessions[tabId]) as T | undefined
  // The component may be reused for another tab of the same kind.
  const fallback = useRef<{ tabId: string; value: T } | null>(null)
  if (fallback.current?.tabId !== tabId) fallback.current = { tabId, value: initial() }
  const start = fallback.current.value
  const update = useCallback(
    (patch: Partial<T>) =>
      useSessionStore.setState((s) => ({
        sessions: { ...s.sessions, [tabId]: { ...((s.sessions[tabId] as T | undefined) ?? start), ...patch } },
      })),
    [tabId, start],
  )
  return [stored ?? start, update]
}
