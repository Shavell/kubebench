import {
  resolvePluginStatus,
  usePluginStatusStore,
  type PluginLoadError,
  type ResolvedPluginStatus,
} from '@/core/plugin/pluginStatusStore'
import { registry } from '@/core/plugin/registry'
import type { PluginManifest, PluginSource } from '@/core/plugin/types'
import { useRegistryRevision } from '@/core/plugin/useRegistry'
import { appVersion } from '@/data/mode'
import type { ServerPlugin } from '@/data/types'
import { useManifestText } from '@/i18n'

export type ExtensionGroup = 'core' | 'builtin' | 'external'

/** One plugin as the Extensions view shows it: its UI part, its server part, or both. */
export interface Extension {
  id: string
  /** Display strings, already translated. */
  name: string
  description: string
  version: string
  category: string
  icon: string
  group: ExtensionGroup
  source: PluginSource
  core: boolean
  /** The registered UI part, if any (a backend plugin may have none). */
  manifest?: PluginManifest
  /** The entry in GET /api/plugins (backend and external plugins). */
  server?: ServerPlugin
  status: ResolvedPluginStatus
  loadError?: PluginLoadError
}

const GROUP_ORDER: Record<ExtensionGroup, number> = { core: 0, builtin: 1, external: 2 }

/**
 * Every plugin the workbench knows: registered UI plugins plus the
 * server's catalog (backend plugins, and external plugins whether or not
 * their script is loaded). Follows registrations and status changes.
 */
export function useExtensions(): Extension[] {
  useRegistryRevision()
  const state = usePluginStatusStore()
  const text = useManifestText()

  const ids = new Set([...registry.all().map((p) => p.id), ...Object.keys(state.serverPlugins)])
  const extensions = [...ids].map((id): Extension => {
    const manifest = registry.get(id)
    const server = state.serverPlugins[id]
    const source: PluginSource = registry.sourceOf(id) ?? (server?.external ? 'external' : 'builtin')
    const core = Boolean(manifest?.core)
    return {
      id,
      name: manifest ? text(id, manifest.name) : (server?.name ?? id),
      description: manifest?.description ? text(id, manifest.description) : (server?.description ?? ''),
      version: manifest?.version ?? server?.version ?? (source === 'builtin' ? appVersion() : ''),
      category: manifest?.category ?? server?.category ?? 'tools',
      icon: manifest?.activityBar?.icon ?? (source === 'external' ? 'extensions' : 'package'),
      group: core ? 'core' : source,
      source,
      core,
      manifest,
      server,
      status: resolvePluginStatus(id, manifest, source, state),
      loadError: state.externalErrors[id],
    }
  })
  return extensions.sort(
    (a, b) => GROUP_ORDER[a.group] - GROUP_ORDER[b.group] || a.name.localeCompare(b.name),
  )
}
