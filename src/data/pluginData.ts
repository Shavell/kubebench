import type { DependencyList } from 'react'
import { registry } from '@/core/plugin/registry'
import { resolvePluginStatus, usePluginStatusStore } from '@/core/plugin/pluginStatusStore'
import { apiDelete, apiGet, apiPatch, apiPost, apiPut, type QueryParams } from './apiClient'
import { useQuery, type QueryScope, type QueryState } from './DataContext'
import { dataMode } from './mode'

/**
 * Data access for plugin-owned capabilities (design §6.5). A plugin keeps
 * its capability in `src/plugins/<id>/data/` (types, provider interface,
 * mock, live) and exports the getter made by `definePluginData`.
 */

/** Client for a backend plugin's routes: `/api/plugins/<pluginId><path>`. */
export function pluginApi(pluginId: string) {
  const base = `/plugins/${encodeURIComponent(pluginId)}`
  return {
    get: <T>(path: string, params?: QueryParams) => apiGet<T>(`${base}${path}`, params),
    post: <T>(path: string, body?: unknown, params?: QueryParams) =>
      apiPost<T>(`${base}${path}`, body, params),
    put: <T>(path: string, body?: unknown, params?: QueryParams) =>
      apiPut<T>(`${base}${path}`, body, params),
    patch: <T>(path: string, body?: unknown, params?: QueryParams) =>
      apiPatch<T>(`${base}${path}`, body, params),
    del: <T>(path: string, params?: QueryParams) => apiDelete<T>(`${base}${path}`, params),
  }
}

export type PluginApi = ReturnType<typeof pluginApi>

/**
 * Returns a memoized getter for a plugin capability that picks the mock or
 * live implementation by the boot data mode. Live never falls back to mock.
 * Call the getter from components or hooks (after boot), not at module scope.
 */
export function definePluginData<T>(
  pluginId: string,
  impl: { mock: () => T; live: (api: PluginApi) => T },
): () => T {
  let cached: { mode: typeof dataMode.mode; value: T } | null = null
  return () => {
    if (cached?.mode !== dataMode.mode) {
      cached = {
        mode: dataMode.mode,
        value: dataMode.mode === 'live' ? impl.live(pluginApi(pluginId)) : impl.mock(),
      }
    }
    return cached.value
  }
}

/**
 * Query hook for plugin data; re-runs on `deps` changes and on
 * refreshData(), and — when `scope` names the resources it reads (topics
 * like `pods` or `applications.argoproj.io`) — when live updates report
 * a change of them (design §7.10).
 */
export function usePluginQuery<T>(fn: () => Promise<T>, deps: DependencyList, scope?: QueryScope): QueryState<T> {
  return useQuery(fn, deps, scope)
}

/** Enabled/available state of a plugin, e.g. to show an Unavailable page. */
export function usePluginStatus(pluginId: string): {
  enabled: boolean
  available: boolean
  reason?: string
  loading: boolean
} {
  const state = usePluginStatusStore()
  return resolvePluginStatus(
    pluginId,
    registry.get(pluginId),
    registry.sourceOf(pluginId) ?? 'external',
    state,
  )
}
