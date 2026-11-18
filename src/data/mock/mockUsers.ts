import { reasonError } from '../reasonError'
import type { UsersProvider } from '../providers'
import { MIN_PASSWORD_LENGTH, passwordLength, USERNAME_PATTERN } from '../rules'
import type { AppUser, AssignmentRole, GlobalRole } from '../types'
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
    clusters: assignments.map((a) => a.cluster),
    assignments,
  }
}

function find(id: number): StoredUser | undefined {
  return users.find((u) => u.id === id)
}

const notFound = (id: number) => reasonError(404, 'not_found', 'users.not_found', { id })
const isLastAdmin = (user: StoredUser) =>
  user.role === 'admin' && users.filter((u) => u.role === 'admin').length === 1

export function createMockUsers(): UsersProvider {
  return {
    listUsers: () => {
      const names = mockClusterNames()
      return delay(users.map((u) => toAppUser(u, names)))
    },
    createUser: (rawUsername, password, role) => {
      const username = rawUsername.trim()
      if (!USERNAME_PATTERN.test(username)) {
        return delayReject(reasonError(400, 'bad_request', 'users.invalid_username'))
      }
      if (passwordLength(password) < MIN_PASSWORD_LENGTH) {
        return delayReject(reasonError(400, 'bad_request', 'password.too_short', { min: MIN_PASSWORD_LENGTH }))
      }
      if (users.some((u) => u.username === username)) {
        return delayReject(reasonError(409, 'conflict', 'users.exists', { name: username }))
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
        return delayReject(reasonError(409, 'conflict', 'users.last_admin_demote'))
      }
      if (update.password !== undefined && passwordLength(update.password) < MIN_PASSWORD_LENGTH) {
        return delayReject(reasonError(400, 'bad_request', 'password.too_short', { min: MIN_PASSWORD_LENGTH }))
      }
      if (update.role) user.role = update.role
      if (update.password !== undefined) user.mustChangePassword = true
      return delay(undefined)
    },
    deleteUser: (id) => {
      const user = find(id)
      if (!user) return delayReject(notFound(id))
      if (user.username === MOCK_USERNAME) {
        return delayReject(reasonError(409, 'conflict', 'users.delete_self'))
      }
      if (isLastAdmin(user)) return delayReject(reasonError(409, 'conflict', 'users.last_admin_delete'))
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
