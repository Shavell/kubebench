import { useTranslation } from 'react-i18next'
import { registry } from '@/core/plugin/registry'
import { useRegistryRevision } from '@/core/plugin/useRegistry'
import { useLayoutStore } from '@/core/store/layoutStore'
import { TabStrip } from './TabStrip'
import { Icon } from '@/shared/Icon'
import './EditorArea.css'

export function EditorArea() {
  // Plugins register and get enabled/disabled at runtime; re-render on every change.
  useRegistryRevision()
  const { t } = useTranslation()
  const tabs = useLayoutStore((s) => s.tabs)
  const activeTabId = useLayoutStore((s) => s.activeTabId)

  const activeTab = tabs.find((x) => x.id === activeTabId) ?? null
  const Renderer = activeTab ? registry.tabRenderer(activeTab.kind) : null

  return (
    <div className="editor-area">
      {tabs.length > 0 && <TabStrip />}
      <div className="editor-area__body">
        {activeTab && Renderer ? (
          <Renderer tab={activeTab} />
        ) : (
          <div className="editor-area__watermark">
            <Icon name="dashboard" size={72} />
            <div className="editor-area__watermark-title">KubeBench</div>
            <div className="editor-area__watermark-hints">
              <div>{t('shell.editor.openHint')}</div>
              <div>{t('shell.editor.modulesHint')}</div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
