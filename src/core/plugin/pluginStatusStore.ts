import { create } from 'zustand'
import { PLATFORM_NS } from '@/core/locales'
import { errorMessage } from '@/data/apiClient'
import type { PluginsProvider } from '@/data/providers'
import type { ServerPlugin } from '@/data/types'
import { i18n } from '@/i18n'
import type { PluginManifest, PluginSource } from './types'

/** Why an external plugin's script could not be loaded or did not register. */
export interface PluginLoadError {
  reason: 'invalidEntry' | 'network' | 'timeout' | 'threw' | 'notRegistered'
  /** The script URL, or the message of the exception the script threw. */
  detail?: string
}

/**
 * Enable/disable choices and availability of plugins (design §6.4), loaded
 * from GET /api/plugins at boot. The registry reads it to decide which
 * contributions the shell shows; the Extensions view edits it.
 */
interface PluginStatusState {
  /** False until the catalog has been loaded (or failed to load). */
  loaded: boolean
  /** Mock mode: availability is not probed, every plugin counts as available. */
  assumeAvailable: boolean
  /** Why the catalog could not be loaded; availability is then unknown and assumed. */
  loadError: string | null
  /** Backend and external plugins reported by the server, by id. */
  serverPlugins: Record<string, ServerPlugin>
  /** Admin enable/disable choices for any plugin id. */
  settings: Record<string, { enabled: boolean }>
  /** External plugin id → why its script failed to load or register. */
  externalErrors: Record<string, PluginLoadError>

  load: (provider: PluginsProvider, options: { assumeAvailable: boolean }) => Promise<void>
  setEnabled: (provider: PluginsProvider, id: string, enabled: boolean) => Promise<void>
  setExternalError: (id: string, error: PluginLoadError | null) => void
}

export const usePluginStatusStore = create<PluginStatusState>((set) => ({
  loaded: false,
  assumeAvailable: false,
  loadError: null,
  serverPlugins: {},
  settings: {},
  externalErrors: {},

  load: async (provider, { assumeAvailable }) => {
    try {
      const catalog = await provider.listPlugins()
      set({
        loaded: true,
        assumeAvailable,
        loadError: null,
        serverPlugins: Object.fromEntries(catalog.plugins.map((p) => [p.id, p])),
        settings: catalog.settings,
      })
    } catch (err) {
      set({ loaded: true, assumeAvailable, loadError: errorMessage(err) })
    }
  },

  setEnabled: async (provider, id, enabled) => {
    await provider.setPluginEnabled(id, enabled)
    set((s) => ({
      settings: { ...s.settings, [id]: { enabled } },
      serverPlugins: s.serverPlugins[id]
        ? { ...s.serverPlugins, [id]: { ...s.serverPlugins[id], enabled } }
        : s.serverPlugins,
    }))
  },

  setExternalError: (id, error) =>
    set((s) => {
      const externalErrors = { ...s.externalErrors }
      if (error) externalErrors[id] = error
      else delete externalErrors[id]
      return { externalErrors }
    }),
}))

export interface ResolvedPluginStatus {
  enabled: boolean
  available: boolean
  /** Why the plugin is unavailable (translated). */
  reason?: string
  /** The catalog has not been loaded yet. */
  loading: boolean
}

type StatusInputs = Pick<
  PluginStatusState,
  'loaded' | 'assumeAvailable' | 'loadError' | 'serverPlugins' | 'settings' | 'externalErrors'
>

const t = (key: string, options?: Record<string, unknown>) =>
  i18n.t(key, { ns: PLATFORM_NS, ...options })

/** Translated explanation of an external plugin load failure. */
export function describeLoadError(error: PluginLoadError): string {
  return t(`plugins.load.${error.reason}`, { detail: error.detail ?? '' })
}

/**
 * Effective status of one plugin. Core plugins are always on. Otherwise
 * the admin's choice wins, then the server's flag, then the default:
 * built-in plugins start enabled, external ones disabled.
 */
export function resolvePluginStatus(
  id: string,
  manifest: Pick<PluginManifest, 'core' | 'backendPlugin'> | undefined,
  source: PluginSource,
  state: StatusInputs = usePluginStatusStore.getState(),
): ResolvedPluginStatus {
  const loading = !state.loaded
  if (manifest?.core) return { enabled: true, available: true, loading }

  const server = state.serverPlugins[id]
  const enabled = state.settings[id]?.enabled ?? server?.enabled ?? source === 'builtin'
  const unavailable = (reason: string) => ({ enabled, available: false, reason, loading })

  const externalError = state.externalErrors[id]
  if (externalError) return unavailable(describeLoadError(externalError))
  // Without a catalog (mock mode, or the request failed) availability is unknown.
  if (state.assumeAvailable || state.loadError) return { enabled, available: true, loading }

  const backendId = manifest?.backendPlugin
  if (backendId) {
    const backend = state.serverPlugins[backendId]
    if (!backend) return unavailable(t('plugins.reasons.backendMissing', { id: backendId }))
    if (!backend.enabled) return unavailable(t('plugins.reasons.backendDisabled', { id: backendId }))
    if (!backend.status.available) {
      return unavailable(backend.status.reason || t('plugins.reasons.backendUnavailable', { id: backendId }))
    }
  }
  if (server && !server.status.available) {
    return unavailable(server.status.reason || t('plugins.reasons.unavailable', { id }))
  }
  return { enabled, available: true, loading }
}
