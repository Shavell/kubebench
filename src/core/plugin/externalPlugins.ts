import type { ServerPlugin } from '@/data/types'
import { PLUGIN_ID_PATTERN } from '@/data/rules'
import { usePluginStatusStore, type PluginLoadError } from './pluginStatusStore'
import { registry } from './registry'
import type { PluginManifest as SdkPluginManifest } from './sdk/kubebench'

/**
 * Runtime-loaded UI plugins (design §6.8). Each enabled external plugin in
 * the /api/plugins catalog gets a <script> for its entry; the script calls
 * `window.KubeBench.registerPlugin(manifest)` while it runs. Failures are
 * recorded in the plugin status store, where the Extensions view shows them.
 */

const LOAD_TIMEOUT_MS = 5000

/** Plugins whose script is loading: only these may register, each once. */
const loading = new Map<string, { registered: boolean }>()

const isObject = (value: unknown): value is Record<string, unknown> =>
  typeof value === 'object' && value !== null

/**
 * `window.KubeBench.registerPlugin`. Throws (inside the plugin's script,
 * where the loader reports it) for a malformed manifest or an id that is
 * not an external plugin being loaded right now, so a plugin can neither
 * register under another plugin's id nor declare itself core.
 */
export function registerExternalPlugin(manifest: SdkPluginManifest): void {
  if (!isObject(manifest) || typeof manifest.id !== 'string' || typeof manifest.name !== 'string') {
    throw new Error('KubeBench.registerPlugin: the manifest needs a string id and name')
  }
  const slot = loading.get(manifest.id)
  if (!slot) {
    throw new Error(`KubeBench.registerPlugin: "${manifest.id}" is not an enabled external plugin that is loading`)
  }
  if (slot.registered) throw new Error(`KubeBench.registerPlugin: "${manifest.id}" is already registered`)
  const server = usePluginStatusStore.getState().serverPlugins[manifest.id]
  registry.register({ ...manifest, version: manifest.version ?? server?.version, core: false }, 'external')
  slot.registered = true
}

/** The plugin's script URL, if the catalog entry points into its own /plugins/<id>/ directory. */
function entryUrl(plugin: ServerPlugin): URL | null {
  if (!PLUGIN_ID_PATTERN.test(plugin.id)) return null
  try {
    const url = new URL(plugin.entry, window.location.origin)
    const inPluginDir = url.pathname.startsWith(`/plugins/${plugin.id}/`)
    return url.origin === window.location.origin && inPluginDir ? url : null
  } catch {
    return null
  }
}

/** Loads one external plugin's script. Resolves when it registered or failed (never rejects). */
export function loadExternalPlugin(plugin: ServerPlugin): Promise<void> {
  const store = usePluginStatusStore.getState()
  if (registry.get(plugin.id) || loading.has(plugin.id)) return Promise.resolve()
  store.setExternalError(plugin.id, null)
  const url = entryUrl(plugin)
  if (!url) {
    store.setExternalError(plugin.id, { reason: 'invalidEntry', detail: plugin.entry })
    return Promise.resolve()
  }

  return new Promise((resolve) => {
    const slot = { registered: false }
    loading.set(plugin.id, slot)
    const script = document.createElement('script')
    let thrown: string | null = null

    // An exception while the script runs (including one from registerPlugin)
    // reaches window.onerror, not the script's error event.
    const onWindowError = (event: ErrorEvent) => {
      if (event.filename === url.href) thrown ??= event.message
    }
    const finish = (error: PluginLoadError | null) => {
      clearTimeout(timer)
      window.removeEventListener('error', onWindowError)
      script.onload = script.onerror = null
      loading.delete(plugin.id)
      if (error) {
        script.remove()
        usePluginStatusStore.getState().setExternalError(plugin.id, error)
      }
      resolve()
    }
    const timer = setTimeout(
      () => finish({ reason: 'timeout', detail: String(LOAD_TIMEOUT_MS / 1000) }),
      LOAD_TIMEOUT_MS,
    )

    window.addEventListener('error', onWindowError)
    script.onload = () => {
      if (slot.registered) finish(null)
      else if (thrown) finish({ reason: 'threw', detail: thrown })
      else finish({ reason: 'notRegistered' })
    }
    script.onerror = () => finish({ reason: 'network', detail: url.pathname })
    script.src = url.href
    script.async = true
    script.dataset.kubebenchPlugin = plugin.id
    document.head.append(script)
  })
}

/** Loads every enabled external plugin of the catalog, in parallel. */
export async function loadExternalPlugins(plugins: ServerPlugin[]): Promise<void> {
  const enabled = plugins.filter((p) => p.external && p.enabled && p.entry)
  await Promise.all(enabled.map(loadExternalPlugin))
}
