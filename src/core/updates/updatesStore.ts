import { create } from 'zustand'
import type { UpdatesProvider } from '@/data/providers'
import type { UpdateMode, UpdateStatus } from '@/data/types'
import type { ChangelogRelease } from './changelog'

/** The once-per-version What's new dialog after an upgrade. */
export interface WhatsNewRequest {
  version: string
  /** The version the user saw before; empty for a first look. */
  from: string
  releases: ChangelogRelease[]
}

export type UpdatesDialog = { kind: 'available' } | ({ kind: 'whatsNew' } & WhatsNewRequest)

interface UpdatesState {
  /** The server's latest check; null until loaded. */
  status: UpdateStatus | null
  /** Why the last load failed, when it did. */
  loadError: unknown
  /** A forced check or a mode change is running. */
  busy: boolean
  dialog: UpdatesDialog | null
  /** Re-reads the status (the server checks first when its cache expired). */
  load: (provider: UpdatesProvider) => Promise<void>
  /** Admin: checks now; rejects on failure. */
  check: (provider: UpdatesProvider) => Promise<UpdateStatus>
  /** Admin: stores the mode; rejects on failure. */
  setMode: (provider: UpdatesProvider, mode: UpdateMode) => Promise<UpdateStatus>
  showAvailable: () => void
  showWhatsNew: (request: WhatsNewRequest) => void
  closeDialog: () => void
}

/**
 * Update status shared by the status bar item, the Settings section, the
 * About dialog and the commands, plus which updates dialog is open.
 */
export const useUpdatesStore = create<UpdatesState>((set) => {
  const busy = async (run: () => Promise<UpdateStatus>) => {
    set({ busy: true })
    try {
      const status = await run()
      set({ status, loadError: null })
      return status
    } finally {
      set({ busy: false })
    }
  }
  return {
    status: null,
    loadError: null,
    busy: false,
    dialog: null,
    load: async (provider) => {
      try {
        set({ status: await provider.status(), loadError: null })
      } catch (err) {
        set({ loadError: err })
      }
    },
    check: (provider) => busy(() => provider.checkNow()),
    setMode: (provider, mode) => busy(() => provider.setMode(mode)),
    showAvailable: () => set({ dialog: { kind: 'available' } }),
    showWhatsNew: (request) => set({ dialog: { kind: 'whatsNew', ...request } }),
    closeDialog: () => set({ dialog: null }),
  }
})
