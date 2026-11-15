import en from '@/core/locales/en.json'
import tr from '@/core/locales/tr.json'
import type { PluginManifest } from '@/core/plugin/types'
import { signOut } from '@/core/session/signOut'
import type { DataProviders } from '@/data/providers'
import { dataMode } from '@/data/mode'
import { i18n } from '@/i18n'
import { notify } from '@/shared/notify'
import { useWorkbenchDialog } from './workbenchDialogs'

const NS = 'workbench'

/**
 * The workbench's own contributions (commands for the About dialog, the
 * keyboard shortcuts cheat sheet and the account), registered through the plugin registry like any module so they
 * appear in the command palette and the Extensions view. Its display
 * strings live under "workbench" in the platform locale files.
 */
export function createWorkbenchPlugin(providers: DataProviders): PluginManifest {
  const show = useWorkbenchDialog.getState().show
  return {
    id: NS,
    name: 'manifest.name',
    description: 'manifest.description',
    category: 'core',
    core: true,
    locales: { en: en.workbench, tr: tr.workbench },
    commands: [
      { id: 'workbench.about', title: 'commands.about', icon: 'info', run: () => show('about') },
      { id: 'workbench.shortcuts', title: 'commands.shortcuts', icon: 'keyboard', keybinding: '?', run: () => show('shortcuts') },
      { id: 'workbench.changePassword', title: 'commands.changePassword', icon: 'key', run: () => show('changePassword') },
      {
        id: 'workbench.signOut',
        title: 'commands.signOut',
        icon: 'sign-out',
        run: () => {
          if (dataMode.mode === 'mock') notify.info(i18n.t('notices.demoSignOut', { ns: NS }))
          else void signOut(providers.session)
        },
      },
    ],
  }
}
