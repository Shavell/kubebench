import clsx from 'clsx'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { TabDescriptor } from '@/core/plugin/types'
import { Icon } from '@/shared/Icon'
import { showContextMenu } from '@/shared/contextMenu'
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

/**
 * The tab's title in the current language: tabs opened with a `titleKey`
 * are translated on every render; others keep the title they were opened with.
 */
function tabTitle(tab: TabDescriptor, t: TFunction): string {
  if (!tab.titleKey) return tab.title
  return t(tab.titleKey, { ...tab.titleValues, ns: tab.titleNs, defaultValue: tab.title })
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
          className={clsx('tab', tab.id === activeTabId && 'tab--active')}
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
            title={t('common.close')}
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
