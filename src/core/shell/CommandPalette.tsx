import { useEffect, useRef, useState } from 'react'
import clsx from 'clsx'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { registry } from '@/core/plugin/registry'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { CommandContribution } from '@/core/plugin/types'
import { useManifestText } from '@/i18n'
import { Icon } from '@/shared/Icon'
import './CommandPalette.css'

type ManifestText = ReturnType<typeof useManifestText>

/** Core commands that belong to the shell rather than any plugin (titles translated). */
function coreCommands(t: TFunction, manifestText: ManifestText): CommandContribution[] {
  const layout = () => useLayoutStore.getState()
  return [
    {
      id: 'core.togglePanel',
      title: t('shell.commands.togglePanel'),
      icon: 'layout-panel',
      run: () => layout().togglePanel(),
    },
    {
      id: 'core.toggleSidebar',
      title: t('shell.commands.toggleSidebar'),
      icon: 'layout-sidebar-left',
      run: () => layout().setActivePlugin(layout().activePluginId),
    },
    {
      id: 'core.closeTab',
      title: t('shell.commands.closeTab'),
      icon: 'close',
      run: () => {
        const s = layout()
        if (s.activeTabId) s.closeTab(s.activeTabId)
      },
    },
    {
      id: 'core.closeAllTabs',
      title: t('shell.commands.closeAllTabs'),
      icon: 'close-all',
      run: () => layout().closeAllTabs(),
    },
    ...registry.activityBarEntries().map((p) => ({
      id: `core.show.${p.id}`,
      title: t('shell.commands.showPlugin', { name: manifestText(p.id, p.name) }),
      icon: p.activityBar!.icon,
      run: () => {
        const s = layout()
        if (s.activePluginId !== p.id) s.setActivePlugin(p.id)
        else if (s.sidebarCollapsed) s.setActivePlugin(p.id)
      },
    })),
  ]
}

/**
 * Case- and accent-insensitive form for matching translated titles:
 * "is yukleri" finds "İş Yükleri", and dotted/dotless i are interchangeable.
 */
function fold(text: string): string {
  return text.normalize('NFD').replace(/\p{M}/gu, '').toLowerCase().replace(/ı/g, 'i')
}

/** Subsequence match on folded text, like VSCode's fuzzy filter. */
function fuzzyMatch(query: string, target: string): boolean {
  const haystack = fold(target)
  let ti = 0
  for (const ch of fold(query)) {
    if (ch === ' ') continue
    ti = haystack.indexOf(ch, ti)
    if (ti === -1) return false
    ti++
  }
  return true
}

export function CommandPalette() {
  const { t } = useTranslation()
  const manifestText = useManifestText()
  const paletteOpen = useLayoutStore((s) => s.paletteOpen)
  const setPaletteOpen = useLayoutStore((s) => s.setPaletteOpen)
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  // Built per render while open so titles follow the current language;
  // plugin command titles are i18n keys in the contributing plugin's namespace.
  const commands: CommandContribution[] = paletteOpen
    ? [
        ...coreCommands(t, manifestText),
        ...registry.commands().map((c) => ({ ...c, title: manifestText(c.pluginId, c.title) })),
      ]
    : []
  const filtered = commands.filter((c) => fuzzyMatch(query, c.title))

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
      // Otherwise the browser's default action for this Enter would also
      // activate whatever the command just focused, e.g. a dialog's button.
      e.preventDefault()
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
          placeholder={t('shell.palette.placeholder')}
          aria-label={t('shell.palette.label')}
          value={query}
          onChange={(e) => setQuery(e.target.value)}
          onKeyDown={onKeyDown}
          spellCheck={false}
        />
        <div className="palette__list" ref={listRef}>
          {filtered.length === 0 && <div className="palette__empty">{t('shell.palette.empty')}</div>}
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
