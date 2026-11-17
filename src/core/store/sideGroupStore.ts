import { create } from 'zustand'
import type { TabDescriptor } from '@/core/plugin/types'

/**
 * The side editor group (design §7.14): a second group of tabs to the
 * right of the editor area, like VS Code's split editor, for views that
 * stay open next to the page being worked on (the service browser).
 * Tabs are the same descriptors as editor tabs and are rendered by the
 * same tab renderers; plugins open them with `openInSideGroup()`
 * (core/plugin/sideGroup.ts). Unlike the editor area, the group keeps its
 * inactive tabs mounted, so switching tabs does not reload a page.
 */
interface SideGroupState {
  tabs: TabDescriptor[]
  activeTabId: string | null
  /** Width in px; clamped to SIDE_MIN..SIDE_MAX. */
  width: number
  open: (tab: TabDescriptor) => void
  close: (id: string) => void
  closeAll: () => void
  setActive: (id: string) => void
  setWidth: (width: number) => void
}

export const SIDE_MIN = 280
export const SIDE_MAX = 1600

export const useSideGroupStore = create<SideGroupState>((set) => ({
  tabs: [],
  activeTabId: null,
  width: 560,

  open: (tab) =>
    set((s) =>
      s.tabs.some((t) => t.id === tab.id) ? { activeTabId: tab.id } : { tabs: [...s.tabs, tab], activeTabId: tab.id },
    ),

  close: (id) =>
    set((s) => {
      const index = s.tabs.findIndex((t) => t.id === id)
      const tabs = s.tabs.filter((t) => t.id !== id)
      let activeTabId = s.activeTabId
      if (activeTabId === id) activeTabId = (tabs[index] ?? tabs[index - 1])?.id ?? null
      return { tabs, activeTabId }
    }),

  closeAll: () => set({ tabs: [], activeTabId: null }),

  setActive: (activeTabId) => set({ activeTabId }),

  setWidth: (width) => set({ width: Math.min(SIDE_MAX, Math.max(SIDE_MIN, width)) }),
}))
