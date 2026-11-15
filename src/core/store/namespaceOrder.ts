/**
 * Ordering of namespace pickers by the user's favourites (pure, unit tested
 * in namespaceOrder.test.ts): pinned namespaces first in the order they were
 * pinned, then the rest alphabetically; hidden ones are left out unless
 * asked for, or currently selected (a filter in use never vanishes).
 */
export interface NamespacePrefs {
  pinned: string[]
  hidden: string[]
}

export interface OrderedNamespaces {
  pinned: string[]
  others: string[]
}

export const EMPTY_PREFS: NamespacePrefs = { pinned: [], hidden: [] }

export function orderNamespaces(
  names: readonly string[],
  prefs: NamespacePrefs,
  options: { includeHidden?: boolean; keep?: string } = {},
): OrderedNamespaces {
  const present = new Set(names)
  const hidden = new Set(prefs.hidden)
  const shown = (name: string) => options.includeHidden || !hidden.has(name) || name === options.keep
  const pinned = prefs.pinned.filter((name) => present.has(name) && shown(name))
  const pinnedSet = new Set(pinned)
  const others = names.filter((name) => !pinnedSet.has(name) && shown(name)).sort((a, b) => a.localeCompare(b))
  return { pinned, others }
}

/** Adds `name` to `list` when missing, removes it otherwise. */
export function toggled(list: readonly string[], name: string): string[] {
  return list.includes(name) ? list.filter((n) => n !== name) : [...list, name]
}
