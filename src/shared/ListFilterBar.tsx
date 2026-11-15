import { useEffect, useRef, useState, type ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import clsx from 'clsx'
import { Icon } from './Icon'
import { useListFilterFocus } from './listFilterFocus'
import './ListFilterBar.css'

interface ListFilterBarProps {
  search: string
  onSearch: (value: string) => void
  /** Whether any non-search filter is active (keeps the funnel highlighted). */
  filtersActive?: boolean
  /** Already translated "X of Y" summary, always visible. */
  summary?: string
  /** Extra filter controls (selects etc.), shown when expanded. */
  children?: ReactNode
}

/**
 * Compact filter controls that live inside a Toolbar's action area.
 * Collapsed to a single funnel button until needed (or `/` is pressed,
 * see listFilterFocus); column headers handle sorting.
 */
export function ListFilterBar({
  search,
  onSearch,
  filtersActive,
  summary,
  children,
}: ListFilterBarProps) {
  const { t } = useTranslation()
  const [open, setOpen] = useState(false)
  const active = Boolean(search) || filtersActive
  const expanded = open || active
  const inputRef = useRef<HTMLInputElement>(null)
  const focusRequest = useListFilterFocus((s) => s.seq)
  const handledRequest = useRef(focusRequest)

  // The `/` shortcut: expand, then focus the search field once it is rendered.
  useEffect(() => {
    if (focusRequest === handledRequest.current) return
    handledRequest.current = focusRequest
    setOpen(true)
    requestAnimationFrame(() => inputRef.current?.focus())
  }, [focusRequest])

  return (
    <div className="inline-filters">
      {summary && <span className="inline-filters__summary">{summary}</span>}
      {expanded && (
        <>
          <div className="inline-filters__search">
            <Icon name="search" size={12} />
            <input
              ref={inputRef}
              value={search}
              onChange={(e) => onSearch(e.target.value)}
              placeholder={t('common.filter')}
              aria-label={t('filters.search')}
              spellCheck={false}
            />
            {search && (
              <button className="inline-filters__clear" title={t('common.clear')} onClick={() => onSearch('')}>
                <Icon name="close" size={11} />
              </button>
            )}
          </div>
          {children}
        </>
      )}
      <button
        className={clsx('inline-filters__toggle', active && 'inline-filters__toggle--active')}
        title={expanded ? t('filters.hide') : t('filters.show')}
        onClick={() => setOpen((v) => !v)}
      >
        <Icon name={active ? 'filter-filled' : 'filter'} size={15} />
      </button>
    </div>
  )
}
