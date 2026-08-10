import { registry } from '@/core/plugin/registry'
import { useLayoutStore } from '@/core/store/layoutStore'
import { ResizeHandle } from '@/shared/ResizeHandle'
import './SideBar.css'

export function SideBar() {
  const activePluginId = useLayoutStore((s) => s.activePluginId)

  const view = registry.sidebarFor(activePluginId)
  if (!view) return null
  const View = view.component

  return (
    <div className="sidebar">
      <div className="sidebar__header">{view.title.toUpperCase()}</div>
      <div className="sidebar__content">
        <View />
      </div>
      <ResizeHandle
        orientation="vertical"
        onResize={(delta) => {
          const s = useLayoutStore.getState()
          s.setSidebarWidth(s.sidebarWidth + delta)
        }}
      />
    </div>
  )
}
