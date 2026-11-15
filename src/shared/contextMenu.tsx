import { useEffect, useLayoutEffect, useRef } from 'react'
import { create } from 'zustand'
import clsx from 'clsx'
import { Icon } from './Icon'
import './contextMenu.css'

const ITEM_SELECTOR = '.ctx-menu__item:not(:disabled)'

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

  // Keyboard use: the first item takes focus, Esc gives it back to the opener.
  const opener = useRef<HTMLElement | null>(null)
  const restoreFocus = () => {
    if (opener.current?.isConnected) opener.current.focus()
    opener.current = null
  }
  useEffect(() => {
    if (!open) return
    opener.current = document.activeElement instanceof HTMLElement ? document.activeElement : null
    menuRef.current?.querySelector<HTMLElement>(ITEM_SELECTOR)?.focus()
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key === 'Escape') {
        restoreFocus()
        close()
      }
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

  /** ↑/↓, Home and End move between the enabled items. */
  const onMenuKeyDown = (e: React.KeyboardEvent<HTMLDivElement>) => {
    const items = [...(menuRef.current?.querySelectorAll<HTMLElement>(ITEM_SELECTOR) ?? [])]
    if (items.length === 0) return
    const index = items.indexOf(document.activeElement as HTMLElement)
    const next =
      e.key === 'ArrowDown'
        ? items[(index + 1) % items.length]
        : e.key === 'ArrowUp'
          ? items[(index - 1 + items.length) % items.length]
          : e.key === 'Home'
            ? items[0]
            : e.key === 'End'
              ? items[items.length - 1]
              : undefined
    if (next) {
      e.preventDefault()
      next.focus()
    } else if (e.key === 'Tab') {
      e.preventDefault()
    }
  }

  return (
    <div
      ref={menuRef}
      className="ctx-menu"
      role="menu"
      style={{ left: x, top: y }}
      onMouseDown={(e) => e.stopPropagation()}
      onContextMenu={(e) => e.preventDefault()}
      onKeyDown={onMenuKeyDown}
    >
      {items.map((item, i) =>
        item.type === 'separator' ? (
          <div key={i} className="ctx-menu__separator" role="separator" />
        ) : (
          <button
            key={i}
            type="button"
            role="menuitem"
            className={clsx(
              'ctx-menu__item',
              item.danger && 'ctx-menu__item--danger',
              item.disabled && 'ctx-menu__item--disabled',
            )}
            disabled={item.disabled}
            title={item.title}
            // The pointer and the keyboard share one highlight: the focused item.
            onMouseEnter={(e) => !item.disabled && e.currentTarget.focus()}
            onClick={() => {
              // Back to the opener first, so a dialog the item opens returns focus there.
              restoreFocus()
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
