import clsx from 'clsx'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { TabDescriptor } from '@/core/plugin/types'
import { Icon } from '@/shared/Icon'
import { showContextMenu } from '@/shared/contextMenu'
import { tabTitle } from './tabTitle'
import './TabStrip.css'

function openTabMenu(e: React.MouseEvent, tab: TabDescriptor, t: TFunction) {
  const s = useLayoutStore.getState()
  const index = s.tabs.findIndex((x) => x.id === tab.id)
  showContextMenu(e, [
    { label: t('common.close'), icon: 'close', onClick: () => s.closeTab(tab.id) },
    {
      label: t('shell.tabs.closeOthers'),
      disabled: s.tabs.length < 2,
      onClick: () => s.closeOtherTabs(tab.id),
    },
    {
      label: t('shell.tabs.closeToRight'),
      disabled: index === s.tabs.length - 1,
      onClick: () => s.closeTabsToRight(tab.id),
    },
    { type: 'separator' },
    { label: t('shell.tabs.closeAll'), icon: 'close-all', onClick: () => s.closeAllTabs() },
  ])
}

export function TabStrip() {
  const { t } = useTranslation()
  const tabs = useLayoutStore((s) => s.tabs)
  const activeTabId = useLayoutStore((s) => s.activeTabId)
  const setActiveTab = useLayoutStore((s) => s.setActiveTab)
  const closeTab = useLayoutStore((s) => s.closeTab)

  return (
    <div className="tab-strip">
      {tabs.map((tab) => (
        <div
          key={tab.id}
          className={clsx('tab', tab.id === activeTabId && 'tab--active', tab.dirty && 'tab--dirty')}
          onClick={() => setActiveTab(tab.id)}
          onAuxClick={(e) => {
            if (e.button === 1) closeTab(tab.id)
          }}
          onContextMenu={(e) => openTabMenu(e, tab, t)}
          title={tabTitle(tab, t)}
        >
          {tab.icon && <Icon name={tab.icon} size={14} className="tab__icon" />}
          <span className="tab__title">{tabTitle(tab, t)}</span>
          <button
            className="tab__close"
            title={tab.dirty ? t('shell.tabs.unsaved.closeDirty') : t('common.close')}
            onClick={(e) => {
              e.stopPropagation()
              closeTab(tab.id)
            }}
          >
            {tab.dirty && <Icon name="circle-filled" size={10} className="tab__dirty" />}
            <Icon name="close" size={14} className="tab__close-icon" />
          </button>
        </div>
      ))}
    </div>
  )
}
