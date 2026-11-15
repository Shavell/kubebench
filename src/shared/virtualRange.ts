/**
 * Row windowing for long tables (pure, so it can be unit tested without a
 * DOM). Rows have one fixed height; the table body starts `bodyOffset` px
 * below the top of the scroll container's content.
 */
export interface VirtualViewport {
  /** scrollTop of the scroll container. */
  scrollTop: number
  /** clientHeight of the scroll container. */
  viewportHeight: number
  /** Distance from the container's content top to the first body row. */
  bodyOffset: number
  rowHeight: number
  count: number
  /** Extra rows rendered above and below the visible ones. */
  overscan?: number
}

/** Half-open index range [start, end) of the rows to render. */
export interface RowRange {
  start: number
  end: number
}

export function visibleRange({
  scrollTop,
  viewportHeight,
  bodyOffset,
  rowHeight,
  count,
  overscan = 12,
}: VirtualViewport): RowRange {
  if (count <= 0 || rowHeight <= 0) return { start: 0, end: 0 }
  const top = scrollTop - bodyOffset
  const first = Math.floor(top / rowHeight)
  const last = Math.ceil((top + viewportHeight) / rowHeight)
  const start = clamp(first - overscan, 0, count)
  const end = clamp(last + overscan, start, count)
  return { start, end }
}

/**
 * The scrollTop that brings row `index` fully into view below a sticky
 * header of `stickyTop` px, or null when it is already visible.
 */
export function scrollTopFor(
  index: number,
  { scrollTop, viewportHeight, bodyOffset, rowHeight }: Omit<VirtualViewport, 'count' | 'overscan'>,
  stickyTop = 0,
): number | null {
  const rowTop = bodyOffset + index * rowHeight
  const rowBottom = rowTop + rowHeight
  if (rowTop < scrollTop + stickyTop) return Math.max(0, rowTop - stickyTop)
  if (rowBottom > scrollTop + viewportHeight) return rowBottom - viewportHeight
  return null
}

/**
 * Index after moving `delta` rows from `current` (-1: none yet), kept inside [0, count).
 * A move of at least `count` rows (Home/End) goes to that end, also from no row.
 */
export function moveIndex(current: number, delta: number, count: number): number {
  if (count === 0) return -1
  if (Math.abs(delta) >= count) return delta < 0 ? 0 : count - 1
  if (current < 0) return delta < 0 ? count - 1 : 0
  return clamp(current + delta, 0, count - 1)
}

function clamp(value: number, min: number, max: number): number {
  return Math.min(max, Math.max(min, value))
}
