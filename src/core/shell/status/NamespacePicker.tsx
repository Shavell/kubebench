import clsx from 'clsx'
import { useEffect, useId, useLayoutEffect, useRef, useState, type KeyboardEvent } from 'react'
import { createPortal } from 'react-dom'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { fold } from '@/core/palette/match'
import { ALL_NAMESPACES, useContextStore } from '@/core/store/contextStore'
import { orderNamespaces } from '@/core/store/namespaceOrder'
import { useNamespacePrefs } from '@/core/store/namespacePrefsStore'
import { useProviderQuery } from '@/data/DataContext'
import { Checkbox } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import './NamespacePicker.css'

interface Entry {
  value: string
  label: string
  pinned: boolean
  hidden: boolean
  /** First entry of the pinned or of the other namespaces: gets a group heading. */
  heading?: string
}

interface NamespacePickerProps {
  /** The status bar button that opened the picker; it sits above it. */
  anchor: HTMLElement
  onClose: () => void
}

/**
 * The namespace filter's picker: type to filter, ↑/↓ and Enter to choose,
 * favourites (pinned, Alt+P) first, hidden namespaces (Alt+H) left out
 * until "Show hidden" is ticked. Preferences are kept per cluster.
 */
export function NamespacePicker({ anchor, onClose }: NamespacePickerProps) {
  const { t } = useTranslation(PLATFORM_NS)
  const namespace = useContextStore((s) => s.namespace)
  const setNamespace = useContextStore((s) => s.setNamespace)
  const prefs = useNamespacePrefs()
  const { data, loading, error } = useProviderQuery((p) => p.cluster.listNamespaces(), [])
  const [query, setQuery] = useState('')
  const [showHidden, setShowHidden] = useState(false)
  const [active, setActive] = useState(0)
  const panelRef = useRef<HTMLDivElement>(null)
  const inputRef = useRef<HTMLInputElement>(null)
  const listId = useId()

  const names = (data ?? []).map((ns) => ns.name)
  const ordered = orderNamespaces(names, prefs, { includeHidden: showHidden, keep: namespace })
  const hiddenSet = new Set(prefs.hidden)
  const hiddenCount = names.filter((name) => hiddenSet.has(name)).length
  const needle = fold(query.trim())
  const matches = (name: string) => !needle || fold(name).includes(needle)
  const pinned = ordered.pinned.filter(matches)
  const others = ordered.others.filter(matches)
  const entries: Entry[] = [
    ...(needle ? [] : [{ value: ALL_NAMESPACES, label: t('statusBar.namespace.all'), pinned: false, hidden: false }]),
    ...pinned.map((name, i) => ({
      value: name,
      label: name,
      pinned: true,
      hidden: false,
      heading: i === 0 ? t('namespaces.pinnedGroup') : undefined,
    })),
    ...others.map((name, i) => ({
      value: name,
      label: name,
      pinned: false,
      hidden: hiddenSet.has(name),
      heading: i === 0 && pinned.length > 0 ? t('namespaces.othersGroup') : undefined,
    })),
  ]
  const activeIndex = Math.min(active, entries.length - 1)

  // Above the anchor, kept inside the window.
  useLayoutEffect(() => {
    const panel = panelRef.current
    if (!panel) return
    const rect = anchor.getBoundingClientRect()
    panel.style.left = `${Math.max(4, Math.min(rect.left, window.innerWidth - panel.offsetWidth - 4))}px`
    panel.style.bottom = `${window.innerHeight - rect.top + 2}px`
  }, [anchor])

  const onCloseRef = useRef(onClose)
  useLayoutEffect(() => {
    onCloseRef.current = onClose
  })

  useEffect(() => {
    inputRef.current?.focus()
    const close = () => onCloseRef.current()
    const onPointerDown = (event: MouseEvent) => {
      const target = event.target as Node
      if (!panelRef.current?.contains(target) && !anchor.contains(target)) close()
    }
    window.addEventListener('mousedown', onPointerDown)
    window.addEventListener('resize', close)
    return () => {
      window.removeEventListener('mousedown', onPointerDown)
      window.removeEventListener('resize', close)
    }
  }, [anchor])

  useEffect(() => {
    panelRef.current?.querySelector('.ns-picker__option--active')?.scrollIntoView({ block: 'nearest' })
  }, [activeIndex])

  const closeAndReturnFocus = () => {
    onClose()
    anchor.focus()
  }

  const choose = (value: string) => {
    setNamespace(value)
    closeAndReturnFocus()
  }

  const onKeyDown = (event: KeyboardEvent<HTMLInputElement>) => {
    const entry = entries[activeIndex]
    if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
      event.preventDefault()
      const delta = event.key === 'ArrowDown' ? 1 : -1
      setActive((activeIndex + delta + entries.length) % Math.max(1, entries.length))
    } else if (event.key === 'Enter') {
      event.preventDefault()
      if (entry) choose(entry.value)
    } else if (event.key === 'Escape') {
      event.preventDefault()
      closeAndReturnFocus()
    } else if (event.key === 'Tab') {
      onClose()
    } else if (event.altKey && entry && entry.value !== ALL_NAMESPACES && (event.code === 'KeyP' || event.code === 'KeyH')) {
      event.preventDefault()
      if (event.code === 'KeyP') prefs.togglePinned(entry.value)
      else prefs.toggleHidden(entry.value)
    }
  }

  const status = loading && !data ? t('namespaces.loading') : error ? error : entries.length === 0 ? t('namespaces.noMatch') : null

  return createPortal(
    <div ref={panelRef} className="ns-picker" role="dialog" aria-label={t('statusBar.namespace.tooltip')}>
      <input
        ref={inputRef}
        className="ns-picker__input"
        role="combobox"
        aria-expanded="true"
        aria-controls={listId}
        aria-activedescendant={entries[activeIndex] ? `${listId}-${activeIndex}` : undefined}
        aria-label={t('namespaces.search')}
        placeholder={t('namespaces.search')}
        value={query}
        spellCheck={false}
        onChange={(e) => {
          setQuery(e.target.value)
          setActive(0)
        }}
        onKeyDown={onKeyDown}
      />
      <div id={listId} role="listbox" className="ns-picker__list" aria-label={t('statusBar.namespace.tooltip')}>
        {entries.map((entry, i) => (
          <div key={entry.value || '*'} role="presentation">
            {entry.heading && <div className="ns-picker__heading" role="presentation">{entry.heading}</div>}
            <div
              id={`${listId}-${i}`}
              role="option"
              aria-selected={entry.value === namespace}
              className={clsx(
                'ns-picker__option',
                i === activeIndex && 'ns-picker__option--active',
                entry.hidden && 'ns-picker__option--hidden',
              )}
              onMouseEnter={() => setActive(i)}
              onMouseDown={(e) => e.preventDefault()}
              onClick={() => choose(entry.value)}
            >
              <span className="ns-picker__check">{entry.value === namespace && <Icon name="check" size={14} />}</span>
              <span className="ns-picker__label">{entry.label}</span>
              {entry.value !== ALL_NAMESPACES && (
                <span className="ns-picker__actions">
                  <button
                    type="button"
                    tabIndex={-1}
                    className={clsx('ns-picker__action', entry.pinned && 'ns-picker__action--on')}
                    title={entry.pinned ? t('namespaces.unpin') : t('namespaces.pin')}
                    aria-label={`${entry.pinned ? t('namespaces.unpin') : t('namespaces.pin')}: ${entry.label}`}
                    onClick={(e) => {
                      e.stopPropagation()
                      prefs.togglePinned(entry.value)
                    }}
                  >
                    <Icon name={entry.pinned ? 'pinned' : 'pin'} size={14} />
                  </button>
                  <button
                    type="button"
                    tabIndex={-1}
                    className={clsx('ns-picker__action', entry.hidden && 'ns-picker__action--on')}
                    title={entry.hidden ? t('namespaces.unhide') : t('namespaces.hide')}
                    aria-label={`${entry.hidden ? t('namespaces.unhide') : t('namespaces.hide')}: ${entry.label}`}
                    onClick={(e) => {
                      e.stopPropagation()
                      prefs.toggleHidden(entry.value)
                    }}
                  >
                    <Icon name={entry.hidden ? 'eye' : 'eye-closed'} size={14} />
                  </button>
                </span>
              )}
            </div>
          </div>
        ))}
        {status && <div className="ns-picker__status">{status}</div>}
      </div>
      <div className="ns-picker__footer">
        <Checkbox
          checked={showHidden}
          disabled={hiddenCount === 0}
          onChange={setShowHidden}
          label={t('namespaces.showHidden', { count: hiddenCount })}
        />
        <span className="ns-picker__hint">{t('namespaces.keysHint')}</span>
      </div>
    </div>,
    document.body,
  )
}
