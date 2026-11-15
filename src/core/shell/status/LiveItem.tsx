import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import { useLiveStore, type LiveState } from '@/core/live/liveStore'
import { PLATFORM_NS } from '@/core/locales'
import { useSettingsStore } from '@/core/store/settingsStore'
import { useDataMode } from '@/data/mode'
import { Icon } from '@/shared/Icon'
import { StatusMenu, type StatusMenuEntry } from './StatusMenu'
import { useMenuAnchor } from './useMenuAnchor'

const ICONS: Record<LiveState, string> = {
  connecting: 'loading',
  live: 'pulse',
  polling: 'sync',
  offline: 'debug-disconnect',
  off: 'circle-slash',
}

/**
 * Live updates (design §7.10): Live, Polling, Offline, … Its menu lists
 * the topics refreshed on a timer, and turns live updates on or off.
 */
export function LiveItem() {
  const { t } = useTranslation(PLATFORM_NS)
  const { mode, server } = useDataMode()
  const { state, reason, polled, watched } = useLiveStore()
  const enabled = useSettingsStore((s) => s.liveUpdates)
  const autoRefreshSeconds = useSettingsStore((s) => s.autoRefreshSeconds)
  const setSetting = useSettingsStore((s) => s.set)
  const { anchor, toggle, close } = useMenuAnchor()

  const stateHint = () => {
    switch (state) {
      case 'live':
        return polled.length
          ? t('statusBar.live.hint.livePartly', { count: polled.length })
          : t('statusBar.live.hint.live')
      case 'polling':
        if (!enabled) return t('statusBar.live.hint.disabled', { count: autoRefreshSeconds })
        return reason
          ? t('statusBar.live.hint.refused', { reason })
          : t('statusBar.live.hint.polling', { count: autoRefreshSeconds || 30 })
      case 'offline':
        return t('statusBar.live.hint.offline')
      case 'connecting':
        return t('statusBar.live.hint.connecting')
      case 'off':
        return t('statusBar.live.hint.off')
    }
  }
  const lines = [mode === 'live' ? t('statusBar.source.liveHint', { server: server ?? '' }) : null, stateHint()]
  const title = lines.filter(Boolean).join('\n')

  const entries: StatusMenuEntry[] = polled.map((p) => ({
    id: `polled:${p.topic}`,
    label: p.topic,
    icon: 'sync',
    detail: p.overflow ? t('statusBar.live.menu.overflow') : p.reason,
    disabled: true,
    onSelect: () => {},
  }))
  if (entries.length) entries.push({ type: 'separator', id: 'sep' })
  entries.push({
    id: 'toggle',
    label: enabled ? t('statusBar.live.menu.turnOff') : t('statusBar.live.menu.turnOn'),
    icon: enabled ? 'debug-pause' : 'play',
    onSelect: () => {
      setSetting('liveUpdates', !enabled)
      close()
    },
  })

  return (
    <>
      <button
        type="button"
        className={clsx('status-bar__item status-bar__item--button', state === 'offline' && 'status-bar__item--offline')}
        title={title}
        aria-haspopup="menu"
        aria-expanded={Boolean(anchor)}
        data-live-state={state}
        onClick={toggle}
      >
        <Icon name={ICONS[state]} size={13} className={clsx(state === 'connecting' && 'codicon-modifier-spin')} />
        <span>{t(`statusBar.live.state.${state}`)}</span>
      </button>
      {anchor && (
        <StatusMenu
          anchor={anchor}
          label={t('statusBar.live.menu.label')}
          header={
            <>
              <div>{stateHint()}</div>
              {watched.length > 0 && <div>{t('statusBar.live.menu.watched', { topics: watched.join(', ') })}</div>}
            </>
          }
          entries={entries}
          onClose={close}
        />
      )}
    </>
  )
}
