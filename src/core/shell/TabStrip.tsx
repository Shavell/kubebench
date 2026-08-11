import clsx from 'clsx'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { TabDescriptor } from '@/core/plugin/types'
import { Icon } from '@/shared/Icon'
import { showContextMenu } from '@/shared/contextMenu'
import './TabStrip.css'

function openTabMenu(e: React.MouseEvent, tab: TabDescriptor) {
  const s = useLayoutStore.getState()
  const index = s.tabs.findIndex((t) => t.id === tab.id)
  showContextMenu(e, [
    { label: 'Close', icon: 'close', onClick: () => s.closeTab(tab.id) },
    {
      label: 'Close Others',
      disabled: s.tabs.length < 2,
      onClick: () => s.closeOtherTabs(tab.id),
    },
    {
      label: 'Close to the Right',
      disabled: index === s.tabs.length - 1,
      onClick: () => s.closeTabsToRight(tab.id),
    },
    { type: 'separator' },
    { label: 'Close All', icon: 'close-all', onClick: () => s.closeAllTabs() },
  ])
}

export function TabStrip() {
  const tabs = useLayoutStore((s) => s.tabs)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const setActiveTab = useLayoutStore((s) => s.setActiveTab)
  const closeTab = useLayoutStore((s) => s.closeTab)

  return (
    <div className="tab-strip">
      {tabs.map((tab) => (
        <div
          key={tab.id}
          className={clsx('tab', tab.id === activeTabId && 'tab--active')}
          onClick={() => setActiveTab(tab.id)}
          onAuxClick={(e) => {
            if (e.button === 1) closeTab(tab.id)
          }}
          onContextMenu={(e) => openTabMenu(e, tab)}
          title={tab.title}
        >
          {tab.icon && <Icon name={tab.icon} size={14} className="tab__icon" />}
          <span className="tab__title">{tab.title}</span>
          <button
            className="tab__close"
            title="Close"
            onClick={(e) => {
              e.stopPropagation()
              closeTab(tab.id)
            }}
          >
            <Icon name="close" size={14} />
          </button>
        </div>
      ))}
    </div>
  )
}
