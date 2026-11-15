import type { TFunction } from 'i18next'
import type { CommandContribution } from '@/core/plugin/types'

/** One line of the shortcuts list: a label and its key combinations (alternatives). */
export interface ShortcutEntry {
  id: string
  label: string
  /** Keybinding strings ("Ctrl+B", "ArrowUp"); several are alternatives. */
  keys: string[]
}

export interface ShortcutGroup {
  id: string
  title: string
  entries: ShortcutEntry[]
}

/** Keys handled by views themselves (tables, palette, namespace picker), by group: [label key, keys]. */
const VIEW_KEYS: Record<string, [string, string[]][]> = {
  lists: [
    ['moveRow', ['ArrowUp', 'ArrowDown']],
    ['firstLast', ['Home', 'End']],
    ['page', ['PageUp', 'PageDown']],
    ['openRow', ['Enter']],
    ['selectRow', ['Space']],
    ['rowMenu', ['Shift+F10', 'ContextMenu']],
  ],
  palette: [
    ['choose', ['ArrowUp', 'ArrowDown']],
    ['complete', ['Tab']],
    ['run', ['Enter']],
    ['close', ['Escape']],
  ],
  namespaces: [
    ['pin', ['Alt+P']],
    ['hide', ['Alt+H']],
  ],
}

/**
 * Every shortcut of the workbench, grouped: the commands with keybindings
 * (`commands`, titles already translated) and the keys views handle
 * themselves. `t` is bound to the platform namespace.
 */
export function shortcutGroups(t: TFunction, commands: readonly CommandContribution[]): ShortcutGroup[] {
  const commandEntries: ShortcutEntry[] = [
    { id: 'palette', label: t('shortcuts.entries.openPalette'), keys: ['Ctrl+Shift+P', 'F1'] },
    ...commands
      .filter((c) => c.keybinding)
      .map((c) => ({ id: c.id, label: c.title, keys: [c.keybinding!] })),
  ]
  return [
    { id: 'commands', title: t('shortcuts.groups.commands'), entries: commandEntries },
    ...Object.entries(VIEW_KEYS).map(([group, entries]) => ({
      id: group,
      title: t(`shortcuts.groups.${group}`),
      entries: entries.map(([label, keys]) => ({ id: `${group}.${label}`, label: t(`shortcuts.entries.${group}.${label}`), keys })),
    })),
  ]
}
