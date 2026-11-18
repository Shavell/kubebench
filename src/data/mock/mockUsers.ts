import { ApiError } from '../apiClient'
import type { UsersProvider } from '../providers'
import { MIN_PASSWORD_LENGTH, passwordLength, USERNAME_PATTERN } from '../rules'
import type { AppUser, AssignmentRole, AuthSource, GlobalRole } from '../types'
import { delay, delayReject } from './delay'
import { mockClusterNames } from './mockInventory'

/**
 * In-memory user store for mock mode. It mirrors the server rules that
 * matter to the UI (unique names, password length, last admin, cannot
 * delete yourself) so error paths can be exercised without a backend.
 */
interface StoredUser {
  id: number
  username: string
  role: GlobalRole
  mustChangePassword: boolean
  assignments: { clusterId: number; role: AssignmentRole }[]
  /** External users (design §3.6): source, profile and the groups of their last sign-in. */
  external?: { source: Exclude<AuthSource, 'local'>; displayName: string; email: string; groups: string[]; lastSignIn: string }
}

/** The mock session always belongs to this user. */
export const MOCK_USERNAME = 'admin'

const users: StoredUser[] = [
  { id: 1, username: MOCK_USERNAME, role: 'admin', mustChangePassword: false, assignments: [] },
  {
    id: 2,
    username: 'alice',
    role: 'user',
    mustChangePassword: false,
    assignments: [
      { clusterId: 1, role: 'operator' },
      { clusterId: 2, role: 'operator' },
      { clusterId: 3, role: 'viewer' },
    ],
  },
  {
    id: 3,
    username: 'bob',
    role: 'user',
    mustChangePassword: true,
    assignments: [{ clusterId: 1, role: 'viewer' }],
  },
  { id: 4, username: 'ops-lead', role: 'admin', mustChangePassword: false, assignments: [] },
  {
    id: 5,
    username: 'jane.doe',
    role: 'user',
    mustChangePassword: false,
    assignments: [{ clusterId: 3, role: 'operator' }],
    external: {
      source: 'oidc',
      displayName: 'Jane Doe',
      email: 'jane.doe@example.com',
      groups: ['sre', 'everyone'],
      lastSignIn: '2026-11-02T08:30:00Z',
    },
  },
]

function toAppUser(user: StoredUser, names: Map<number, string>): AppUser {
  const assignments = user.assignments
    .filter((a) => names.has(a.clusterId))
    .map((a) => ({ clusterId: a.clusterId, cluster: names.get(a.clusterId)!, role: a.role }))
  return {
    id: user.id,
    username: user.username,
    role: user.role,
    mustChangePassword: user.mustChangePassword,
    source: user.external?.source ?? 'local',
    displayName: user.external?.displayName ?? '',
    email: user.external?.email ?? '',
    clusters: assignments.map((a) => a.cluster),
    assignments,
  }
}

/** The account with this username: its source and, for external users, the last sign-in. */
export function mockAccount(
  username: string,
): { source: AuthSource; groups: string[]; lastSignIn: string | null } | undefined {
  const user = users.find((u) => u.username === username)
  if (!user) return undefined
  if (!user.external) return { source: 'local', groups: [], lastSignIn: null }
  return { source: user.external.source, groups: user.external.groups, lastSignIn: user.external.lastSignIn }
}

function find(id: number): StoredUser | undefined {
  return users.find((u) => u.id === id)
}

export const badRequest = (message: string) => new ApiError(400, 'bad_request', message)
const conflict = (message: string) => new ApiError(409, 'conflict', message)
const notFound = (id: number) => new ApiError(404, 'not_found', `user ${id} not found`)
/** Only local admins count: they are the break-glass accounts (design §3.6). */
const isLastAdmin = (user: StoredUser) =>
  user.role === 'admin' && !user.external && users.filter((u) => u.role === 'admin' && !u.external).length === 1

export function createMockUsers(): UsersProvider {
  return {
    listUsers: () => {
      const names = mockClusterNames()
      return delay(users.map((u) => toAppUser(u, names)))
    },
    createUser: (rawUsername, password, role) => {
      const username = rawUsername.trim()
      if (!USERNAME_PATTERN.test(username)) {
        return delayReject(badRequest("a username is 1-64 letters, digits, '.', '_', '@' or '-'"))
      }
      if (passwordLength(password) < MIN_PASSWORD_LENGTH) {
        return delayReject(badRequest(`password must be at least ${MIN_PASSWORD_LENGTH} characters`))
      }
      if (users.some((u) => u.username === username)) {
        return delayReject(conflict(`user "${username}" already exists`))
      }
      users.push({
        id: Math.max(0, ...users.map((u) => u.id)) + 1,
        username,
        role,
        mustChangePassword: true,
        assignments: [],
      })
      return delay(undefined)
    },
    updateUser: (id, update) => {
      const user = find(id)
      if (!user) return delayReject(notFound(id))
      if (update.role === 'user' && isLastAdmin(user)) {
        return delayReject(conflict('cannot demote the last local admin'))
      }
      if (update.password !== undefined && user.external) {
        return delayReject(badRequest('this account signs in through an identity provider; its password is managed there'))
      }
      if (update.password !== undefined && passwordLength(update.password) < MIN_PASSWORD_LENGTH) {
        return delayReject(badRequest(`password must be at least ${MIN_PASSWORD_LENGTH} characters`))
      }
      if (update.role) user.role = update.role
      if (update.password !== undefined) user.mustChangePassword = true
      return delay(undefined)
    },
    deleteUser: (id) => {
      const user = find(id)
      if (!user) return delayReject(notFound(id))
      if (user.username === MOCK_USERNAME) {
        return delayReject(conflict('cannot delete your own account'))
      }
      if (isLastAdmin(user)) return delayReject(conflict('cannot delete the last local admin'))
      users.splice(users.indexOf(user), 1)
      return delay(undefined)
    },
    assignClusters: (id, assignments) => {
      const user = find(id)
      if (!user) return delayReject(notFound(id))
      user.assignments = assignments.map((a) => ({ ...a }))
      return delay(undefined)
    },
  }
}
