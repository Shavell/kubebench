import { create } from 'zustand'

/** All namespaces (no filter). */
export const ALL_NAMESPACES = ''

interface ClusterContextState {
  contexts: string[]
  activeContext: string
  namespace: string
  setActiveContext: (context: string) => void
  setNamespace: (namespace: string) => void
}

export const useContextStore = create<ClusterContextState>((set) => ({
  contexts: ['kind-dev-cluster', 'prod-eu-west-1', 'staging'],
  activeContext: 'kind-dev-cluster',
  namespace: ALL_NAMESPACES,
  setActiveContext: (activeContext) => set({ activeContext }),
  setNamespace: (namespace) => set({ namespace }),
}))
