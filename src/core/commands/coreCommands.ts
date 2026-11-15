import type { TFunction } from 'i18next'
import { registry } from '@/core/plugin/registry'
import type { CommandContribution } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { openNamespacePicker } from '@/core/store/namespacePickerRequest'
import type { useManifestText } from '@/i18n'
import { focusListFilter } from '@/shared/listFilterFocus'

type ManifestText = ReturnType<typeof useManifestText>

const layout = () => useLayoutStore.getState()

/** Activates the editor tab `delta` places away from the active one, wrapping around. */
function cycleTab(delta: number): void {
  const { tabs, activeTabId, setActiveTab } = layout()
  if (tabs.length === 0) return
  const index = tabs.findIndex((t) => t.id === activeTabId)
  setActiveTab(tabs[(index + delta + tabs.length) % tabs.length].id)
}

/**
 * Commands of the workbench itself rather than of a plugin (titles
 * translated with the core namespace's `t`), with their shortcuts. The
 * palette lists them and the shell binds their keybindings.
 */
export function coreCommands(t: TFunction, manifestText: ManifestText): CommandContribution[] {
  return [
    {
      id: 'core.togglePanel',
      title: t('shell.commands.togglePanel'),
      icon: 'layout-panel',
      keybinding: 'Ctrl+`',
      run: () => layout().togglePanel(),
    },
    {
      id: 'core.toggleSidebar',
      title: t('shell.commands.toggleSidebar'),
      icon: 'layout-sidebar-left',
      keybinding: 'Ctrl+B',
      run: () => layout().setActivePlugin(layout().activePluginId),
    },
    {
      id: 'core.closeTab',
      title: t('shell.commands.closeTab'),
      icon: 'close',
      keybinding: 'Alt+W',
      run: () => {
        const s = layout()
        if (s.activeTabId) s.closeTab(s.activeTabId)
      },
    },
    {
      id: 'core.closeAllTabs',
      title: t('shell.commands.closeAllTabs'),
      icon: 'close-all',
      run: () => layout().closeAllTabs(),
    },
    {
      id: 'core.nextTab',
      title: t('shell.commands.nextTab'),
      icon: 'arrow-right',
      keybinding: 'Alt+PageDown',
      run: () => cycleTab(1),
    },
    {
      id: 'core.previousTab',
      title: t('shell.commands.previousTab'),
      icon: 'arrow-left',
      keybinding: 'Alt+PageUp',
      run: () => cycleTab(-1),
    },
    {
      id: 'core.focusFilter',
      title: t('shell.commands.focusFilter'),
      icon: 'filter',
      keybinding: '/',
      run: focusListFilter,
    },
    {
      id: 'core.pickNamespace',
      title: t('shell.commands.pickNamespace'),
      icon: 'folder',
      keybinding: 'Alt+N',
      run: openNamespacePicker,
    },
    ...registry.activityBarEntries().map((p) => ({
      id: `core.show.${p.id}`,
      title: t('shell.commands.showPlugin', { name: manifestText(p.id, p.name) }),
      icon: p.activityBar!.icon,
      run: () => {
        const s = layout()
        if (s.activePluginId !== p.id) s.setActivePlugin(p.id)
        else if (s.sidebarCollapsed) s.setActivePlugin(p.id)
      },
    })),
  ]
}

/** Every command with its title translated: the workbench's, then the plugins'. */
export function allCommands(t: TFunction, manifestText: ManifestText): CommandContribution[] {
  return [
    ...coreCommands(t, manifestText),
    ...registry.commands().map((c) => ({ ...c, title: manifestText(c.pluginId, c.title) })),
  ]
}
