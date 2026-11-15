import { apiDelete, apiGet, apiPatch, apiPost, apiPut } from '../apiClient'
import type {
  AuditProvider,
  InventoryProvider,
  PluginsProvider,
  SessionProvider,
  UsersProvider,
} from '../providers'
import type { AppUser, AuditPage, ClusterEntry, PluginCatalog } from '../types'
import { fetchServerInfo } from './info'
import { parsePlatform } from './platform'

/**
 * Inventory, users, session, audit and plugin management. Go encodes empty
 * slices/maps as null, so list fields are normalised to empty values here.
 */

export function createApiInventory(): InventoryProvider {
  return {
    listClusters: async () =>
      (await apiGet<ClusterEntry[]>('/clusters')).map((c) => ({ ...c, platform: parsePlatform(c.platform) })),
    addCluster: async ({ name, kubeconfigPath, content, address, context, credentials }) => {
      await apiPost('/clusters', { name, path: kubeconfigPath, content, address, context, credentials })
    },
    removeCluster: async (name) => {
      await apiDelete('/clusters', { name })
    },
    activateCluster: async (name) => {
      await apiPost('/clusters/activate', undefined, { name })
    },
    updateCluster: async (id, update) => {
      await apiPatch(`/clusters/${id}`, update)
    },
  }
}

export function createApiUsers(): UsersProvider {
  return {
    listUsers: async () =>
      (await apiGet<AppUser[]>('/users')).map((u) => ({
        ...u,
        mustChangePassword: u.mustChangePassword ?? false,
        clusters: u.clusters ?? [],
        assignments: u.assignments ?? [],
      })),
    createUser: async (username, password, role) => {
      await apiPost('/users', { username, password, role })
    },
    updateUser: async (id, update) => {
      await apiPatch(`/users/${id}`, update)
    },
    deleteUser: async (id) => {
      await apiDelete(`/users/${id}`)
    },
    assignClusters: async (id, assignments) => {
      await apiPut(`/users/${id}/clusters`, { assignments })
    },
  }
}

export function createApiSession(): SessionProvider {
  return {
    info: fetchServerInfo,
    changePassword: async (currentPassword, newPassword) => {
      await apiPost('/auth/password', { currentPassword, newPassword })
    },
    logout: async () => {
      await apiPost('/auth/logout')
    },
  }
}

export function createApiAudit(): AuditProvider {
  return {
    listAudit: async ({ limit, before, user, cluster, action }) => {
      const page = await apiGet<AuditPage>('/audit', { limit, before, user, cluster, action })
      return { items: page.items ?? [], nextBefore: page.nextBefore ?? null }
    },
  }
}

export function createApiPlugins(): PluginsProvider {
  return {
    listPlugins: async () => {
      const catalog = await apiGet<PluginCatalog>('/plugins')
      return { plugins: catalog.plugins ?? [], settings: catalog.settings ?? {} }
    },
    setPluginEnabled: async (id, enabled) => {
      await apiPut(`/plugins/${encodeURIComponent(id)}`, { enabled })
    },
  }
}
