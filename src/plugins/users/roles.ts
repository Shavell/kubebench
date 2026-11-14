import type { AssignmentRole, GlobalRole } from '@/data/types'

/** i18n keys (users namespace) of role names. */
export const globalRoleKey = (role: GlobalRole) => `roles.${role}`
export const assignmentRoleKey = (role: AssignmentRole) => `roles.${role}`

export const GLOBAL_ROLES: readonly GlobalRole[] = ['user', 'admin']
export const ASSIGNMENT_ROLES: readonly AssignmentRole[] = ['viewer', 'operator']
