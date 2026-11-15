import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { dataMode, useDataMode } from '@/data/mode'
import type { SessionPermissions } from '@/data/types'
import { i18n } from '@/i18n'
import { can as canWith, deniedCode, deniedReasonKey, type DeniedCode, type Permission } from './permissionRules'

/**
 * Effective permissions of the signed-in user on the active cluster
 * (docs/design-v0.5.md §3). Boot stores them in `dataMode`: live mode
 * takes them from /api/info, mock mode acts as an admin on the active mock
 * cluster (a read-only mock cluster takes write/exec away).
 *
 * The hooks re-render when the session facts are re-read; the plain
 * functions read the current facts.
 */
export type { Permission }

/** Before boot has stored a session nothing restricts the UI (the server still decides). */
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

export function usePermissions(): SessionPermissions {
  useDataMode()
  return currentPermissions()
}

export function can(permission: Permission, perms: SessionPermissions = currentPermissions()): boolean {
  return canWith(permission, perms)
}

/** Why `permission` is not held (translated, for tooltips), or null when it is. */
export function deniedReason(
  permission: Permission,
  perms: SessionPermissions = currentPermissions(),
): string | null {
  const key = deniedReasonKey(permission, perms)
  return key ? i18n.t(key, { ns: PLATFORM_NS }) : null
}

/** A permission held, or denied with the reason and the matching error code. */
export type PermissionState =
  | { allowed: true; reason: null; code: null }
  | { allowed: false; reason: string; code: DeniedCode }

/**
 * Whether a permission is held and, if not, the reason to show on the
 * disabled control and the error code that selects the explanation of
 * `<Unavailable>`. Re-renders on language and session changes.
 */
export function usePermission(permission: Permission): PermissionState {
  const { t } = useTranslation(PLATFORM_NS)
  const perms = usePermissions()
  const key = deniedReasonKey(permission, perms)
  const code = deniedCode(permission, perms)
  return key === null || code === null ? { allowed: true, reason: null, code: null } : { allowed: false, reason: t(key), code }
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
  const perms = usePermissions()
  return can(perm, perms) ? children : fallback
}
