import { useTranslation } from 'react-i18next'
import { loadExternalPlugin } from '@/core/plugin/externalPlugins'
import { describeLoadError, usePluginStatusStore } from '@/core/plugin/pluginStatusStore'
import { registry } from '@/core/plugin/registry'
import { useProviders } from '@/data/DataContext'
import { dataMode } from '@/data/mode'
import { notify } from '@/shared/notify'
import type { Extension } from './catalog'
import { EXTENSIONS_NS } from './namespace'

/**
 * Enables or disables a plugin for everyone (admin, PUT /api/plugins/{id}).
 * Enabling an external plugin whose script was never loaded loads it now;
 * plugins with a server part re-read the catalog, whose availability the
 * server decides. Rejects with the server's error.
 */
export function usePluginToggle(): (extension: Extension, enabled: boolean) => Promise<void> {
  const { t } = useTranslation(EXTENSIONS_NS)
  const { plugins } = useProviders()

  return async (extension, enabled) => {
    const store = usePluginStatusStore.getState()
    await store.setEnabled(plugins, extension.id, enabled)
    if (enabled && extension.source === 'external' && !registry.get(extension.id)) {
      const entry = usePluginStatusStore.getState().serverPlugins[extension.id]
      if (entry) await loadExternalPlugin(entry)
    }
    if (extension.server) await store.load(plugins, { assumeAvailable: dataMode.mode === 'mock' })

    const loadError = usePluginStatusStore.getState().externalErrors[extension.id]
    if (enabled && loadError) {
      notify.error(t('toggle.loadFailed', { name: extension.name }), { detail: describeLoadError(loadError) })
    } else {
      notify.success(t(enabled ? 'toggle.enabled' : 'toggle.disabled', { name: extension.name }))
    }
  }
}
