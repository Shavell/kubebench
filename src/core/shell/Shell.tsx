import { useEffect } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useSettingsStore } from '@/core/store/settingsStore'
import { refreshData } from '@/data/DataContext'
import { ContextMenuHost } from '@/shared/contextMenu'
import { Toaster } from '@/shared/Toaster'
import { ActivityBar } from './ActivityBar'
import { BottomPanel } from './BottomPanel'
import { CommandPalette } from './CommandPalette'
import { EditorArea } from './EditorArea'
import { SessionGuard } from './SessionGuard'
import { SideBar } from './SideBar'
import { StatusBar } from './StatusBar'
import { WorkbenchDialogs } from './WorkbenchDialogs'
import './Shell.css'

export function Shell() {
  const sidebarCollapsed = useLayoutStore((s) => s.sidebarCollapsed)
  const sidebarWidth = useLayoutStore((s) => s.sidebarWidth)
  const panelVisible = useLayoutStore((s) => s.panelVisible)
  const panelHeight = useLayoutStore((s) => s.panelHeight)
  const panelMaximized = useLayoutStore((s) => s.panelMaximized)
  const terminalFontSize = useSettingsStore((s) => s.terminalFontSize)
  const autoRefreshSeconds = useSettingsStore((s) => s.autoRefreshSeconds)

  useEffect(() => {
    document.documentElement.style.setProperty('--terminal-font-size', `${terminalFontSize}px`)
  }, [terminalFontSize])

  useEffect(() => {
    if (!autoRefreshSeconds) return
    const timer = setInterval(refreshData, autoRefreshSeconds * 1000)
    return () => clearInterval(timer)
  }, [autoRefreshSeconds])

  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      const paletteChord = (e.metaKey || e.ctrlKey) && e.shiftKey && e.code === 'KeyP'
      if (paletteChord || e.key === 'F1') {
        e.preventDefault()
        const s = useLayoutStore.getState()
        s.setPaletteOpen(!s.paletteOpen)
      }
    }
    window.addEventListener('keydown', onKeyDown, true)
    return () => window.removeEventListener('keydown', onKeyDown, true)
  }, [])

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
      <SessionGuard />
      <Toaster />
    </div>
  )
}
