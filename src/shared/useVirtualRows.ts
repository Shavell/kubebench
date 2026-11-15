import { useCallback, useLayoutEffect, useRef, useState, type RefObject } from 'react'
import { scrollTopFor, visibleRange, type RowRange } from './virtualRange'

/** Height of a table row before one was measured (13px text, 5px padding, border). */
const DEFAULT_ROW_HEIGHT = 27

interface Geometry {
  scrollTop: number
  viewportHeight: number
  /** First body row's distance from the container's content top. */
  bodyOffset: number
  /** Height of the sticky header, which covers the top of the viewport. */
  headerHeight: number
}

/** The nearest ancestor that scrolls vertically (e.g. `.page__scroll`), if any. */
function scrollParent(element: HTMLElement): HTMLElement | null {
  for (let node = element.parentElement; node; node = node.parentElement) {
    const { overflowY } = getComputedStyle(node)
    if (overflowY === 'auto' || overflowY === 'scroll') return node
  }
  return null
}

function measure(table: HTMLTableElement, container: HTMLElement): Geometry {
  const headerHeight = table.tHead?.offsetHeight ?? 0
  const tableTop = table.getBoundingClientRect().top - container.getBoundingClientRect().top + container.scrollTop
  return {
    scrollTop: container.scrollTop,
    viewportHeight: container.clientHeight,
    bodyOffset: tableTop + headerHeight,
    headerHeight,
  }
}

export interface VirtualRows {
  /** Rows to render; the whole list while windowing is off. */
  range: RowRange
  /** Whether rows outside `range` are replaced by spacer rows. */
  windowed: boolean
  rowHeight: number
  /** Scrolls the container so that row `index` is visible below the sticky header. */
  scrollToIndex: (index: number) => void
}

/**
 * Renders only the rows of a long table that are in (or near) its scroll
 * container's viewport. Rows are assumed to share one height, measured from
 * the first rendered row. Without a scrolling ancestor, or while `enabled`
 * is false, every row is rendered.
 */
export function useVirtualRows(tableRef: RefObject<HTMLTableElement | null>, count: number, enabled: boolean): VirtualRows {
  const [container, setContainer] = useState<HTMLElement | null>(null)
  const [geometry, setGeometry] = useState<Geometry | null>(null)
  const [rowHeight, setRowHeight] = useState(DEFAULT_ROW_HEIGHT)
  const frame = useRef(0)

  useLayoutEffect(() => {
    if (tableRef.current) setContainer(scrollParent(tableRef.current))
  }, [tableRef])

  const windowed = enabled && container !== null

  // Track the viewport while windowing; one update per animation frame.
  useLayoutEffect(() => {
    const table = tableRef.current
    if (!windowed || !table || !container) return
    const update = () => {
      frame.current = 0
      setGeometry(measure(table, container))
    }
    const schedule = () => {
      if (!frame.current) frame.current = requestAnimationFrame(update)
    }
    update()
    container.addEventListener('scroll', schedule, { passive: true })
    const resize = new ResizeObserver(schedule)
    resize.observe(container)
    return () => {
      container.removeEventListener('scroll', schedule)
      resize.disconnect()
      cancelAnimationFrame(frame.current)
      frame.current = 0
    }
  }, [windowed, container, tableRef])

  // Measure the real row height once rows are on screen (themes and fonts may differ).
  useLayoutEffect(() => {
    if (!windowed) return
    const row = tableRef.current?.tBodies[0]?.querySelector<HTMLTableRowElement>('tr[data-row-index]')
    const height = row?.getBoundingClientRect().height
    if (height && Math.abs(height - rowHeight) > 0.5) setRowHeight(height)
  }, [windowed, count, geometry, rowHeight, tableRef])

  const range: RowRange =
    windowed && geometry
      ? visibleRange({ ...geometry, rowHeight, count })
      : // Before the first measurement: the first screenful, enough to measure from.
        windowed
        ? { start: 0, end: Math.min(count, 60) }
        : { start: 0, end: count }

  const scrollToIndex = useCallback(
    (index: number) => {
      const table = tableRef.current
      const scroller = container ?? (table ? scrollParent(table) : null)
      if (!table || !scroller) return
      const current = measure(table, scroller)
      const row = table.tBodies[0]?.querySelector<HTMLTableRowElement>(`tr[data-row-index="${index}"]`)
      // A rendered row is placed by its real position; a windowed-out one by the fixed height.
      const height = row ? row.getBoundingClientRect().height : rowHeight
      const offset = row
        ? row.getBoundingClientRect().top - scroller.getBoundingClientRect().top + scroller.scrollTop - index * height
        : current.bodyOffset
      const top = scrollTopFor(index, { ...current, bodyOffset: offset, rowHeight: height }, current.headerHeight)
      if (top !== null) scroller.scrollTop = top
    },
    [container, rowHeight, tableRef],
  )

  return { range, windowed, rowHeight, scrollToIndex }
}
