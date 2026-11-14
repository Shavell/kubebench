import clsx from 'clsx'
import { useEffect, useLayoutEffect, useRef, type KeyboardEvent, type ReactNode } from 'react'
import { createPortal } from 'react-dom'
import { Icon } from '@/shared/Icon'

export interface StatusMenuItem {
  type?: 'item'
  id: string
  label: string
  icon?: string
  /** Muted text after the label, e.g. an environment or why the item is disabled. */
  detail?: string
  /** Shows a check mark (e.g. the active cluster); the item becomes a menuitemradio. */
  checked?: boolean
  disabled?: boolean
  onSelect: () => void
}

export type StatusMenuEntry = StatusMenuItem | { type: 'separator'; id: string }

interface StatusMenuProps {
  /** The status bar button that opened the menu; the menu sits above it. */
  anchor: HTMLElement
  /** Accessible name of the menu. */
  label: string
  /** Non-interactive text at the top, e.g. "Signed in as alice". */
  header?: ReactNode
  entries: StatusMenuEntry[]
  onClose: () => void
}

const ITEM_SELECTOR = '[role^="menuitem"]:not(:disabled)'

/**
 * Pop-up menu of a status bar item. Arrow keys, Home and End move between
 * items, Esc or Tab closes it and focus returns to the status bar button.
 */
export function StatusMenu({ anchor, label, header, entries, onClose }: StatusMenuProps) {
  const menuRef = useRef<HTMLDivElement>(null)

  // Above the anchor, kept inside the window.
  useLayoutEffect(() => {
    const menu = menuRef.current
    if (!menu) return
    const rect = anchor.getBoundingClientRect()
    const left = Math.min(rect.left, window.innerWidth - menu.offsetWidth - 4)
    menu.style.left = `${Math.max(4, left)}px`
    menu.style.bottom = `${window.innerHeight - rect.top + 2}px`
  }, [anchor, entries])

  // The latest onClose, so the listeners below are set up once per opening.
  const onCloseRef = useRef(onClose)
  useLayoutEffect(() => {
    onCloseRef.current = onClose
  })

  useEffect(() => {
    menuRef.current?.querySelector<HTMLElement>(ITEM_SELECTOR)?.focus()
    const close = () => onCloseRef.current()
    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as Node
      // A press on the anchor is left to its own click handler (it toggles the menu).
      if (!menuRef.current?.contains(target) && !anchor.contains(target)) close()
    }
    window.addEventListener('mousedown', onPointerDown)
    window.addEventListener('resize', close)
    window.addEventListener('blur', close)
    return () => {
      window.removeEventListener('mousedown', onPointerDown)
      window.removeEventListener('resize', close)
      window.removeEventListener('blur', close)
    }
  }, [anchor])

  const closeAndReturnFocus = () => {
    onClose()
    anchor.focus()
  }

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const items = [...(menuRef.current?.querySelectorAll<HTMLElement>(ITEM_SELECTOR) ?? [])]
    const index = items.indexOf(document.activeElement as HTMLElement)
    let next: HTMLElement | undefined
    if (event.key === 'ArrowDown') next = items[(index + 1) % items.length]
    else if (event.key === 'ArrowUp') next = items[(index - 1 + items.length) % items.length]
    else if (event.key === 'Home') next = items[0]
    else if (event.key === 'End') next = items[items.length - 1]
    else if (event.key === 'Escape') {
      event.preventDefault()
      closeAndReturnFocus()
      return
    } else if (event.key === 'Tab') {
      onClose()
      return
    }
    if (next) {
      event.preventDefault()
      next.focus()
    }
  }

  // The header stays outside the role="menu" element, which may only own items and separators.
  return createPortal(
    <div ref={menuRef} className="status-menu" onKeyDown={onKeyDown}>
      {header && <div className="status-menu__header">{header}</div>}
      <div role="menu" aria-label={label}>
        {entries.map((entry) =>
          entry.type === 'separator' ? (
            <div key={entry.id} className="status-menu__separator" role="separator" />
          ) : (
            <button
              key={entry.id}
              type="button"
              role={entry.checked === undefined ? 'menuitem' : 'menuitemradio'}
              aria-checked={entry.checked}
              disabled={entry.disabled}
              className={clsx('status-menu__item', entry.checked && 'status-menu__item--checked')}
              onClick={() => {
                closeAndReturnFocus()
                entry.onSelect()
              }}
            >
              <span className="status-menu__icon">
                {entry.checked ? <Icon name="check" size={14} /> : entry.icon && <Icon name={entry.icon} size={14} />}
              </span>
              <span className="status-menu__label">{entry.label}</span>
              {entry.detail && <span className="status-menu__detail">{entry.detail}</span>}
            </button>
          ),
        )}
      </div>
    </div>,
    document.body,
  )
}
