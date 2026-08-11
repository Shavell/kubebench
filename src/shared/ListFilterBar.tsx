import { useState, type ReactNode } from 'react'
import clsx from 'clsx'
import { Icon } from './Icon'
import './ListFilterBar.css'

interface ListFilterBarProps {
  search: string
  onSearch: (value: string) => void
  /** Whether any non-search filter is active (keeps the funnel highlighted). */
  filtersActive?: boolean
  /** "X of Y" summary, always visible. */
  summary?: string
  /** Extra filter controls (selects etc.), shown when expanded. */
  children?: ReactNode
}

/**
 * Compact filter controls that live inside a Toolbar's action area.
 * Collapsed to a single funnel button until needed; column headers
 * handle sorting.
 */
export function ListFilterBar({
  search,
  onSearch,
  filtersActive,
  summary,
  children,
}: ListFilterBarProps) {
  const [open, setOpen] = useState(false)
  const active = Boolean(search) || filtersActive
  const expanded = open || active

  return (
    <div className="inline-filters">
      {summary && <span className="inline-filters__summary">{summary}</span>}
      {expanded && (
        <>
          <div className="inline-filters__search">
            <Icon name="search" size={12} />
            <input
              value={search}
              onChange={(e) => onSearch(e.target.value)}
              placeholder="Filter…"
              spellCheck={false}
            />
            {search && (
              <button className="inline-filters__clear" title="Clear" onClick={() => onSearch('')}>
                <Icon name="close" size={11} />
              </button>
            )}
          </div>
          {children}
        </>
      )}
      <button
        className={clsx('inline-filters__toggle', active && 'inline-filters__toggle--active')}
        title={expanded ? 'Hide filters' : 'Show filters'}
        onClick={() => setOpen((v) => !v)}
      >
        <Icon name={active ? 'filter-filled' : 'filter'} size={15} />
      </button>
    </div>
  )
}
