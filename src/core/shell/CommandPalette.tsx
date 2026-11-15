import { useEffect, useRef, useState } from 'react'
import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import { allCommands } from '@/core/commands/coreCommands'
import { PLATFORM_NS } from '@/core/locales'
import { fuzzyMatch } from '@/core/palette/match'
import { expandAlias, parseKubectl } from '@/core/palette/kubectl'
import { kubectlNeeds, kubectlRows, verbRows, type KubectlEnv, type PaletteRow } from '@/core/palette/kubectlRows'
import { useKubectlData } from '@/core/palette/useKubectlData'
import { deniedReason } from '@/core/permissions'
import { showInPanel } from '@/core/plugin/panelRequests'
import { registry } from '@/core/plugin/registry'
import { KeyCaps } from '@/core/shortcuts/KeyCaps'
import { useAliases } from '@/core/store/aliasStore'
import { useContextStore } from '@/core/store/contextStore'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviders } from '@/data/DataContext'
import { useManifestText } from '@/i18n'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import './CommandPalette.css'

/**
 * The command palette (Ctrl+Shift+P / F1): fuzzy search over every command,
 * and kubectl-style commands (`get pods -n shop`, `logs web-1`, `ns shop`,
 * see core/palette/kubectl.ts) with Tab completion and the user's aliases.
 */
export function CommandPalette() {
  const paletteOpen = useLayoutStore((s) => s.paletteOpen)
  return paletteOpen ? <OpenPalette /> : null
}

function OpenPalette() {
  const { t } = useTranslation()
  const { t: tp } = useTranslation(PLATFORM_NS)
  const manifestText = useManifestText()
  const providers = useProviders()
  const setPaletteOpen = useLayoutStore((s) => s.setPaletteOpen)
  const currentNamespace = useContextStore((s) => s.namespace)
  const { aliases } = useAliases()
  const [query, setQuery] = useState('')
  const [selected, setSelected] = useState(0)
  const inputRef = useRef<HTMLInputElement>(null)
  const listRef = useRef<HTMLDivElement>(null)

  const expanded = expandAlias(query, aliases)
  const command = parseKubectl(expanded)
  const kinds = registry.resourceKinds()
  const needs = command ? kubectlNeeds(command, kinds, currentNamespace) : null
  const data = useKubectlData(needs, { bookmark: tp('statusBar.cluster.bookmark'), broken: tp('statusBar.cluster.broken') })

  const close = () => setPaletteOpen(false)

  const env: KubectlEnv = {
    kinds,
    ...data,
    currentNamespace,
    denied: (permission) => deniedReason(permission),
    t: (key, values) => tp(key, values ?? {}),
    actions: {
      setNamespace: (namespace) => useContextStore.getState().setNamespace(namespace),
      showLogs: (target) => {
        if (!showInPanel('pod-logs', { ...target })) notify.info(tp('palette.kubectl.noPanel'))
      },
      openShell: (target) => {
        if (!showInPanel('pod-exec', { ...target })) notify.info(tp('palette.kubectl.noPanel'))
      },
      switchCluster: (name) => {
        notify.info(tp('statusBar.cluster.switching', { name }))
        providers.inventory.activateCluster(name).then(
          () => window.location.reload(),
          (err: unknown) => notify.error(err),
        )
      },
    },
  }

  // Titles are built per render so they follow the current language.
  const rows: PaletteRow[] = command
    ? kubectlRows(command, expanded, env)
    : [
        ...allCommands(t, manifestText)
          .filter((c) => fuzzyMatch(query, c.title))
          .map((c) => ({ id: c.id, label: c.title, icon: c.icon ?? 'circle-small', run: c.run, keys: c.keybinding ? [c.keybinding] : undefined })),
        ...verbRows(query, aliases, env.t),
      ]
  const selectedIndex = Math.min(selected, rows.length - 1)

  useEffect(() => {
    // Focus after the overlay mounts.
    requestAnimationFrame(() => inputRef.current?.focus())
  }, [])

  useEffect(() => {
    listRef.current?.querySelector('.palette__row--selected')?.scrollIntoView({ block: 'nearest' })
  }, [selectedIndex])

  const updateQuery = (next: string) => {
    setQuery(next)
    setSelected(0)
  }

  const activate = (row: PaletteRow | undefined) => {
    if (!row || row.disabledReason) return
    if (row.run) {
      close()
      row.run()
    } else if (row.complete !== undefined) {
      updateQuery(row.complete)
      inputRef.current?.focus()
    }
  }

  const onKeyDown = (e: React.KeyboardEvent) => {
    if (e.key === 'Escape') {
      close()
    } else if (e.key === 'ArrowDown') {
      e.preventDefault()
      setSelected(Math.min(rows.length - 1, selectedIndex + 1))
    } else if (e.key === 'ArrowUp') {
      e.preventDefault()
      setSelected(Math.max(0, selectedIndex - 1))
    } else if (e.key === 'Tab') {
      // Tab completes; it never moves the focus out of the palette.
      e.preventDefault()
      const row = rows[selectedIndex]
      if (row?.complete !== undefined) updateQuery(row.complete)
    } else if (e.key === 'Enter') {
      // Otherwise the browser's default action for this Enter would also
      // activate whatever the command just focused, e.g. a dialog's button.
      e.preventDefault()
      activate(rows[selectedIndex])
    }
  }

  return (
    <div className="palette-overlay" onMouseDown={close}>
      <div className="palette" onMouseDown={(e) => e.stopPropagation()}>
        <input
          ref={inputRef}
          className="palette__input"
          placeholder={t('shell.palette.placeholder')}
          aria-label={t('shell.palette.label')}
          value={query}
          onChange={(e) => updateQuery(e.target.value)}
          onKeyDown={onKeyDown}
          spellCheck={false}
          autoComplete="off"
        />
        {command && expanded !== query && <div className="palette__expansion">{expanded}</div>}
        <div className="palette__list" ref={listRef} role="listbox" aria-label={t('shell.palette.label')}>
          {rows.length === 0 && <div className="palette__empty">{t('shell.palette.empty')}</div>}
          {rows.map((row, i) => (
            <div
              key={row.id}
              role="option"
              aria-selected={i === selectedIndex}
              aria-disabled={row.disabledReason ? true : undefined}
              title={row.disabledReason}
              className={clsx(
                'palette__row',
                i === selectedIndex && 'palette__row--selected',
                row.disabledReason && 'palette__row--disabled',
                !row.run && !row.complete && 'palette__row--info',
              )}
              onMouseEnter={() => setSelected(i)}
              onClick={() => activate(row)}
            >
              <Icon
                name={row.icon}
                size={14}
                className={clsx('palette__icon', row.icon === 'loading' && 'codicon-modifier-spin')}
              />
              <span className={clsx('palette__label', command && 'palette__label--command')}>{row.label}</span>
              {(row.disabledReason ?? row.detail) && (
                <span className="palette__detail">{row.disabledReason ?? row.detail}</span>
              )}
              {row.keys?.map((binding) => <KeyCaps key={binding} binding={binding} />)}
            </div>
          ))}
        </div>
        {!query && <div className="palette__hint">{tp('palette.kubectl.hint')}</div>}
      </div>
    </div>
  )
}
