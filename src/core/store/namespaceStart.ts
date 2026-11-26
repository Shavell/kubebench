import { create } from 'zustand'
import { persist } from 'zustand/middleware'
import { dataMode, type DataMode } from '@/data/mode'
import { useContextStore } from './contextStore'
import { startNamespace } from './namespaceOrder'
import { namespaceScope } from './namespacePrefsStore'

/**
 * The namespace filter a session starts with (design §7.16): the one the
 * user last chose on the cluster, else the cluster's default namespace
 * (the kubeconfig context's, or one the credential may use when it cannot
 * list namespaces), else all namespaces. The last choice is kept in this
 * browser per user and cluster.
 */
interface LastNamespaceState {
  byScope: Record<string, string>
  remember: (scope: string, namespace: string) => void
}

export const useLastNamespaceStore = create<LastNamespaceState>()(
  persist(
    (set) => ({
      byScope: {},
      remember: (scope, namespace) => set((s) => ({ byScope: { ...s.byScope, [scope]: namespace } })),
    }),
    { name: 'kubebench.lastNamespace' },
  ),
)


let unsubscribe: (() => void) | null = null

/**
 * Sets the namespace filter for the booted session and remembers later
 * choices. Boot calls it once `dataMode` is set.
 */
export function initNamespace(mode: DataMode = dataMode): void {
  const scope = namespaceScope(mode)
  const remembered = useLastNamespaceStore.getState().byScope[scope]
  useContextStore.getState().setNamespace(startNamespace(mode.capabilities?.defaultNamespace, remembered))
  unsubscribe?.()
  unsubscribe = useContextStore.subscribe((state, previous) => {
    if (state.namespace !== previous.namespace) useLastNamespaceStore.getState().remember(scope, state.namespace)
  })
}
