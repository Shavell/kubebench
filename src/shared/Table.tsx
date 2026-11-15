import clsx from 'clsx'
import { useLayoutEffect, useMemo, useRef, useState, type KeyboardEvent, type ReactNode } from 'react'
import { Icon } from './Icon'
import { useVirtualRows } from './useVirtualRows'
import { moveIndex } from './virtualRange'
import './form/form.css'
import './Table.css'

export interface Column<T> {
  key: string
  header: string
  width?: string
  render: (row: T) => ReactNode
  /** Enables click-to-sort on this column. */
  sortValue?: (row: T) => string | number
}

/** Checkbox selection of rows, by their `rowKey`. */
export interface TableSelection<T> {
  selected: ReadonlySet<string>
  onChange: (selected: ReadonlySet<string>) => void
  /** Accessible name of a row's checkbox, e.g. "Select web-1" (translated). */
  rowLabel: (row: T) => string
  /** Accessible name of the header checkbox that (de)selects every row shown. */
  allLabel: string
}

/** Tables with more rows than this render only the rows near the viewport. */
export const VIRTUALIZE_ABOVE = 300

interface TableProps<T> {
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T) => string
  onRowClick?: (row: T) => void
  onRowContextMenu?: (row: T, e: React.MouseEvent) => void
  /** Adds a checkbox column; the header box covers the rows shown. */
  selection?: TableSelection<T>
}

/**
 * Sortable table of the workbench. Rows the user can act on (click, context
 * menu, selection) are reachable from the keyboard: the table takes focus,
 * ↑/↓, Home/End and PageUp/PageDown move the active row, Enter opens it,
 * Space toggles its checkbox and Shift+F10 (or the menu key) opens its
 * context menu. Above VIRTUALIZE_ABOVE rows only the rows near the viewport
 * of the scrolling ancestor are rendered.
 */
