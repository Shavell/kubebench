import { ApiError } from '../apiClient'
import type { IdentityProvider } from '../providers'
import { USERNAME_PATTERN } from '../rules'
import type { AssignmentRole, AuthSettings, GroupMapping, GroupMappingInput, SignInPreview } from '../types'
import { delay, delayReject } from './delay'
import { mockClusterNames } from './mockInventory'
import { mockAccount } from './mockUsers'

/**
 * External sign-in for the demo (design §3.6): a configured OIDC provider,
 * in-memory group mappings and a preview that resolves them like the
 * server (case-insensitive exact group match, strongest cluster role wins).
 */
const settings: AuthSettings = {
  publicUrl: 'https://kubebench.example.com',
  publicUrlConfigured: true,
  requireGroup: false,
  providers: [
    {
      source: 'oidc',
      name: 'Keycloak',
      status: { ok: true, error: '' },
      settings: [
        { key: 'issuer', value: 'https://sso.example.com/realms/platform' },
        { key: 'clientId', value: 'kubebench' },
        { key: 'clientSecret', value: '', secretSet: true },
        { key: 'redirectUrl', value: 'https://kubebench.example.com/api/auth/oidc/callback' },
        { key: 'scopes', value: 'openid profile email' },
        { key: 'usernameClaim', value: 'preferred_username' },
        { key: 'groupsClaim', value: 'groups' },
        { key: 'allowedDomains', value: 'example.com' },
        { key: 'caFile', value: '' },
      ],
    },
  ],
}

let nextId = 4
const mappings: Omit<GroupMapping, 'clusterExists'>[] = [
  { id: 1, group: 'platform-admins', source: '', role: 'admin', cluster: '' },
  { id: 2, group: 'sre', source: 'oidc', role: 'operator', cluster: 'prod-eu-west-1' },
  { id: 3, group: 'developers', source: '', role: 'viewer', cluster: 'staging' },
]

const badRequest = (message: string) => new ApiError(400, 'bad_request', message)
const conflict = (message: string) => new ApiError(409, 'conflict', message)
const notFound = (id: number) => new ApiError(404, 'not_found', `mapping ${id} not found`)

function clusterNameSet(): Set<string> {
  return new Set(mockClusterNames().values())
}

/** The server's validation of a mapping (store.GroupMapping.Validate). */
function validate(input: GroupMappingInput, exceptId?: number): ApiError | null {
  const group = input.group.trim()
  const cluster = input.cluster.trim()
  if (!group) return badRequest('the group is required')
  if (input.role === 'admin' && cluster) return badRequest('an admin mapping applies to every cluster: leave the cluster empty')
  if (input.role !== 'admin' && !cluster) return badRequest('an operator or viewer mapping needs a cluster')
  const duplicate = mappings.some(
    (m) =>
      m.id !== exceptId &&
      m.group.toLowerCase() === group.toLowerCase() &&
      m.source === input.source &&
      m.cluster === cluster,
  )
  return duplicate ? conflict(`group "${group}" already has a mapping for this cluster`) : null
}

function normalized(input: GroupMappingInput): GroupMappingInput {
  return { ...input, group: input.group.trim(), cluster: input.cluster.trim() }
}

export function createMockIdentity(): IdentityProvider {
  return {
    settings: () => delay(structuredClone(settings)),
    listMappings: () => {
      const known = clusterNameSet()
      const sorted = [...mappings].sort((a, b) => a.group.localeCompare(b.group))
      return delay(sorted.map((m) => ({ ...m, clusterExists: m.cluster === '' || known.has(m.cluster) })))
    },
    createMapping: (input) => {
      const error = validate(input)
      if (error) return delayReject(error)
      mappings.push({ id: nextId++, ...normalized(input) })
      return delay(undefined)
    },
    updateMapping: (id, input) => {
      const mapping = mappings.find((m) => m.id === id)
      if (!mapping) return delayReject(notFound(id))
      const error = validate(input, id)
      if (error) return delayReject(error)
      Object.assign(mapping, normalized(input))
      return delay(undefined)
    },
    deleteMapping: (id) => {
      const index = mappings.findIndex((m) => m.id === id)
      if (index < 0) return delayReject(notFound(id))
      mappings.splice(index, 1)
      return delay(undefined)
    },
    preview: (source, rawUsername, groups) => {
      const username = rawUsername.trim()
      const known = mockAccount(username)
      const result: SignInPreview = {
        source,
        username,
        groups: groups ?? (known?.source === source ? known.groups : []),
        groupsFrom: groups ? 'input' : known?.source === source ? 'lastSignIn' : 'none',
        admin: false,
        clusters: [],
        matched: [],
        allowed: true,
        reason: '',
        existingSource: known?.source ?? '',
        lastSignIn: known?.source === source ? known.lastSignIn : null,
      }
      if (!USERNAME_PATTERN.test(username)) return delay({ ...result, allowed: false, reason: 'invalid_username' })
      const clusterNames = clusterNameSet()
      const roles = new Map<string, AssignmentRole>()
      for (const m of mappings) {
        if (m.source && m.source !== source) continue
        if (!result.groups.some((g) => g.toLowerCase() === m.group.toLowerCase())) continue
        result.matched.push(m.id)
        if (m.role === 'admin') result.admin = true
        else if (roles.get(m.cluster) !== 'operator') roles.set(m.cluster, m.role)
      }
      result.clusters = [...roles]
        .sort(([a], [b]) => a.localeCompare(b))
        .map(([cluster, role]) => ({ cluster, role, exists: clusterNames.has(cluster) }))
      if (known && known.source !== source) {
        result.allowed = false
        result.reason = 'username_conflict'
      }
      return delay(result)
    },
  }
}
