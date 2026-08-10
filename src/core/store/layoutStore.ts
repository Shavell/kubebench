import { create } from 'zustand'
import type { TabDescriptor } from '@/core/plugin/types'

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

  setActivePlugin: (id: string) => void
  setSidebarWidth: (width: number) => void
  setPanelHeight: (height: number) => void
  togglePanel: () => void
  togglePanelMaximized: () => void
  setActivePanelTab: (id: string) => void
  openTab: (tab: TabDescriptor) => void
  closeTab: (id: string) => void
  setActiveTab: (id: string) => void
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

  setActiveTab: (id) => set({ activeTabId: id }),
}))
