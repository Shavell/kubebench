import { create } from 'zustand'
import { persist } from 'zustand/middleware'

/** Tail sizes offered by the log view; 0 is the whole log. */
export const TAIL_SIZES = [100, 500, 1000, 5000, 0] as const

/** How log views start and show lines; the user's last choice, kept in localStorage. */
interface LogPreferences {
  tail: number
  timestamps: boolean
  wrap: boolean
  colorize: boolean
  set: <K extends Exclude<keyof LogPreferences, 'set'>>(key: K, value: LogPreferences[K]) => void
}

export const useLogPreferences = create<LogPreferences>()(
  persist(
    (set) => ({
      tail: 500,
      timestamps: true,
      wrap: false,
      colorize: true,
      set: (key, value) => set({ [key]: value }),
    }),
    { name: 'kubebench.logs' },
  ),
)
