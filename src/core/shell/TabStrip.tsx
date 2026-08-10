import clsx from 'clsx'
import { useLayoutStore } from '@/core/store/layoutStore'
import { Icon } from '@/shared/Icon'
import './TabStrip.css'

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
