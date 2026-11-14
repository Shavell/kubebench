import { useSyncExternalStore } from 'react'
import { useSettingsStore, type ThemePreference } from '@/core/store/settingsStore'

/**
 * Applies the color theme (design §6.7): resolves the Settings preference
 * (dark | light | system) to a concrete theme, sets `<html data-theme>`
 * (theme.css switches every token on it) and `color-scheme` for native
 * controls, and follows OS changes while the preference is "system".
 */
export type ResolvedTheme = 'dark' | 'light'

const lightQuery =
  typeof window !== 'undefined' && window.matchMedia
    ? window.matchMedia('(prefers-color-scheme: light)')
    : null

const listeners = new Set<() => void>()
let resolved: ResolvedTheme = 'dark'

function resolveTheme(preference: ThemePreference): ResolvedTheme {
  if (preference === 'light') return 'light'
  if (preference === 'system') return lightQuery?.matches ? 'light' : 'dark'
  return 'dark'
}

function apply(): void {
  const next = resolveTheme(useSettingsStore.getState().theme)
  const root = document.documentElement
  root.dataset.theme = next
  root.style.colorScheme = next
  if (next !== resolved) {
    resolved = next
    listeners.forEach((listener) => listener())
  }
}

let started = false

/** Applies the theme now and keeps it in sync with Settings and the OS. Idempotent. */
export function initTheme(): void {
  if (started) return
  started = true
  apply()
  useSettingsStore.subscribe((state, prev) => {
    if (state.theme !== prev.theme) apply()
  })
  lightQuery?.addEventListener('change', apply)
}

export function currentTheme(): ResolvedTheme {
  return resolved
}

/** The concrete theme in effect; re-renders when it changes (e.g. for Monaco). */
export function useResolvedTheme(): ResolvedTheme {
  return useSyncExternalStore(
    (onChange) => {
      listeners.add(onChange)
      return () => listeners.delete(onChange)
    },
    currentTheme,
  )
}
