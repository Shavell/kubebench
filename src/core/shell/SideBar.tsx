import { registry } from '@/core/plugin/registry'
import { useRegistryRevision } from '@/core/plugin/useRegistry'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useManifestText } from '@/i18n'
import { ResizeHandle } from '@/shared/ResizeHandle'
import './SideBar.css'

export function SideBar() {
  // Plugins register and get enabled/disabled at runtime; re-render on every change.
  useRegistryRevision()
  const manifestText = useManifestText()
  const activePluginId = useLayoutStore((s) => s.activePluginId)

  const view = registry.sidebarFor(activePluginId)
  if (!view) return null
  const View = view.component

  return (
    <div className="sidebar">
      {/* Uppercased in CSS, which follows <html lang> (Turkish i → İ). */}
      <div className="sidebar__header">{manifestText(activePluginId, view.title)}</div>
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
