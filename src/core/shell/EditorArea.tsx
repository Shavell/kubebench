import { registry } from '@/core/plugin/registry'
import { useLayoutStore } from '@/core/store/layoutStore'
import { TabStrip } from './TabStrip'
import { Icon } from '@/shared/Icon'
import './EditorArea.css'

export function EditorArea() {
  const tabs = useLayoutStore((s) => s.tabs)
  const activeTabId = useLayoutStore((s) => s.activeTabId)

  const activeTab = tabs.find((t) => t.id === activeTabId) ?? null
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
              <div>Select a resource from the sidebar to open it</div>
              <div>
                Switch modules from the activity bar — Workloads, CRDs, Helm, Metrics
              </div>
            </div>
          </div>
        )}
      </div>
    </div>
  )
}
