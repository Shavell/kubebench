import { useEffect, useLayoutEffect, useRef } from 'react'
import { useTranslation } from 'react-i18next'
import { allCommands } from '@/core/commands/coreCommands'
import { isPlainKey, matchesKeybinding } from '@/core/shortcuts/keybinding'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useSettingsStore } from '@/core/store/settingsStore'
import { useManifestText } from '@/i18n'

/** Fields where typed characters belong to the field (plain-key shortcuts stay off there). */
function isTyping(target: EventTarget | null): boolean {
  if (!(target instanceof HTMLElement)) return false
  if (target.isContentEditable || target.closest('.monaco-editor')) return true
  if (target instanceof HTMLTextAreaElement || target instanceof HTMLSelectElement) return true
  return target instanceof HTMLInputElement && !['checkbox', 'radio', 'button', 'submit', 'reset'].includes(target.type)
}

/**
 * Binds the workbench's keyboard shortcuts: Ctrl+Shift+P / F1 always
 * toggle the palette; command keybindings (core and plugin commands) fire
 * unless shortcuts are turned off in Settings, a dialog or the palette is
 * open, or the focus is in a terminal (which needs Ctrl keys itself).
 * Plain keys such as `?` and `/` never fire while typing in a field.
 */
export function useGlobalShortcuts(): void {
  const { t } = useTranslation()
  const manifestText = useManifestText()

  // The palette chord, in the capture phase so no view can swallow it.
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

  // The current translators, so the listener below is set up once.
  const text = useRef({ t, manifestText })
  useLayoutEffect(() => {
    text.current = { t, manifestText }
  })

  // Command keybindings, in the bubble phase: views that handle a key themselves come first.
  useEffect(() => {
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.defaultPrevented || e.isComposing || e.repeat) return
      if (!useSettingsStore.getState().shortcutsEnabled || useLayoutStore.getState().paletteOpen) return
      if (document.querySelector('[aria-modal="true"]')) return
      const target = e.target instanceof HTMLElement ? e.target : null
      if (target?.closest('.xterm')) return
      const commands = allCommands(text.current.t, text.current.manifestText)
      const command = commands.find((c) => c.keybinding && matchesKeybinding(e, c.keybinding))
      if (!command?.keybinding || (isPlainKey(command.keybinding) && isTyping(target))) return
      e.preventDefault()
      command.run()
    }
    window.addEventListener('keydown', onKeyDown)
    return () => window.removeEventListener('keydown', onKeyDown)
  }, [])
}
