import type { Navigation } from '../data/types.ts'

/** What the tab learns from one load event of its frame. */
export interface FrameLoad {
  /** The location of the frame's document, or null when it is unknown. */
  location: string | null
  /** The newest navigation seen, the next load's starting point. */
  seen: number
}

/**
 * The location of the document the frame just loaded, from the session's
 * navigations: the first one recorded after `seen` (the newest navigation
 * of the previous load). The frame's own document is requested before the
 * frames nested in its page, whose loads the load event waits for, so the
 * later navigations of the same load are theirs. A nested frame that
 * navigates on its own between two loads of the frame is taken for the
 * frame's document at the next load.
 */
export function frameLocation(navigations: Navigation[], seen: number): FrameLoad {
  const next = navigations.find((n) => n.seq > seen)
  if (!next) return { location: null, seen }
  return { location: next.location || null, seen: navigations[navigations.length - 1].seq }
}
