import clsx from 'clsx'
import { registry } from '@/core/plugin/registry'
import { useLayoutStore } from '@/core/store/layoutStore'
import { Icon } from '@/shared/Icon'
import { ResizeHandle } from '@/shared/ResizeHandle'
import './BottomPanel.css'

export function BottomPanel() {
  const activePanelTabId = useLayoutStore((s) => s.activePanelTabId)
  const panelMaximized = useLayoutStore((s) => s.panelMaximized)
  const setActivePanelTab = useLayoutStore((s) => s.setActivePanelTab)
  const togglePanel = useLayoutStore((s) => s.togglePanel)
  const togglePanelMaximized = useLayoutStore((s) => s.togglePanelMaximized)

  const tabs = registry.bottomPanelTabs()
  const active = tabs.find((t) => t.id === activePanelTabId) ?? tabs[0]
  const ActiveComponent = active?.component

  return (
    <div className="bottom-panel">
      <ResizeHandle
        orientation="horizontal"
        onResize={(delta) => {
          const s = useLayoutStore.getState()
          s.setPanelHeight(s.panelHeight - delta)
        }}
      />
      <div className="bottom-panel__header">
        <div className="bottom-panel__tabs">
          {tabs.map((tab) => (
            <button
              key={tab.id}
              className={clsx(
                'bottom-panel__tab',
                tab.id === active?.id && 'bottom-panel__tab--active',
              )}
              onClick={() => setActivePanelTab(tab.id)}
            >
              {tab.title}
            </button>
          ))}
        </div>
        <div className="bottom-panel__actions">
          <button
            className="bottom-panel__action"
            title={panelMaximized ? 'Restore Panel Size' : 'Maximize Panel Size'}
            onClick={togglePanelMaximized}
          >
            <Icon name={panelMaximized ? 'chevron-down' : 'chevron-up'} size={16} />
          </button>
          <button className="bottom-panel__action" title="Close Panel" onClick={togglePanel}>
            <Icon name="close" size={16} />
          </button>
        </div>
      </div>
      <div className="bottom-panel__content">{ActiveComponent && <ActiveComponent />}</div>
    </div>
  )
}
