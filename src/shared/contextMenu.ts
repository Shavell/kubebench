import type { MouseEvent } from 'react'
import { create } from 'zustand'
import { uiZoom } from './zoom'

export type ContextMenuItem =
  | {
      type?: 'item'
      label: string
      icon?: string
      danger?: boolean
      disabled?: boolean
      /** Tooltip, e.g. why a disabled item cannot be used. */
      title?: string
      onClick: () => void
    }
  | { type: 'separator' }

interface ContextMenuState {
  open: boolean
  /** Pointer position in the page's CSS pixels (see ./zoom). */
  x: number
  y: number
  items: ContextMenuItem[]
  close: () => void
}

/** The open menu; rendered by `ContextMenuHost`. */
export const useContextMenuStore = create<ContextMenuState>((set) => ({
  open: false,
  x: 0,
  y: 0,
  items: [],
  close: () => set({ open: false }),
}))

/**
 * Open a custom context menu at the mouse position. Call from any
 * onContextMenu handler:
 *
 *   onContextMenu={(e) => showContextMenu(e, [{label: "Close", onClick: ...}])}
 */
export function showContextMenu(e: MouseEvent, items: ContextMenuItem[]): void {
  e.preventDefault()
  e.stopPropagation()
  const zoom = uiZoom()
  useContextMenuStore.setState({ open: true, x: e.clientX / zoom, y: e.clientY / zoom, items })
}

