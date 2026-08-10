import { useMemo, useState, type ReactNode } from 'react'
import { Icon } from './Icon'
import './Table.css'

export interface Column<T> {
  key: string
  header: string
  width?: string
  render: (row: T) => ReactNode
  /** Enables click-to-sort on this column. */
  sortValue?: (row: T) => string | number
}

interface TableProps<T> {
  columns: Column<T>[]
  rows: T[]
  rowKey: (row: T) => string
  onRowClick?: (row: T) => void
}

export function Table<T>({ columns, rows, rowKey, onRowClick }: TableProps<T>) {
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

  return (
    <table className="kb-table">
      <thead>
        <tr>
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
        {sorted.map((row) => (
          <tr
            key={rowKey(row)}
            className={onRowClick ? 'kb-table__row--clickable' : undefined}
            onClick={() => onRowClick?.(row)}
          >
            {columns.map((col) => (
              <td key={col.key}>{col.render(row)}</td>
            ))}
          </tr>
        ))}
      </tbody>
    </table>
  )
}
