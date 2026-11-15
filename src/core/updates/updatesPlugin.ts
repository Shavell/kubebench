import type { CommandContribution, PluginManifest } from '@/core/plugin/types'
import { dataMode } from '@/data/mode'
import type { DataProviders } from '@/data/providers'
import { ChangelogTab } from './ChangelogTab'
import { UPDATES_NS, updatesLocales } from './namespace'
import { runUpdateCheck } from './runUpdateCheck'
import { UpdatesSettingsSection } from './UpdatesSettingsSection'
import { UpdateStatusItem } from './UpdateStatusItem'
import { CHANGELOG_TAB_KIND, openChangelogTab } from './whatsNew'

/**
 * Release checks and What's new (design §7.11), registered at boot like the
 * workbench plugin: the Settings section, the status bar item (admins), the
 * changelog tab and the Help commands. The dialogs are hosted by the shell
 * (<UpdatesDialogs/>).
 */
export function createUpdatesPlugin(providers: DataProviders): PluginManifest {
  const commands: CommandContribution[] = [
    { id: 'updates.whatsNew', title: 'commands.whatsNew', icon: 'sparkle', run: openChangelogTab },
  ]
  if (dataMode.permissions?.isAdmin) {
    commands.push({
      id: 'updates.check',
      title: 'commands.checkNow',
      icon: 'arrow-circle-up',
      run: () => void runUpdateCheck(providers.updates, { openDialog: true }),
    })
  }
  return {
    id: UPDATES_NS,
    name: 'manifest.name',
    description: 'manifest.description',
    category: 'core',
    core: true,
    locales: updatesLocales,
    tabRenderers: [{ kind: CHANGELOG_TAB_KIND, component: ChangelogTab }],
    statusBarItems: [{ id: 'update-available', alignment: 'right', priority: 10, component: UpdateStatusItem }],
    settingsSections: [{ id: 'updates', title: 'settings.title', component: UpdatesSettingsSection }],
    commands,
  }
}
