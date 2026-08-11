import { useEffect, useMemo, useRef, useState } from 'react'
import clsx from 'clsx'
import { registry } from '@/core/plugin/registry'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { CommandContribution } from '@/core/plugin/types'
import { Icon } from '@/shared/Icon'
import './CommandPalette.css'

/** Core commands that belong to the shell rather than any plugin. */
function coreCommands(): CommandContribution[] {
  const layout = () => useLayoutStore.getState()
  return [
    {
      id: 'core.togglePanel',
      title: 'View: Toggle Panel',
      icon: 'layout-panel',
      run: () => layout().togglePanel(),
    },
    {
      id: 'core.toggleSidebar',
      title: 'View: Toggle Sidebar',
      icon: 'layout-sidebar-left',
      run: () => layout().setActivePlugin(layout().activePluginId),
    },
    {
      id: 'core.closeTab',
      title: 'View: Close Active Tab',
      icon: 'close',
      run: () => {
        const s = layout()
        if (s.activeTabId) s.closeTab(s.activeTabId)
      },
    },
    {
      id: 'core.closeAllTabs',
      title: 'View: Close All Tabs',
      icon: 'close-all',
      run: () => layout().closeAllTabs(),
    },
    ...registry.activityBarEntries().map((p) => ({
      id: `core.show.${p.id}`,
      title: `View: Show ${p.name}`,
      icon: p.activityBar!.icon,
      run: () => {
        const s = layout()
        if (s.activePluginId !== p.id) s.setActivePlugin(p.id)
        else if (s.sidebarCollapsed) s.setActivePlugin(p.id)
      },
    })),
  ]
}

/** Case-insensitive subsequence match, like VSCode's fuzzy filter. */
function fuzzyMatch(query: string, target: string): boolean {
  const q = query.toLowerCase()
  const t = target.toLowerCase()
  let ti = 0
  for (const ch of q) {
    if (ch === ' ') continue
    ti = t.indexOf(ch, ti)
    if (ti === -1) return false
    ti++
  }
  return true
}

export function CommandPalette() {
  const paletteOpen = useLayoutStore((s) => s.paletteOpen)
  const setPaletteOpen = useLayoutStore((s) => s.setPaletteOpen)
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  const commands = useMemo(
    () => (paletteOpen ? [...coreCommands(), ...registry.commands()] : []),
    [paletteOpen],
  )

  const filtered = useMemo(
    () => commands.filter((c) => fuzzyMatch(query, c.title)),
    [commands, query],
  )

  useEffect(() => {
    if (paletteOpen) {
      setQuery('')
      setSelected(0)
      // Focus after the overlay mounts.
      requestAnimationFrame(() => inputRef.current?.focus())
    }
  }, [paletteOpen])

  useEffect(() => setSelected(0), [query])

  useEffect(() => {
    listRef.current
      ?.querySelector('.palette__row--selected')
      ?.scrollIntoView({ block: 'nearest' })
  }, [selected])

  if (!paletteOpen) return null

  const runCommand = (command: CommandContribution) => {
    setPaletteOpen(false)
    command.run()
  }

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      setPaletteOpen(false)
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelected((i) => Math.min(filtered.length - 1, i + 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelected((i) => Math.max(0, i - 1))
    } else if (e.key === 'Enter') {
      const command = filtered[selected]
      if (command) runCommand(command)
    }
  }

  return (
    <div className="palette-overlay" onMouseDown={() => setPaletteOpen(false)}>
      <div className="palette" onMouseDown={(e) => e.stopPropagation()}>
        <input
          ref={inputRef}
          className="palette__input"
          placeholder="Type a command…"
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={onKeyDown}
          spellCheck={false}
        />
        <div className="palette__list" ref={listRef}>
          {filtered.length === 0 && <div className="palette__empty">No matching commands</div>}
          {filtered.map((command, i) => (
            <div
              key={command.id}
              className={clsx('palette__row', i === selected && 'palette__row--selected')}
              onMouseEnter={() => setSelected(i)}
              onClick={() => runCommand(command)}
            >
              <Icon name={command.icon ?? 'circle-small'} size={14} className="palette__icon" />
              <span>{command.title}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  )
}
