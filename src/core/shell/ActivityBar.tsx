import clsx from 'clsx'
import { registry } from '@/core/plugin/registry'
import { useLayoutStore } from '@/core/store/layoutStore'
import { Icon } from '@/shared/Icon'
import './ActivityBar.css'

export function ActivityBar() {
  const activePluginId = useLayoutStore((s) => s.activePluginId)
  const sidebarCollapsed = useLayoutStore((s) => s.sidebarCollapsed)
  const setActivePlugin = useLayoutStore((s) => s.setActivePlugin)
  const togglePanel = useLayoutStore((s) => s.togglePanel)

  return (
    <div className="activity-bar">
      {registry.activityBarEntries().map((plugin) => (
        <button
          key={plugin.id}
          className={clsx(
            'activity-bar__item',
            plugin.id === activePluginId && !sidebarCollapsed && 'activity-bar__item--active',
          )}
          title={plugin.activityBar!.tooltip}
          onClick={() => setActivePlugin(plugin.id)}
        >
          <Icon name={plugin.activityBar!.icon} size={24} />
        </button>
      ))}
      <div className="activity-bar__spacer" />
      <button className="activity-bar__item" title="Toggle Panel" onClick={togglePanel}>
        <Icon name="layout-panel" size={24} />
      </button>
      <button className="activity-bar__item" title="Settings (mock)">
        <Icon name="settings-gear" size={24} />
      </button>
    </div>
  )
}
