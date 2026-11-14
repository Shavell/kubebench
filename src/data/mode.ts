/**
 * Which data source the app was bootstrapped with, plus the session facts
 * reported by `/api/info`. Set once in main.tsx before render; the app
 * reloads after a cluster switch, so components may read it directly.
 */
export type ClusterRole = 'admin' | 'operator' | 'viewer'
export type ClusterEnvironment = '' | 'development' | 'staging' | 'production'

export interface SessionPermissions {
  /** Effective role on the active cluster. */
  role: ClusterRole
  canRead: boolean
  canWrite: boolean
  canExec: boolean
  isAdmin: boolean
  /** The active cluster is flagged read-only (blocks write/exec for everyone). */
  readOnly: boolean
}

export interface ActiveCluster {
  id: number
  name: string
  environment: ClusterEnvironment
  readOnly: boolean
}

export interface DataMode {
  mode: 'live' | 'mock'
  context: string
  server?: string
  user?: string
  role?: 'admin' | 'user'
  version?: string
  commit?: string
  cluster?: ActiveCluster
  /** Missing in mock mode → everything allowed. */
  permissions?: SessionPermissions
  features?: { pathKubeconfig?: boolean; externalPlugins?: boolean }
  mustChangePassword?: boolean
}

export const dataMode: DataMode = { mode: 'mock', context: 'kind-dev-cluster (mock)' }

export function setDataMode(next: DataMode): void {
  Object.assign(dataMode, next)
}
