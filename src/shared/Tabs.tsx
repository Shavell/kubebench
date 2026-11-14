import clsx from 'clsx'
import { useId, useRef, type KeyboardEvent, type ReactNode } from 'react'
import { Icon } from './Icon'
import './Tabs.css'

export interface TabItem<K extends string = string> {
  id: K
  /** Already translated. */
  label: string
  icon?: string
  /** Count or short text after the label, e.g. the number of events. */
  badge?: string | number
  disabled?: boolean
}

export interface TabsProps<K extends string = string> {
  tabs: readonly TabItem<K>[]
  active: K
  onChange: (id: K) => void
  /** Accessible name of the tab list, e.g. the page title. */
  ariaLabel?: string
  /** Content of the active tab, rendered in a tab panel labelled by its tab. */
  children?: ReactNode
  className?: string
}

/**
 * In-page tab strip (Overview / Events / YAML …), not the editor tabs.
 * Arrow keys, Home and End move between tabs (WAI-ARIA tabs pattern).
 *
 *   <Tabs tabs={tabs} active={tab} onChange={setTab}>
 *     {tab === 'events' ? <EventsTable … /> : <Overview … />}
 *   </Tabs>
 */
export function Tabs<K extends string>({
  tabs,
  active,
  onChange,
  ariaLabel,
  children,
  className,
}: TabsProps<K>) {
  const baseId = useId()
  const listRef = useRef<HTMLDivElement>(null)
  const tabId = (id: K) => `${baseId}tab-${id}`
  const panelId = `${baseId}panel`

  const onKeyDown = (event: KeyboardEvent<HTMLDivElement>) => {
    const enabled = tabs.filter((tab) => !tab.disabled)
    const index = enabled.findIndex((tab) => tab.id === active)
    let next: TabItem<K> | undefined
    if (event.key === 'ArrowRight') next = enabled[(index + 1) % enabled.length]
    else if (event.key === 'ArrowLeft') next = enabled[(index - 1 + enabled.length) % enabled.length]
    else if (event.key === 'Home') next = enabled[0]
    else if (event.key === 'End') next = enabled[enabled.length - 1]
    if (!next) return
    event.preventDefault()
    onChange(next.id)
    listRef.current?.querySelector<HTMLElement>(`[data-tab="${CSS.escape(next.id)}"]`)?.focus()
  }

  return (
    <div className={clsx('kb-tabs', className)}>
      <div ref={listRef} role="tablist" aria-label={ariaLabel} className="kb-tabs__list" onKeyDown={onKeyDown}>
        {tabs.map((tab) => {
          const selected = tab.id === active
          return (
            <button
              key={tab.id}
              type="button"
              role="tab"
              id={tabId(tab.id)}
              data-tab={tab.id}
              aria-selected={selected}
              aria-controls={children === undefined ? undefined : panelId}
              tabIndex={selected ? 0 : -1}
              disabled={tab.disabled}
              className={clsx('kb-tabs__tab', selected && 'kb-tabs__tab--active')}
              onClick={() => onChange(tab.id)}
            >
              {tab.icon && <Icon name={tab.icon} size={14} />}
              <span>{tab.label}</span>
              {tab.badge !== undefined && <span className="kb-tabs__badge">{tab.badge}</span>}
            </button>
          )
        })}
      </div>
      {children !== undefined && (
        <div role="tabpanel" id={panelId} aria-labelledby={tabId(active)} className="kb-tabs__panel">
          {children}
        </div>
      )}
    </div>
  )
}
