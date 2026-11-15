import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { dataMode, useDataMode, type DataMode } from '@/data/mode'
import { EMPTY_PREFS, orderNamespaces, toggled, type NamespacePrefs, type OrderedNamespaces } from './namespaceOrder'

/**
 * Favourite (pinned) and hidden namespaces, persisted in this browser per
 * user and cluster: a namespace pinned on one cluster says nothing about
 * another cluster's namespace of the same name.
 */
interface NamespacePrefsState {
  byScope: Record<string, NamespacePrefs>
  togglePinned: (scope: string, namespace: string) => void
  toggleHidden: (scope: string, namespace: string) => void
}

export const useNamespacePrefsStore = create<NamespacePrefsState>()(
  persist(
    (set) => ({
      byScope: {},
      // Pinned and hidden exclude each other: pinning shows, hiding unpins.
      togglePinned: (scope, namespace) =>
        set((s) => {
          const prefs = s.byScope[scope] ?? EMPTY_PREFS
          const pinned = toggled(prefs.pinned, namespace)
          const hidden = prefs.hidden.filter((n) => n !== namespace)
          return { byScope: { ...s.byScope, [scope]: { pinned, hidden } } }
        }),
      toggleHidden: (scope, namespace) =>
        set((s) => {
          const prefs = s.byScope[scope] ?? EMPTY_PREFS
          const hidden = toggled(prefs.hidden, namespace)
          const pinned = prefs.pinned.filter((n) => n !== namespace)
          return { byScope: { ...s.byScope, [scope]: { pinned, hidden } } }
        }),
    }),
    { name: 'kubebench.namespaces' },
  ),
)

/** The preference scope of a session: data mode, user and active cluster. */
export function namespaceScope(mode: DataMode = dataMode): string {
  return `${mode.mode}:${mode.user ?? ''}@${mode.cluster?.name ?? mode.context}`
}

/** The active cluster's namespace preferences, with toggles bound to it. */
export function useNamespacePrefs(): NamespacePrefs & {
  scope: string
  togglePinned: (namespace: string) => void
  toggleHidden: (namespace: string) => void
} {
  const scope = namespaceScope(useDataMode())
  const prefs = useNamespacePrefsStore((s) => s.byScope[scope]) ?? EMPTY_PREFS
  const togglePinned = useNamespacePrefsStore((s) => s.togglePinned)
  const toggleHidden = useNamespacePrefsStore((s) => s.toggleHidden)
  return {
    ...prefs,
    scope,
    togglePinned: (namespace) => togglePinned(scope, namespace),
    toggleHidden: (namespace) => toggleHidden(scope, namespace),
  }
}

/**
 * Namespaces for a picker of the active cluster, favourites first and
 * hidden ones left out (`selected` always stays, so a filter in use does
 * not vanish from its own select).
 */
export function useNamespaceChoices(names: readonly string[], selected?: string): OrderedNamespaces {
  const prefs = useNamespacePrefs()
  return orderNamespaces(names, prefs, { keep: selected })
}
