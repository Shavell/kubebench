import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { useDataMode } from '@/data/mode'
import { prefsOwner } from './prefsOwner'
import { useSettingsStore } from './settingsStore'

/**
 * Interface scale and fonts, kept in this browser for each signed-in user
 * (like the command aliases). Font families are CSS `font-family` lists;
 * `''` means the theme's monospace stack (`--font-mono`).
 */
export interface DisplaySettings {
  /** Zoom of the whole interface, in percent. */
  uiScale: number
  editorFontFamily: string
  /** Editor (Monaco) font size, in px. */
  editorFontSize: number
  terminalFontFamily: string
  /** Terminal, exec and log view font size, in px. */
  terminalFontSize: number
}

export const UI_SCALES = [90, 100, 110, 120, 130] as const
export const EDITOR_FONT_SIZES = [11, 12, 13, 14, 15, 16, 18, 20] as const
export const TERMINAL_FONT_SIZES = [11, 12, 12.5, 13, 14, 15, 16, 18] as const

const DEFAULTS: Omit<DisplaySettings, 'terminalFontSize'> = {
  uiScale: 100,
  editorFontFamily: '',
  editorFontSize: 13,
  terminalFontFamily: '',
}

interface DisplayState {
  byUser: Record<string, Partial<DisplaySettings>>
  update: (owner: string, patch: Partial<DisplaySettings>) => void
}

export const useDisplayStore = create<DisplayState>()(
  persist(
    (set) => ({
      byUser: {},
      update: (owner, patch) => set((s) => ({ byUser: { ...s.byUser, [owner]: { ...s.byUser[owner], ...patch } } })),
    }),
    { name: 'kubebench.display' },
  ),
)

const NONE: Partial<DisplaySettings> = {}

/**
 * The signed-in user's display settings and a setter for one of them. The
 * terminal font size defaults to the browser-wide one of earlier versions,
 * so an existing choice carries over.
 */
export function useDisplaySettings(): DisplaySettings & {
  set: <K extends keyof DisplaySettings>(key: K, value: DisplaySettings[K]) => void
} {
  const owner = prefsOwner(useDataMode())
  const stored = useDisplayStore((s) => s.byUser[owner]) ?? NONE
  const update = useDisplayStore((s) => s.update)
  const legacyTerminalFontSize = useSettingsStore((s) => s.terminalFontSize)
  return {
    ...DEFAULTS,
    terminalFontSize: legacyTerminalFontSize,
    ...stored,
    set: (key, value) => update(owner, { [key]: value }),
  }
}
