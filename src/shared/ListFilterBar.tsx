import type { ReactNode } from 'react'
import { Icon } from './Icon'
import './ListFilterBar.css'

interface ListFilterBarProps {
  search: string
  onSearch: (value: string) => void
  /** Right-aligned "X of Y" style summary. */
  summary?: string
  /** Extra filter controls (selects etc.). */
  children?: ReactNode
}

/** Filter strip under a list page's toolbar. Column headers handle sorting. */
export function ListFilterBar({ search, onSearch, summary, children }: ListFilterBarProps) {
  return (
    <div className="list-filters">
      <div className="list-filters__search">
        <Icon name="search" size={13} />
        <input
          value={search}
          onChange={(e) => onSearch(e.target.value)}
          placeholder="Filter by name…"
          spellCheck={false}
        />
        {search && (
          <button className="list-filters__clear" title="Clear" onClick={() => onSearch('')}>
            <Icon name="close" size={12} />
          </button>
        )}
      </div>
      {children}
      <span className="list-filters__summary">{summary}</span>
    </div>
  )
}
