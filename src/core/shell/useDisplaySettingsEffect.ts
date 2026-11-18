import { useEffect } from 'react'
import { useDisplaySettings } from '@/core/store/displayStore'
import { fontStack } from '@/shared/fontFamily'

/**
 * Applies the user's interface scale and terminal font to the document:
 * the scale as CSS `zoom` of <body> (everything, dialogs and menus
 * included, scales together; see shared/zoom.ts), the terminal font as the
 * `--terminal-font-*` tokens of the log views. Monaco and xterm.js read
 * their fonts themselves (they paint on canvases); xterm.js does not
 * handle zoom, so its screen cancels the scale and enlarges its font
 * instead (terminal plugin, TerminalSession).
 */
export function useDisplaySettingsEffect(): void {
  const { uiScale, terminalFontFamily, terminalFontSize } = useDisplaySettings()

  useEffect(() => {
    const body = document.body.style
    body.zoom = uiScale === 100 ? '' : String(uiScale / 100)
    return () => {
      body.zoom = ''
    }
  }, [uiScale])

  useEffect(() => {
    const root = document.documentElement.style
    root.setProperty('--terminal-font-size', `${terminalFontSize}px`)
    root.setProperty('--terminal-font-family', fontStack(terminalFontFamily, 'var(--font-mono)'))
  }, [terminalFontFamily, terminalFontSize])
}
