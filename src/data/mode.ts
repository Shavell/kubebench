import { useSyncExternalStore } from 'react'
import type {
  ActiveCluster,
  GlobalRole,
  LiveCapabilities,
  ServerFeatures,
  ServerInfo,
  SessionPermissions,
} from './types'

export type { ActiveCluster, ClusterEnvironment, ClusterRole, SessionPermissions } from './types'

/**
 * Which data source the app was booted with, plus the session facts
 * reported by `/api/info` (docs/design-v0.5.md §4.1; mock mode builds the
 * same facts from the mock inventory).
 *
 * Boot sets it before the first render. It changes afterwards only when
 * the session is re-read (e.g. an admin edits the active cluster's
 * environment or read-only flag); a cluster switch reloads the app.
 * Components that must follow such changes read it with `useDataMode()`.
 */
export interface DataMode {
  mode: 'live' | 'mock'
  /** Name of the active cluster; empty when there is none. */
  context: string
  server?: string
  user?: string
  role?: GlobalRole
  /** Server version; the bundled `__APP_VERSION__` is used when absent. */
  version?: string
  commit?: string
  cluster?: ActiveCluster
  /** Missing before boot → everything allowed (see core/permissions). */
  permissions?: SessionPermissions
  capabilities?: LiveCapabilities
  features?: ServerFeatures
  mustChangePassword?: boolean
  /** Why no cluster runtime is available (no assignment, bad kubeconfig, …). */
  clusterError?: string
}

export const dataMode: DataMode = { mode: 'mock', context: '' }

let revision = 0
const listeners = new Set<() => void>()

/**
 * Replaces the session facts (fields missing from `next` are cleared) and
 * re-renders `useDataMode()` readers. The object identity of `dataMode`
 * stays the same, so plain readers always see the current facts.
 */
export function setDataMode(next: DataMode): void {
  for (const key of Object.keys(dataMode) as (keyof DataMode)[]) delete dataMode[key]
  Object.assign(dataMode, next)
  revision++
  listeners.forEach((listener) => listener())
}

function subscribe(listener: () => void): () => void {
  listeners.add(listener)
  return () => listeners.delete(listener)
}

/** `dataMode`, re-rendering the caller when the session facts are replaced. */
export function useDataMode(): DataMode {
  useSyncExternalStore(subscribe, () => revision)
  return dataMode
}

/** The DataMode for a session described by `/api/info` (or its mock equivalent). */
export function dataModeFromInfo(mode: DataMode['mode'], info: ServerInfo): DataMode {
  return {
    mode,
    context: info.context,
    server: info.server || undefined,
    user: info.user,
    role: info.role,
    version: info.version || undefined,
    commit: info.commit || undefined,
    cluster: info.cluster ?? undefined,
    permissions: info.permissions,
    capabilities: info.capabilities,
    features: info.features,
    mustChangePassword: info.mustChangePassword,
    clusterError: info.clusterError || undefined,
  }
}

/** Version shown in the UI: the server's in live mode, the bundle's otherwise. */
export function appVersion(): string {
  return dataMode.version || __APP_VERSION__
}
