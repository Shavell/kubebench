import { create } from 'zustand'

/** All namespaces (no filter). */
export const ALL_NAMESPACES = ''

/**
 * The namespace filter shared by list views (set from the status bar).
 * The active cluster is part of the session (`dataMode.cluster`), not of
 * this store: switching clusters goes through the server and reloads.
 */
interface NamespaceFilterState {
  namespace: string
  setNamespace: (namespace: string) => void
}

export const useContextStore = create<NamespaceFilterState>((set) => ({
  namespace: ALL_NAMESPACES,
  setNamespace: (namespace) => set({ namespace }),
}))
