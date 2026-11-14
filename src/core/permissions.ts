import type { ReactNode } from 'react'
import { dataMode, type SessionPermissions } from '@/data/mode'

/**
 * Effective permissions of the signed-in user on the active cluster
 * (docs/design-v0.5.md §3). In mock mode everything is allowed.
 *
 * `dataMode` is set once at boot and the app reloads on cluster switch,
 * so this is a plain read — no subscription needed.
 */
export type Permission = 'read' | 'write' | 'exec' | 'admin'

const ALL_ALLOWED: SessionPermissions = {
  role: 'admin',
  canRead: true,
  canWrite: true,
  canExec: true,
  isAdmin: true,
  readOnly: false,
}

export function currentPermissions(): SessionPermissions {
  return dataMode.mode === 'mock' || !dataMode.permissions ? ALL_ALLOWED : dataMode.permissions
}

/** Hook form (kept as a hook so a reactive store can back it later). */
export function usePermissions(): SessionPermissions {
  return currentPermissions()
}

export function can(permission: Permission, perms: SessionPermissions = currentPermissions()): boolean {
  switch (permission) {
    case 'read':
      return perms.canRead
    case 'write':
      return perms.canWrite
    case 'exec':
      return perms.canExec
    case 'admin':
      return perms.isAdmin
  }
}

/** Renders children only when the permission is held. */
export function Can({
  perm,
  children,
  fallback = null,
}: {
  perm: Permission
  children: ReactNode
  fallback?: ReactNode
}): ReactNode {
  return can(perm) ? children : fallback
}
