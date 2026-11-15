import { create } from 'zustand'
import type { TabDescriptor } from '@/core/plugin/types'

/**
 * A close of tabs with unsaved changes, waiting for the user to confirm
 * that the changes may be discarded (TabCloseGuard asks).
 */
export interface PendingTabClose {
  /** The dirty tabs the close would discard. */
  tabIds: string[]
  /** Performs the close once confirmed. */
  close: () => void
}

interface LayoutState {
  activePluginId: string
  sidebarCollapsed: boolean
  sidebarWidth: number
  panelVisible: boolean
  panelHeight: number
  panelMaximized: boolean
  activePanelTabId: string
  tabs: TabDescriptor[]
  activeTabId: string | null
  paletteOpen: boolean

  setActivePlugin: (id: string) => void
  setPaletteOpen: (open: boolean) => void
  setSidebarWidth: (width: number) => void
  setPanelHeight: (height: number) => void
  togglePanel: () => void
  togglePanelMaximized: () => void
  setActivePanelTab: (id: string) => void
  openTab: (tab: TabDescriptor) => void
  closeTab: (id: string) => void
  closeOtherTabs: (id: string) => void
  closeTabsToRight: (id: string) => void
  closeAllTabs: () => void
  setActiveTab: (id: string) => void
  /** Set while a close of dirty tabs waits for confirmation. */
  pendingClose: PendingTabClose | null
  /** Marks a tab as having unsaved changes, or no longer. */
  setTabDirty: (id: string, dirty: boolean) => void
  /** Confirms (discarding the tabs' changes) or cancels the pending close. */
  resolvePendingClose: (discard: boolean) => void
}

const SIDEBAR_MIN = 170
const SIDEBAR_MAX = 520
const PANEL_MIN = 90
const PANEL_MAX = 600

export const useLayoutStore = create<LayoutState>((set) => ({
  activePluginId: 'workloads',
  sidebarCollapsed: false,
  sidebarWidth: 260,
  panelVisible: true,
  panelHeight: 200,
  panelMaximized: false,
  activePanelTabId: 'terminal',
  tabs: [],
  activeTabId: null,
  paletteOpen: false,

  setPaletteOpen: (paletteOpen) => set({ paletteOpen }),

  setActivePlugin: (id) =>
    set((s) =>
      // Clicking the active icon toggles the sidebar, like VSCode.
      s.activePluginId === id
        ? { sidebarCollapsed: !s.sidebarCollapsed }
        : { activePluginId: id, sidebarCollapsed: false },
    ),

  setSidebarWidth: (width) =>
    set({ sidebarWidth: Math.min(SIDEBAR_MAX, Math.max(SIDEBAR_MIN, width)) }),

  setPanelHeight: (height) =>
    set({
      panelHeight: Math.min(PANEL_MAX, Math.max(PANEL_MIN, height)),
      panelMaximized: false,
    }),

  togglePanel: () => set((s) => ({ panelVisible: !s.panelVisible, panelMaximized: false })),

  togglePanelMaximized: () => set((s) => ({ panelMaximized: !s.panelMaximized })),

  setActivePanelTab: (id) => set({ activePanelTabId: id, panelVisible: true }),

  openTab: (tab) =>
    set((s) =>
      s.tabs.some((t) => t.id === tab.id)
        ? { activeTabId: tab.id }
        : { tabs: [...s.tabs, tab], activeTabId: tab.id },
    ),

  closeTab: (id) =>
    guardClose((tabs) => tabs.filter((t) => t.id === id), () =>
      set((s) => {
        const index = s.tabs.findIndex((t) => t.id === id)
        const tabs = s.tabs.filter((t) => t.id !== id)
        let activeTabId = s.activeTabId
        if (s.activeTabId === id) {
          const neighbor = tabs[index] ?? tabs[index - 1]
          activeTabId = neighbor?.id ?? null
        }
        return { tabs, activeTabId }
      }),
    ),

  closeOtherTabs: (id) =>
    guardClose((tabs) => tabs.filter((t) => t.id !== id), () =>
      set((s) => ({
        tabs: s.tabs.filter((t) => t.id === id),
        activeTabId: s.tabs.some((t) => t.id === id) ? id : null,
      })),
    ),

  closeTabsToRight: (id) =>
    guardClose((tabs) => tabsRightOf(tabs, id), () =>
      set((s) => {
        const index = s.tabs.findIndex((t) => t.id === id)
        if (index === -1) return s
        const tabs = s.tabs.slice(0, index + 1)
        const activeTabId = tabs.some((t) => t.id === s.activeTabId) ? s.activeTabId : id
        return { tabs, activeTabId }
      }),
    ),

  closeAllTabs: () => guardClose((tabs) => tabs, () => set({ tabs: [], activeTabId: null })),

  setActiveTab: (id) => set({ activeTabId: id }),

  pendingClose: null,

  setTabDirty: (id, dirty) =>
    set((s) => ({ tabs: s.tabs.map((t) => (t.id === id && Boolean(t.dirty) !== dirty ? { ...t, dirty } : t)) })),

  resolvePendingClose: (discard) => {
    const pending = useLayoutStore.getState().pendingClose
    set({ pendingClose: null })
    if (!discard || !pending) return
    set((s) => ({ tabs: s.tabs.map((t) => (pending.tabIds.includes(t.id) ? { ...t, dirty: false } : t)) }))
    pending.close()
  },
}))

function tabsRightOf(tabs: TabDescriptor[], id: string): TabDescriptor[] {
  const index = tabs.findIndex((t) => t.id === id)
  return index === -1 ? [] : tabs.slice(index + 1)
}

/**
 * Runs `close` unless the tabs it closes (`closing` picks them) have
 * unsaved changes: then the close waits in `pendingClose` until the user
 * confirms or cancels it.
 */
function guardClose(closing: (tabs: TabDescriptor[]) => TabDescriptor[], close: () => void): void {
  const dirty = closing(useLayoutStore.getState().tabs).filter((t) => t.dirty)
  if (dirty.length === 0) close()
  else useLayoutStore.setState({ pendingClose: { tabIds: dirty.map((t) => t.id), close } })
}
