import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { dataMode } from '@/data/mode'
import type { SessionPermissions } from '@/data/types'
import { i18n } from '@/i18n'

/**
 * Effective permissions of the signed-in user on the active cluster
 * (docs/design-v0.5.md §3). Boot stores them in `dataMode`; mock mode
 * acts as an admin (only a read-only mock cluster takes write/exec away).
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
  return dataMode.permissions ?? ALL_ALLOWED
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

/** i18n key (platform namespace) explaining why `permission` is not held. */
function deniedReasonKey(permission: Permission, perms: SessionPermissions): string | null {
  if (can(permission, perms)) return null
  if (permission === 'admin') return 'permissions.adminRequired'
  if (permission !== 'read' && perms.readOnly) return 'permissions.clusterReadOnly'
  if (perms.role === 'viewer') return 'permissions.viewerRole'
  return 'permissions.notAllowed'
}

/** Why `permission` is not held (translated, for tooltips), or null when it is. */
export function deniedReason(
  permission: Permission,
  perms: SessionPermissions = currentPermissions(),
): string | null {
  const key = deniedReasonKey(permission, perms)
  return key ? i18n.t(key, { ns: PLATFORM_NS }) : null
}

/**
 * Whether a permission is held and, if not, the reason to show on the
 * disabled control. Re-renders on language change.
 */
export function usePermission(permission: Permission): { allowed: boolean; reason: string | null } {
  const { t } = useTranslation(PLATFORM_NS)
  const perms = usePermissions()
  const key = deniedReasonKey(permission, perms)
  return { allowed: key === null, reason: key ? t(key) : null }
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
