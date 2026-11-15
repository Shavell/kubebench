import clsx from 'clsx'
import { useMemo, useState, type ReactNode } from 'react'
import { Icon } from './Icon'
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

interface TableProps<T> {
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T) => string
  onRowClick?: (row: T) => void
  onRowContextMenu?: (row: T, e: React.MouseEvent) => void
  /** Adds a checkbox column; the header box covers the rows shown. */
  selection?: TableSelection<T>
}

export function Table<T>({ columns, rows, rowKey, onRowClick, onRowContextMenu, selection }: TableProps<T>) {
  const [sort, setSort] = useState<{ key: string; dir: 1 | -1 } | null>(null)

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

  const toggleSort = (col: Column<T>) => {
    if (!col.sortValue) return
    setSort((s) =>
      s?.key === col.key ? { key: col.key, dir: s.dir === 1 ? -1 : 1 } : { key: col.key, dir: 1 },
    )
  }

  const shownKeys = selection ? sorted.map(rowKey) : []
  const selectedShown = selection ? shownKeys.filter((k) => selection.selected.has(k)).length : 0
  const allShown = shownKeys.length > 0 && selectedShown === shownKeys.length

  const toggleAll = () => {
    if (!selection) return
    const next = new Set(selection.selected)
    for (const key of shownKeys) {
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

  return (
    <table className="kb-table">
      <thead>
        <tr>
          {selection && (
            <th className="kb-table__select">
              <input
                type="checkbox"
                className="kb-checkbox__box"
                aria-label={selection.allLabel}
                title={selection.allLabel}
                checked={allShown}
                disabled={shownKeys.length === 0}
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
              style={col.width ? { width: col.width } : undefined}
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
        {sorted.map((row) => {
          const key = rowKey(row)
          const selected = selection?.selected.has(key) ?? false
          return (
            <tr
              key={key}
              className={
                clsx(onRowClick && 'kb-table__row--clickable', selected && 'kb-table__row--selected') || undefined
              }
              onClick={() => onRowClick?.(row)}
              onContextMenu={onRowContextMenu ? (e) => onRowContextMenu(row, e) : undefined}
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
      </tbody>
    </table>
  )
}
