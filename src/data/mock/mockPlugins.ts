import type { PluginsProvider } from '../providers'
import type { ServerPlugin } from '../types'
import { delay } from './delay'

/**
 * Mock plugin catalog: the v0.5 backend plugins, all enabled and
 * available. Enable/disable choices are kept in memory for the session.
 */
function backendPlugin(id: string, name: string, category: string, description: string): ServerPlugin {
  return {
    id,
    name,
    version: __APP_VERSION__,
    description,
    category,
    builtin: true,
    external: false,
    enabled: true,
    status: { available: true },
    entry: '',
  }
}

const SERVER_PLUGINS: ServerPlugin[] = [
  backendPlugin('helm', 'Helm', 'tools', 'Helm releases, repositories and charts'),
  backendPlugin('argocd', 'Argo CD', 'gitops', 'Argo CD applications through the Argo CD CRDs'),
  backendPlugin('storage', 'Storage', 'storage', 'PersistentVolume, PVC and StorageClass creator'),
  backendPlugin('security', 'Security', 'security', 'Security posture checks'),
  backendPlugin('access', 'Access', 'security', 'RBAC explorer'),
]

const settings: Record<string, { enabled: boolean }> = {}

export function createMockPlugins(): PluginsProvider {
  return {
    listPlugins: () =>
      delay({
        plugins: SERVER_PLUGINS.map((p) => ({ ...p, enabled: settings[p.id]?.enabled ?? p.enabled })),
        settings: { ...settings },
      }),
    setPluginEnabled: (id, enabled) => {
      settings[id] = { enabled }
      return delay(undefined)
    },
  }
}
