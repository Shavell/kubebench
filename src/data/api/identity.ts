import { apiDelete, apiGet, apiPost, apiPut } from '../apiClient'
import type { IdentityProvider } from '../providers'
import type { AuthSettings, GroupMapping, SignInMethods, SignInPreview } from '../types'

/**
 * External sign-in settings and group mappings (design §3.6, admin
 * routes under /api/settings/auth). Go encodes empty slices as null.
 */
export function createApiIdentity(): IdentityProvider {
  return {
    settings: async () => {
      const settings = await apiGet<AuthSettings>('/settings/auth')
      return {
        ...settings,
        providers: (settings.providers ?? []).map((p) => ({ ...p, settings: p.settings ?? [] })),
      }
    },
    listMappings: async () => (await apiGet<GroupMapping[] | null>('/settings/auth/mappings')) ?? [],
    createMapping: async (mapping) => {
      await apiPost('/settings/auth/mappings', mapping)
    },
    updateMapping: async (id, mapping) => {
      await apiPut(`/settings/auth/mappings/${id}`, mapping)
    },
    deleteMapping: async (id) => {
      await apiDelete(`/settings/auth/mappings/${id}`)
    },
    preview: async (source, username, groups) => {
      const p = await apiPost<SignInPreview>('/settings/auth/preview', { source, username, groups })
      return { ...p, groups: p.groups ?? [], clusters: p.clusters ?? [], matched: p.matched ?? [] }
    },
  }
}

/** GET /api/auth/methods: how users may sign in (public, read before a session exists). */
export async function fetchSignInMethods(): Promise<SignInMethods> {
  const methods = await apiGet<SignInMethods>('/auth/methods')
  return { local: methods.local !== false, providers: methods.providers ?? [] }
}
