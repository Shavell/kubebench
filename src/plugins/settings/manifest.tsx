import type { PluginManifest } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { SettingsPage } from './SettingsPage'

export function openSettingsTab(): void {
  useLayoutStore.getState().openTab({
    id: 'settings',
    title: 'Settings',
    icon: 'settings-gear',
    kind: 'settings',
  })
}

/** Settings screen; opened from the activity bar gear or the palette. */
export const settingsPlugin: PluginManifest = {
  id: 'settings',
  name: 'Settings',
  tabRenderers: [{ kind: 'settings', component: SettingsPage }],
  commands: [
    {
      id: 'settings.open',
      title: 'Preferences: Open Settings',
      icon: 'settings-gear',
      run: openSettingsTab,
    },
  ],
}
