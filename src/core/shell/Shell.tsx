import { useEffect } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { ActivityBar } from './ActivityBar'
import { CommandPalette } from './CommandPalette'
import { ContextMenuHost } from '@/shared/contextMenu'
import { SideBar } from './SideBar'
import { EditorArea } from './EditorArea'
import { BottomPanel } from './BottomPanel'
import { StatusBar } from './StatusBar'
import './Shell.css'

export function Shell() {
  const sidebarCollapsed = useLayoutStore((s) => s.sidebarCollapsed)
  const sidebarWidth = useLayoutStore((s) => s.sidebarWidth)
  const panelVisible = useLayoutStore((s) => s.panelVisible)
  const panelHeight = useLayoutStore((s) => s.panelHeight)
  const panelMaximized = useLayoutStore((s) => s.panelMaximized)

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
    </div>
  )
}
