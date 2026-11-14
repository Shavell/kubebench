import { useSyncExternalStore } from 'react'
import { useSettingsStore, type ThemePreference } from '@/core/store/settingsStore'

/**
 * Applies the colour theme (design §6.7). Importing this module resolves
 * the Settings preference (dark | light | system) to a concrete palette,
 * sets `<html data-theme>` (theme.css switches every token on it) and
 * `color-scheme` (native controls, scroll bars), and keeps both in sync with
 * Settings and, while the preference is "system", with the operating system.
 *
 * index.html applies the same rule before the first paint so the page never
 * flashes the wrong palette; keep resolveTheme() and that inline script in
 * agreement. Import this module once at boot (main.tsx), so that theme
 * changes apply even before any themed component has rendered.
 */
export type ResolvedTheme = 'dark' | 'light'

const lightQuery = window.matchMedia?.('(prefers-color-scheme: light)') ?? null

const listeners = new Set<() => void>()
let resolved: ResolvedTheme = 'dark'

const systemTheme = (): ResolvedTheme => (lightQuery?.matches ? 'light' : 'dark')

function resolveTheme(preference: ThemePreference): ResolvedTheme {
  if (preference === 'system') return systemTheme()
  return preference === 'light' ? 'light' : 'dark'
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

apply()
useSettingsStore.subscribe((state, prev) => {
  if (state.theme !== prev.theme) apply()
})
lightQuery?.addEventListener('change', apply)

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/** The palette in effect; re-renders when it changes (e.g. to retheme Monaco). */
export function useResolvedTheme(): ResolvedTheme {
  return useSyncExternalStore(subscribe, () => resolved)
}

function subscribeSystem(listener: () => void): () => void {
  lightQuery?.addEventListener('change', listener)
  return () => lightQuery?.removeEventListener('change', listener)
}

/** What the "system" preference resolves to right now; re-renders when the OS switches. */
export function useSystemTheme(): ResolvedTheme {
  return useSyncExternalStore(subscribeSystem, systemTheme)
}
