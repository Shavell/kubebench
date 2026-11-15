/**
 * Keyboard shortcut strings such as "Ctrl+Shift+P", "Alt+N", "Ctrl+`" or
 * "?" (pure, unit tested in keybinding.test.ts). "Ctrl" also matches the
 * Cmd key, so one binding serves Linux, Windows and macOS.
 *
 * Letters and digits match the physical key (`event.code`), so Alt+N works
 * on macOS, where Option+N types a dead key; other keys match the
 * character they type (`event.key`), so "?" works on any keyboard layout.
 */
export interface Keybinding {
  ctrl: boolean
  shift: boolean
  alt: boolean
  /** The key without modifiers, e.g. "P", "F1", "PageDown", "`", "?". */
  key: string
}

/** The subset of KeyboardEvent a binding is matched against. */
export interface KeyStroke {
  key: string
  code: string
  ctrlKey: boolean
  metaKey: boolean
  shiftKey: boolean
  altKey: boolean
}

export function parseKeybinding(binding: string): Keybinding {
  // "Ctrl++" names the plus key; otherwise '+' separates the parts.
  const parts = binding.endsWith('++') ? [...binding.slice(0, -2).split('+'), '+'] : binding.split('+')
  const key = parts.pop() ?? ''
  const modifiers = new Set(parts.map((p) => p.toLowerCase()))
  return { ctrl: modifiers.has('ctrl'), shift: modifiers.has('shift'), alt: modifiers.has('alt'), key }
}

const PHYSICAL: Record<string, string> = { '`': 'Backquote' }

/** The event.code a binding's key is matched by, or null to match event.key. */
function codeOf(key: string): string | null {
  if (/^[a-z]$/i.test(key)) return `Key${key.toUpperCase()}`
  if (/^[0-9]$/.test(key)) return `Digit${key}`
  return PHYSICAL[key] ?? null
}

export function matchesKeybinding(event: KeyStroke, binding: string | Keybinding): boolean {
  const b = typeof binding === 'string' ? parseKeybinding(binding) : binding
  if ((event.ctrlKey || event.metaKey) !== b.ctrl || event.altKey !== b.alt) return false
  const code = codeOf(b.key)
  if (code) return event.code === code && event.shiftKey === b.shift
  // Characters such as "?" need Shift on most layouts, so Shift is only
  // checked for named keys (F1, PageDown, …).
  const named = b.key.length > 1
  if (named && event.shiftKey !== b.shift) return false
  return event.key.toLowerCase() === b.key.toLowerCase()
}

/** Whether the binding is a plain key (no Ctrl/Alt), which must not fire while typing. */
export function isPlainKey(binding: string | Keybinding): boolean {
  const b = typeof binding === 'string' ? parseKeybinding(binding) : binding
  return !b.ctrl && !b.alt
}

/** Key caps for display, e.g. ["⌘", "⇧", "P"] on macOS or ["Ctrl", "Shift", "P"] elsewhere. */
export function keyCaps(binding: string, mac: boolean): string[] {
  const b = parseKeybinding(binding)
  const caps: string[] = []
  if (b.ctrl) caps.push(mac ? '⌘' : 'Ctrl')
  if (b.alt) caps.push(mac ? '⌥' : 'Alt')
  if (b.shift) caps.push(mac ? '⇧' : 'Shift')
  caps.push(b.key.length === 1 ? b.key.toUpperCase() : b.key)
  return caps
}
