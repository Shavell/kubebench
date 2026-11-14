import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { i18n } from '@/i18n'
import en from './locales/en.json'
import tr from './locales/tr.json'
import { SETTINGS_NS } from './namespace'
import { SettingsPage } from './SettingsPage'

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
  locales: { en, tr },
  tabRenderers: [{ kind: 'settings', component: SettingsPage }],
  commands: [
    {
      id: 'settings.open',
      title: 'commands.open',
      icon: 'settings-gear',
      run: openSettingsTab,
    },
  ],
}
