import type { KeyboardEvent } from 'react'

/** Elements that can take keyboard focus; invisible ones are filtered out at runtime. */
const FOCUSABLE = [
  'a[href]',
  'button:not([disabled])',
  'input:not([disabled]):not([type="hidden"])',
  'select:not([disabled])',
  'textarea:not([disabled])',
  '[tabindex]:not([tabindex="-1"])',
  '[contenteditable="true"]',
].join(',')

/** Form fields: preferred over buttons when a dialog opens. */
const FIELDS = 'input:not([disabled]):not([type="hidden"]), select:not([disabled]), textarea:not([disabled])'

const isVisible = (el: HTMLElement) => el.getClientRects().length > 0

function focusableIn(root: HTMLElement, selector = FOCUSABLE): HTMLElement[] {
  return [...root.querySelectorAll<HTMLElement>(selector)].filter(isVisible)
}

/**
 * Where focus goes when a dialog opens: an element marked `data-autofocus`,
 * else the first form field of the content (body and footer), else its
 * first focusable element, else the dialog panel itself. The header close
 * button is never the initial target.
 */
export function initialFocusTarget(panel: HTMLElement): HTMLElement {
  const marked = panel.querySelector<HTMLElement>('[data-autofocus]')
  if (marked) return marked
  const content = panel.querySelector<HTMLElement>('.kb-dialog__content') ?? panel
  return focusableIn(content, FIELDS)[0] ?? focusableIn(content)[0] ?? panel
}

/**
 * Keeps Tab and Shift+Tab inside `panel`: moving past the last focusable
 * element wraps to the first one and vice versa.
 */
export function trapTabKey(event: KeyboardEvent, panel: HTMLElement): void {
  const items = focusableIn(panel)
  const active = document.activeElement
  const outside = !(active instanceof Node) || !panel.contains(active)
  if (items.length === 0) {
    event.preventDefault()
    panel.focus()
    return
  }
  const first = items[0]
  const last = items[items.length - 1]
  if (event.shiftKey && (outside || active === first || active === panel)) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && (outside || active === last)) {
    event.preventDefault()
    first.focus()
  }
}
