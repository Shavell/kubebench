import type { ActiveCluster, GlobalRole, SessionPermissions } from './types'

export type { ActiveCluster, ClusterEnvironment, ClusterRole, SessionPermissions } from './types'

/**
 * Which data source the app was bootstrapped with, plus the session facts
 * reported by `/api/info` (docs/design-v0.5.md §4.1). Set once at boot
 * before render; the app reloads after a cluster switch, so components may
 * read it directly.
 */

/** What the backend can do on the active cluster (RBAC + probes). */
export interface LiveCapabilities {
  crds: boolean
  helm: boolean
  exec: boolean
}

export interface ServerFeatures {
  /** Clusters may be added by a server-side kubeconfig path. */
  pathKubeconfig: boolean
  /** External (runtime-loaded) UI plugins are supported. */
  externalPlugins: boolean
}

export interface DataMode {
  mode: 'live' | 'mock'
  context: string
  server?: string
  user?: string
  role?: GlobalRole
  /** Server version; the bundled `__APP_VERSION__` is used when absent. */
  version?: string
  commit?: string
  cluster?: ActiveCluster
  /** Missing in mock mode → everything allowed. */
  permissions?: SessionPermissions
  capabilities?: LiveCapabilities
  features?: ServerFeatures
  mustChangePassword?: boolean
  /** Why no cluster runtime is available (no assignment, bad kubeconfig, …). */
  clusterError?: string
}

export const dataMode: DataMode = { mode: 'mock', context: 'kind-dev-cluster (mock)' }

export function setDataMode(next: DataMode): void {
  Object.assign(dataMode, next)
}

/** Version shown in the UI: the server's in live mode, the bundle's otherwise. */
export function appVersion(): string {
  return dataMode.version || __APP_VERSION__
}
