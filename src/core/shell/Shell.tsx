import { LiveUpdates } from '@/core/live/LiveUpdates'
import { useLayoutStore } from '@/core/store/layoutStore'
import { UpdatesDialogs } from '@/core/updates/UpdatesDialogs'
import { QueryActivityContext } from '@/data/DataContext'
import { ContextMenuHost } from '@/shared/ContextMenuHost'
import { Toaster } from '@/shared/Toaster'
import { ActivityBar } from './ActivityBar'
import { BottomPanel } from './BottomPanel'
import { CommandPalette } from './CommandPalette'
import { EditorArea } from './EditorArea'
import { SessionGuard } from './SessionGuard'
import { SideBar } from './SideBar'
import { SideEditorGroup } from './SideEditorGroup'
import { StatusBar } from './StatusBar'
import { TabCloseGuard } from './TabCloseGuard'
import { useDisplaySettingsEffect } from './useDisplaySettingsEffect'
import { useGlobalShortcuts } from './useGlobalShortcuts'
import { WorkbenchDialogs } from './WorkbenchDialogs'
import './Shell.css'

export function Shell() {
  const sidebarCollapsed = useLayoutStore((s) => s.sidebarCollapsed)
  const sidebarWidth = useLayoutStore((s) => s.sidebarWidth)
  const panelVisible = useLayoutStore((s) => s.panelVisible)
  const panelHeight = useLayoutStore((s) => s.panelHeight)
  const panelMaximized = useLayoutStore((s) => s.panelMaximized)

  useDisplaySettingsEffect()
  useGlobalShortcuts()

  return (
    <div className="shell">
      <div className="shell__body">
        <ActivityBar />
        {!sidebarCollapsed && (
          <div className="shell__sidebar" style={{ width: sidebarWidth }}>
            <SideBar />
          </div>
        )}
        <div className="shell__main">
          {/* Hidden, not unmounted, under a maximized panel: open tabs (a
              service browser's page and history) keep their state; their
              queries wait for live updates until shown. */}
          <div className="shell__editors" hidden={panelVisible && panelMaximized}>
            <QueryActivityContext value={!(panelVisible && panelMaximized)}>
              <EditorArea />
              <SideEditorGroup />
            </QueryActivityContext>
          </div>
          {panelVisible && (
            <div
              className="shell__panel"
              style={panelMaximized ? { flex: 1 } : { height: panelHeight }}
            >
              <BottomPanel />
            </div>
          )}
        </div>
      </div>
      <StatusBar />
      <CommandPalette />
      <ContextMenuHost />
      <WorkbenchDialogs />
      <UpdatesDialogs />
      <TabCloseGuard />
      <SessionGuard />
      <Toaster />
      {/* Live updates and the auto-refresh timer (design §7.10). */}
      <LiveUpdates />
    </div>
  )
}
