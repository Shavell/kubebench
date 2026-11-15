import { create } from 'zustand'

export type WorkbenchDialog = 'about' | 'changePassword' | 'shortcuts'

/**
 * Which workbench-level dialog is open. Commands, the status bar and its
 * menus open them here; <WorkbenchDialogs/> in the shell renders them.
 */
export const useWorkbenchDialog = create<{
  open: WorkbenchDialog | null
  show: (dialog: WorkbenchDialog) => void
  close: () => void
}>((set) => ({
  open: null,
  show: (open) => set({ open }),
  close: () => set({ open: null }),
}))
