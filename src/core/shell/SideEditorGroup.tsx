import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import { registry } from '@/core/plugin/registry'
import { useRegistryRevision } from '@/core/plugin/useRegistry'
import { useSideGroupStore } from '@/core/store/sideGroupStore'
import { Icon } from '@/shared/Icon'
import { ResizeHandle } from '@/shared/ResizeHandle'
import { showContextMenu } from '@/shared/contextMenu'
import { tabTitle } from './tabTitle'
import './TabStrip.css'
import './SideEditorGroup.css'

/**
 * The side editor group (design §7.14) to the right of the editor area,
 * shown while it has tabs; its left edge resizes it. Every tab stays
 * mounted (inactive ones hidden), so a browser page keeps its state.
 */
export function SideEditorGroup() {
  useRegistryRevision()
  const { t } = useTranslation()
  const tabs = useSideGroupStore((s) => s.tabs)
  const activeTabId = useSideGroupStore((s) => s.activeTabId)
  const width = useSideGroupStore((s) => s.width)
  const { setActive, close, closeAll, setWidth } = useSideGroupStore.getState()

  if (tabs.length === 0) return null

  return (
    <div className="side-group" style={{ width }} aria-label={t('shell.sideGroup.label')}>
      <ResizeHandle orientation="vertical" onResize={(delta) => setWidth(useSideGroupStore.getState().width - delta)} />
      <div className="tab-strip">
        {tabs.map((tab) => (
          <div
            key={tab.id}
            className={clsx('tab', tab.id === activeTabId && 'tab--active')}
            onClick={() => setActive(tab.id)}
            onAuxClick={(e) => {
              if (e.button === 1) close(tab.id)
            }}
            onContextMenu={(e) =>
              showContextMenu(e, [
                { label: t('common.close'), icon: 'close', onClick: () => close(tab.id) },
                { label: t('shell.tabs.closeAll'), icon: 'close-all', onClick: closeAll },
              ])
            }
            title={tabTitle(tab, t)}
          >
            {tab.icon && <Icon name={tab.icon} size={14} className="tab__icon" />}
            <span className="tab__title">{tabTitle(tab, t)}</span>
            <button
              className="tab__close"
              title={t('common.close')}
              onClick={(e) => {
                e.stopPropagation()
                close(tab.id)
              }}
            >
              <Icon name="close" size={14} className="tab__close-icon" />
            </button>
          </div>
        ))}
      </div>
      <div className="side-group__body">
        {tabs.map((tab) => {
          const Renderer = registry.tabRenderer(tab.kind)
          return (
            <div key={tab.id} className="side-group__page" hidden={tab.id !== activeTabId}>
              {Renderer && <Renderer tab={tab} />}
            </div>
          )
        })}
      </div>
    </div>
  )
}
