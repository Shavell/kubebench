/**
 * The pure rules behind src/core/permissions.ts: whether a permission is
 * held and how a denial is explained (design §3). Free of imports, so
 * that `node --test` can load it.
 */
/**
 * `browse` opens the service browser (§7.14): the operator or admin role,
 * also on a read-only cluster (where it sends only GET and HEAD).
 */
export type Permission = 'read' | 'write' | 'exec' | 'admin' | 'browse'

/** The session facts the rules read (a SessionPermissions satisfies it). */
export interface PermissionFacts {
  role: string
  canRead: boolean
  canWrite: boolean
  canExec: boolean
  isAdmin: boolean
  readOnly: boolean
}

/** The server's error codes for a denial (§3.3). */
export type DeniedCode = 'admin_required' | 'cluster_read_only' | 'forbidden_role'

export function can(permission: Permission, perms: PermissionFacts): boolean {
  switch (permission) {
    case 'read':
      return perms.canRead
    case 'write':
      return perms.canWrite
    case 'exec':
      return perms.canExec
    case 'admin':
      return perms.isAdmin
    case 'browse':
      return perms.canRead && (perms.role === 'admin' || perms.role === 'operator')
  }
}

/** i18n key (platform namespace) explaining why `permission` is not held, or null when it is. */
export function deniedReasonKey(permission: Permission, perms: PermissionFacts): string | null {
  if (can(permission, perms)) return null
  if (permission === 'admin') return 'permissions.adminRequired'
  if (permission === 'browse') return perms.role === 'viewer' ? 'permissions.browseViewer' : 'permissions.notAllowed'
  if (permission !== 'read' && perms.readOnly) return 'permissions.clusterReadOnly'
  if (perms.role === 'viewer') return 'permissions.viewerRole'
  return 'permissions.notAllowed'
}

/**
 * The error code the server would answer a denied `permission` with, for
 * `<Unavailable>` (its title), or null when it is held: a read-only
 * cluster blocks write and exec for everyone, whatever the role.
 */
export function deniedCode(permission: Permission, perms: PermissionFacts): DeniedCode | null {
  if (can(permission, perms)) return null
  if (permission === 'admin') return 'admin_required'
  if (permission === 'browse') return 'forbidden_role'
  if (permission !== 'read' && perms.readOnly) return 'cluster_read_only'
  return 'forbidden_role'
}