export function Table<T>({ columns, rows, rowKey, onRowClick, onRowContextMenu, selection }: TableProps<T>) {
  const [sort, setSort] = useState<{ key: string; dir: 1 | -1 } | null>(null)
  /** The keyboard's active row, by key, so it survives sorting and refreshes. */
  const [activeKey, setActiveKey] = useState<string | null>(null)
  const tableRef = useRef<HTMLTableElement>(null)

  const sorted = useMemo(() => {
    if (!sort) return rows
    const col = columns.find((c) => c.key === sort.key)
    if (!col?.sortValue) return rows
    const value = col.sortValue
    return [...rows].sort((a, b) => {
      const va = value(a)
      const vb = value(b)
      return (va < vb ? -1 : va > vb ? 1 : 0) * sort.dir
    })
  }, [rows, sort, columns])

  const keys = useMemo(() => sorted.map(rowKey), [sorted, rowKey])
  const virtual = useVirtualRows(tableRef, sorted.length, sorted.length > VIRTUALIZE_ABOVE)
  const columnWidths = useFrozenColumnWidths(tableRef, virtual.windowed, columns)
  const interactive = Boolean(onRowClick || onRowContextMenu || selection)
  const activeIndex = activeKey === null ? -1 : keys.indexOf(activeKey)

  const toggleSort = (col: Column<T>) => {
    if (!col.sortValue) return
    setSort((s) =>
      s?.key === col.key ? { key: col.key, dir: s.dir === 1 ? -1 : 1 } : { key: col.key, dir: 1 },
    )
  }

  const selectedShown = selection ? keys.filter((k) => selection.selected.has(k)).length : 0
  const allShown = keys.length > 0 && selectedShown === keys.length

  const toggleAll = () => {
    if (!selection) return
    const next = new Set(selection.selected)
    for (const key of keys) {
      if (allShown) next.delete(key)
      else next.add(key)
    }
    selection.onChange(next)
  }

  const toggleRow = (key: string) => {
    if (!selection) return
    const next = new Set(selection.selected)
    if (!next.delete(key)) next.add(key)
    selection.onChange(next)
  }

  const activate = (index: number) => {
    if (index < 0) return
    setActiveKey(keys[index])
    virtual.scrollToIndex(index)
  }

  const onKeyDown = (event: KeyboardEvent<HTMLTableElement>) => {
    // Keys typed into a row's checkbox or a cell's own controls stay theirs.
    if (event.target !== event.currentTarget || event.altKey || event.ctrlKey || event.metaKey) return
    const page = Math.max(1, Math.floor((tableRef.current?.parentElement?.clientHeight ?? 0) / virtual.rowHeight) - 1)
    const moves: Record<string, number> = {
      ArrowDown: 1,
      ArrowUp: -1,
      PageDown: page,
      PageUp: -page,
      Home: -sorted.length,
      End: sorted.length,
    }
    const delta = moves[event.key]
    if (delta !== undefined && !event.shiftKey) {
      event.preventDefault()
      activate(moveIndex(activeIndex, delta, sorted.length))
      return
    }
    const row = activeIndex >= 0 ? sorted[activeIndex] : undefined
    if (!row) return
    if (event.key === 'Enter' && onRowClick) {
      event.preventDefault()
      onRowClick(row)
    } else if (event.key === ' ' && selection) {
      event.preventDefault()
      toggleRow(keys[activeIndex])
    } else if ((event.key === 'ContextMenu' || (event.key === 'F10' && event.shiftKey)) && onRowContextMenu) {
      event.preventDefault()
      openRowMenu(activeIndex)
    }
  }

  /** Opens the active row's context menu next to its first cell, like a right-click there. */
  const openRowMenu = (index: number) => {
    const tr = tableRef.current?.querySelector<HTMLTableRowElement>(`tr[data-row-index="${index}"]`)
    if (!tr) return
    const rect = tr.getBoundingClientRect()
    tr.dispatchEvent(
      new MouseEvent('contextmenu', { bubbles: true, cancelable: true, clientX: rect.left + 24, clientY: rect.bottom }),
    )
  }

  const { start, end } = virtual.range
  const spacer = (height: number, position: string) =>
    height > 0 && (
      <tr key={position} className="kb-table__spacer" aria-hidden="true" style={{ height }}>
        <td colSpan={columns.length + (selection ? 1 : 0)} />
      </tr>
    )

  return (
    <table
      ref={tableRef}
      className={clsx('kb-table', columnWidths && 'kb-table--fixed', interactive && 'kb-table--interactive')}
      tabIndex={interactive ? 0 : undefined}
      aria-rowcount={virtual.windowed ? sorted.length + 1 : undefined}
      onKeyDown={interactive ? onKeyDown : undefined}
      onFocus={(event) => {
        // Tabbing into the table starts on the first row.
        if (event.target === event.currentTarget && activeIndex < 0 && sorted.length > 0) setActiveKey(keys[0])
      }}
    >
      {columnWidths && (
        <colgroup>
          {columnWidths.map((width, i) => (
            <col key={i} style={{ width: `${width}%` }} />
          ))}
        </colgroup>
      )}
      <thead>
        <tr aria-rowindex={virtual.windowed ? 1 : undefined}>
          {selection && (
            <th className="kb-table__select">
              <input
                type="checkbox"
                className="kb-checkbox__box"
                aria-label={selection.allLabel}
                title={selection.allLabel}
                checked={allShown}
                disabled={keys.length === 0}
                ref={(box) => {
                  if (box) box.indeterminate = selectedShown > 0 && !allShown
                }}
                onChange={toggleAll}
              />
            </th>
          )}
          {columns.map((col) => (
            <th
              key={col.key}
              style={col.width && !columnWidths ? { width: col.width } : undefined}
              className={col.sortValue ? 'kb-table__th--sortable' : undefined}
              onClick={() => toggleSort(col)}
            >
              {col.header}
              {sort?.key === col.key && (
                <Icon name={sort.dir === 1 ? 'chevron-up' : 'chevron-down'} size={12} />
              )}
            </th>
          ))}
        </tr>
      </thead>
      <tbody>
        {spacer(start * virtual.rowHeight, 'before')}
        {sorted.slice(start, end).map((row, offset) => {
          const index = start + offset
          const key = keys[index]
          const selected = selection?.selected.has(key) ?? false
          return (
            <tr
              key={key}
              data-row-index={index}
              aria-rowindex={virtual.windowed ? index + 2 : undefined}
              className={
                clsx(
                  onRowClick && 'kb-table__row--clickable',
                  selected && 'kb-table__row--selected',
                  index === activeIndex && 'kb-table__row--active',
                ) || undefined
              }
              onClick={() => {
                setActiveKey(key)
                onRowClick?.(row)
              }}
              onContextMenu={
                onRowContextMenu
                  ? (e) => {
                      setActiveKey(key)
                      onRowContextMenu(row, e)
                    }
                  : undefined
              }
            >
              {selection && (
                // Ticking a box must not also open the row.
                <td className="kb-table__select" onClick={(e) => e.stopPropagation()}>
                  <input
                    type="checkbox"
                    className="kb-checkbox__box"
                    aria-label={selection.rowLabel(row)}
                    checked={selected}
                    onChange={() => toggleRow(key)}
                  />
                </td>
              )}
              {columns.map((col) => (
                <td key={col.key}>{col.render(row)}</td>
              ))}
            </tr>
          )
        })}
        {spacer((sorted.length - end) * virtual.rowHeight, 'after')}
      </tbody>
    </table>
  )
}

/**
 * While rows are windowed, the browser would size auto-layout columns from
 * the rendered rows only, so widths would jump while scrolling. The widths
 * of the first rendering are kept instead (as percentages, so they follow
 * the table's width), until windowing ends or the columns change.
 */
function useFrozenColumnWidths<T>(
  tableRef: React.RefObject<HTMLTableElement | null>,
  windowed: boolean,
  columns: Column<T>[],
): number[] | null {
  const [widths, setWidths] = useState<{ columns: string; values: number[] } | null>(null)
  const columnsKey = columns.map((c) => c.key).join('\u0000')

  useLayoutEffect(() => {
    if (!windowed) {
      setWidths(null)
      return
    }
    if (widths?.columns === columnsKey) return
    const table = tableRef.current
    const cells = table?.tHead?.rows[0]?.cells
    if (!table || !cells || table.offsetWidth === 0) return
    const total = table.offsetWidth
    setWidths({ columns: columnsKey, values: [...cells].map((cell) => (cell.offsetWidth / total) * 100) })
  }, [windowed, columnsKey, widths, tableRef])

  return windowed && widths?.columns === columnsKey ? widths.values : null
}
