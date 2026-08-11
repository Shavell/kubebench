import { create } from 'zustand'
import { persist } from 'zustand/middleware'

/** User preferences, persisted to localStorage. */
interface SettingsState {
  /** Metrics history window shown on charts, in minutes. */
  metricsWindow: number
  /** Font size for terminal/log views, in px. */
  terminalFontSize: number
  /** UI theme; only Dark+ exists today. */
  theme: 'dark'
  /** Refetch interval for list pages, in seconds (0 = off). */
  autoRefreshSeconds: number
  set: <K extends keyof SettingsState>(key: K, value: SettingsState[K]) => void
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      metricsWindow: 60,
      terminalFontSize: 12.5,
      theme: 'dark',
      autoRefreshSeconds: 0,
      set: (key, value) => set({ [key]: value }),
    }),
    { name: 'kubebench.settings' },
  ),
)
