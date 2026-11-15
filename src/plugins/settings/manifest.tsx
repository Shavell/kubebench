import { lazyComponent } from '@/core/plugin/lazyComponent'
import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { i18n } from '@/i18n'
import { localeBundles } from '@/i18n/bundles'
import { SETTINGS_NS } from './namespace'

// Tab pages load on first use (separate chunks), keeping the main bundle small.
const SettingsPage = lazyComponent(() => import('./SettingsPage').then((m) => m.SettingsPage))

function openSettingsTab(): void {
  useLayoutStore.getState().openTab({
    id: 'settings',
    title: i18n.t('tab.title', { ns: SETTINGS_NS }),
    titleKey: 'tab.title',
    titleNs: SETTINGS_NS,
    icon: 'settings-gear',
    kind: 'settings',
  })
}

/**
 * Settings screen, opened from the activity bar gear (which opens the
 * "settings" tab kind) or the command palette. Display strings are keys in
 * the "settings" namespace.
 */
export const settingsPlugin: PluginManifest = {
  id: 'settings',
  name: 'manifest.name',
  description: 'manifest.description',
  category: 'core',
  core: true,
  locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
  tabRenderers: [{ kind: 'settings', component: SettingsPage }],
  commands: [
    {
      id: 'settings.open',
      title: 'commands.open',
      icon: 'settings-gear',
      keybinding: 'Ctrl+,',
      run: openSettingsTab,
    },
  ],
}
