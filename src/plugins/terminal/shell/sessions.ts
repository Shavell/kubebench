import { create } from 'zustand'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { ClusterProvider } from '@/data/providers'
import type { TerminalTarget } from '@/data/streams'
import { TerminalSession } from './TerminalSession'

/**
 * The open terminal sessions. Panel sessions are the sub-tabs of the
 * bottom panel's Terminal tab; every `pod-exec` editor tab owns one more,
 * which ends when the tab closes. Sessions are kept here, outside React,
 * so hidden views do not end them.
 */
interface PanelSessionsState {
  /** Panel sessions, in sub-tab order. */
  ids: string[]
  activeId: string | null
  /** `seq` of the last `pod-exec` panel request that became a session. */
  handledRequest: number
}

export const usePanelSessions = create<PanelSessionsState>(() => ({
  ids: [],
  activeId: null,
  handledRequest: 0,
}))

const sessions = new Map<string, TerminalSession>()
let nextPanelId = 1

export function getSession(id: string): TerminalSession | undefined {
  return sessions.get(id)
}

/** Opens a session in a new panel sub-tab and makes it the active one. */
export function openPanelSession(target: TerminalTarget, provider: ClusterProvider): string {
  const id = `panel-${nextPanelId++}`
  sessions.set(id, new TerminalSession(id, target, provider))
  usePanelSessions.setState((s) => ({ ids: [...s.ids, id], activeId: id }))
  return id
}

export function activatePanelSession(id: string): void {
  usePanelSessions.setState({ activeId: id })
}

/** Ends a panel session; its neighbour becomes active. */
export function closePanelSession(id: string): void {
  sessions.get(id)?.dispose()
  sessions.delete(id)
  usePanelSessions.setState((s) => {
    const index = s.ids.indexOf(id)
    const ids = s.ids.filter((other) => other !== id)
    const activeId = s.activeId === id ? (ids[index] ?? ids[index - 1] ?? null) : s.activeId
    return { ids, activeId }
  })
}

const tabSessionId = (tabId: string) => `tab:${tabId}`

/** The session of a `pod-exec` editor tab, created on first use. */
export function tabSession(tabId: string, target: TerminalTarget, provider: ClusterProvider): TerminalSession {
  const id = tabSessionId(tabId)
  let session = sessions.get(id)
  if (!session) {
    session = new TerminalSession(id, target, provider)
    sessions.set(id, session)
  }
  return session
}

// Editor tabs unmount whenever another tab is shown; their sessions end
// only when the tab itself is closed.
useLayoutStore.subscribe((state) => {
  const open = new Set(state.tabs.map((tab) => tabSessionId(tab.id)))
  for (const [id, session] of sessions) {
    if (id.startsWith('tab:') && !open.has(id)) {
      session.dispose()
      sessions.delete(id)
    }
  }
})
