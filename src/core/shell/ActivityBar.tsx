import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import { registry } from '@/core/plugin/registry'
import { useRegistryRevision } from '@/core/plugin/useRegistry'
import { useLayoutStore } from '@/core/store/layoutStore'
import { CORE_NS, useManifestText } from '@/i18n'
import { Icon } from '@/shared/Icon'
import './ActivityBar.css'

export function ActivityBar() {
  // Plugins register and get enabled/disabled at runtime; re-render on every change.
  useRegistryRevision()
  const { t } = useTranslation()
  const manifestText = useManifestText()
  const activePluginId = useLayoutStore((s) => s.activePluginId)
  const sidebarCollapsed = useLayoutStore((s) => s.sidebarCollapsed)
  const setActivePlugin = useLayoutStore((s) => s.setActivePlugin)
  const togglePanel = useLayoutStore((s) => s.togglePanel)
  const openTab = useLayoutStore((s) => s.openTab)

  // The gear opens the settings tab; its renderer is contributed by the
  // settings plugin under the "settings" kind — no direct plugin import.
  const openSettings = () =>
    openTab({
      id: 'settings',
      title: t('common.settings'),
      titleKey: 'common.settings',
      titleNs: CORE_NS,
      icon: 'settings-gear',
      kind: 'settings',
    })

  return (
    <div className="activity-bar">
      {registry.activityBarEntries().map((plugin) => (
        <button
          key={plugin.id}
          className={clsx(
            'activity-bar__item',
            plugin.id === activePluginId && !sidebarCollapsed && 'activity-bar__item--active',
          )}
          title={manifestText(plugin.id, plugin.activityBar!.tooltip)}
          onClick={() => setActivePlugin(plugin.id)}
        >
          <Icon name={plugin.activityBar!.icon} size={24} />
        </button>
      ))}
      <div className="activity-bar__spacer" />
      <button className="activity-bar__item" title={t('shell.panel.toggle')} onClick={togglePanel}>
        <Icon name="layout-panel" size={24} />
      </button>
      <button className="activity-bar__item" title={t('common.settings')} onClick={openSettings}>
        <Icon name="settings-gear" size={24} />
      </button>
    </div>
  )
}
