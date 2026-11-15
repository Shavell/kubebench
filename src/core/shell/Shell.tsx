import { useEffect } from 'react'
import { LiveUpdates } from '@/core/live/LiveUpdates'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useSettingsStore } from '@/core/store/settingsStore'
import { UpdatesDialogs } from '@/core/updates/UpdatesDialogs'
import { ContextMenuHost } from '@/shared/contextMenu'
import { Toaster } from '@/shared/Toaster'
import { ActivityBar } from './ActivityBar'
import { BottomPanel } from './BottomPanel'
import { CommandPalette } from './CommandPalette'
import { EditorArea } from './EditorArea'
import { SessionGuard } from './SessionGuard'
import { SideBar } from './SideBar'
import { StatusBar } from './StatusBar'
import { TabCloseGuard } from './TabCloseGuard'
import { useGlobalShortcuts } from './useGlobalShortcuts'
import { WorkbenchDialogs } from './WorkbenchDialogs'
import './Shell.css'

export function Shell() {
  const sidebarCollapsed = useLayoutStore((s) => s.sidebarCollapsed)
  const sidebarWidth = useLayoutStore((s) => s.sidebarWidth)
  const panelVisible = useLayoutStore((s) => s.panelVisible)
  const panelHeight = useLayoutStore((s) => s.panelHeight)
  const panelMaximized = useLayoutStore((s) => s.panelMaximized)
  const terminalFontSize = useSettingsStore((s) => s.terminalFontSize)

  useEffect(() => {
    document.documentElement.style.setProperty('--terminal-font-size', `${terminalFontSize}px`)
  }, [terminalFontSize])

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
          {!(panelVisible && panelMaximized) && <EditorArea />}
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
