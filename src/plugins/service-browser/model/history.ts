/**
 * The browser tab's own navigation history. The proxied page lives in a
 * cross-origin frame whose history KubeBench cannot drive, so the tab
 * records the locations the frame reported and navigates to them itself.
 */
export interface NavHistory {
  entries: string[]
  index: number
}

const MAX_ENTRIES = 100

export function startHistory(path: string): NavHistory {
  return { entries: [path], index: 0 }
}

export function currentPath(h: NavHistory): string {
  return h.entries[h.index]
}

/**
 * Records that the frame shows `path`. Showing the current entry again
 * (a reload, or going back to it) changes nothing; anything else drops
 * the forward entries and appends `path`.
 */
export function visit(h: NavHistory, path: string): NavHistory {
  if (h.entries[h.index] === path) return h
  const entries = [...h.entries.slice(0, h.index + 1), path].slice(-MAX_ENTRIES)
  return { entries, index: entries.length - 1 }
}

export function canGoBack(h: NavHistory): boolean {
  return h.index > 0
}

export function canGoForward(h: NavHistory): boolean {
  return h.index < h.entries.length - 1
}

export function goBack(h: NavHistory): NavHistory {
  return canGoBack(h) ? { ...h, index: h.index - 1 } : h
}

export function goForward(h: NavHistory): NavHistory {
  return canGoForward(h) ? { ...h, index: h.index + 1 } : h
}
