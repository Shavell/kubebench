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
  /** Where utilization series come from in live mode. */
  metricsSource: 'auto' | 'metrics-server' | 'prometheus'
  /**
   * UI language preference: a supported code ('en', 'tr', …) or 'auto'
   * (follow the browser). Typed as string to keep this store free of
   * i18n imports; src/i18n validates it.
   */
  language: 'auto' | (string & {})
  set: <K extends keyof SettingsState>(key: K, value: SettingsState[K]) => void
}

export const useSettingsStore = create<SettingsState>()(
  persist(
    (set) => ({
      metricsWindow: 60,
      terminalFontSize: 12.5,
      theme: 'dark',
      autoRefreshSeconds: 0,
      metricsSource: 'auto',
      language: 'auto',
      set: (key, value) => set({ [key]: value }),
    }),
    { name: 'kubebench.settings' },
  ),
)
