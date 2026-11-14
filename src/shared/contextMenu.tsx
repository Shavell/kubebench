import { useEffect, useLayoutEffect, useRef } from 'react'
import { create } from 'zustand'
import clsx from 'clsx'
import { Icon } from './Icon'
import './contextMenu.css'

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
  x: number
  y: number
  items: ContextMenuItem[]
  close: () => void
}

const useContextMenuStore = create<ContextMenuState>((set) => ({
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
export function showContextMenu(e: React.MouseEvent, items: ContextMenuItem[]): void {
  e.preventDefault()
  e.stopPropagation()
  useContextMenuStore.setState({ open: true, x: e.clientX, y: e.clientY, items })
}

/** Mounted once by the shell; renders the currently open menu, if any. */
export function ContextMenuHost() {
  const { open, x, y, items, close } = useContextMenuStore()
  const menuRef = useRef<HTMLDivElement>(null)

  useEffect(() => {
    if (!open) return
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') close()
    }
    window.addEventListener('mousedown', close)
    window.addEventListener('blur', close)
    window.addEventListener('keydown', onKeyDown)
    window.addEventListener('resize', close)
    return () => {
      window.removeEventListener('mousedown', close)
      window.removeEventListener('blur', close)
      window.removeEventListener('keydown', onKeyDown)
      window.removeEventListener('resize', close)
    }
  }, [open, close])

  // Clamp into the viewport once rendered.
  useLayoutEffect(() => {
    const menu = menuRef.current
    if (!open || !menu) return
    const rect = menu.getBoundingClientRect()
    const nx = Math.min(x, window.innerWidth - rect.width - 4)
    const ny = Math.min(y, window.innerHeight - rect.height - 4)
    menu.style.left = `${Math.max(4, nx)}px`
    menu.style.top = `${Math.max(4, ny)}px`
  }, [open, x, y, items])

  if (!open) return null

  return (
    <div
      ref={menuRef}
      className="ctx-menu"
      style={{ left: x, top: y }}
      onMouseDown={(e) => e.stopPropagation()}
      onContextMenu={(e) => e.preventDefault()}
    >
      {items.map((item, i) =>
        item.type === 'separator' ? (
          <div key={i} className="ctx-menu__separator" />
        ) : (
          <button
            key={i}
            className={clsx(
              'ctx-menu__item',
              item.danger && 'ctx-menu__item--danger',
              item.disabled && 'ctx-menu__item--disabled',
            )}
            disabled={item.disabled}
            title={item.title}
            onClick={() => {
              close()
              item.onClick()
            }}
          >
            <span className="ctx-menu__icon">{item.icon && <Icon name={item.icon} size={14} />}</span>
            {item.label}
          </button>
        ),
      )}
    </div>
  )
}
