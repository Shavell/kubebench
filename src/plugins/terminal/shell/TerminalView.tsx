import clsx from 'clsx'
import { useEffect, useLayoutEffect, useRef, useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'
import { useDisplaySettings } from '@/core/store/displayStore'
import { useResolvedTheme } from '@/core/theme/themeController'
import { useDataMode } from '@/data/mode'
import { Badge } from '@/shared/Badge'
import { fontStack } from '@/shared/fontFamily'
import { showContextMenu } from '@/shared/contextMenu'
import { Select } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Unavailable } from '@/shared/Unavailable'
import { NS } from '../i18n'
import { ToolButton } from '../ToolButton'
import { usePodContainers } from '../usePodContainers'
import type { TerminalSession, TerminalStatus } from './TerminalSession'
import { monospaceFont, xtermTheme } from './xterm'
import './terminal.css'

/**
 * An interactive terminal (xterm.js) for one session, with its toolbar:
 * the connection status and its reason, a container picker, reconnect and
 * the clipboard. The session outlives the view; mounting it moves the
 * session's screen in, unmounting moves it out.
 */
export function TerminalView({ session }: { session: TerminalSession }) {
  const { t } = useTranslation(NS)
  const status = useSyncExternalStore(session.subscribe, () => session.status)
  const target = session.target
  const { terminalFontSize: fontSize, terminalFontFamily } = useDisplaySettings()
  const theme = useResolvedTheme()
  const mock = useDataMode().mode === 'mock'
  const containers = usePodContainers(target.namespace, target.pod, target.container)
  const bodyRef = useRef<HTMLDivElement>(null)

  useLayoutEffect(() => {
    const body = bodyRef.current
    if (!body) return
    session.attach(body)
    const observer = new ResizeObserver(() => session.fitToHost())
    observer.observe(body)
    return () => {
      observer.disconnect()
      session.detach()
    }
  }, [session])

  useEffect(() => {
    session.setAppearance({ fontSize, fontFamily: fontStack(terminalFontFamily, monospaceFont()), theme: xtermTheme() })
  }, [session, fontSize, terminalFontFamily, theme])

  const copy = () =>
    session.copySelection().then(
      (copied) => copied || notify.info(t('shell.nothingSelected')),
      (err: unknown) => notify.error(t('shell.copyFailed'), { detail: String(err) }),
    )
  const paste = () =>
    navigator.clipboard.readText().then(
      (text) => session.paste(text),
      () => notify.error(t('shell.pasteFailed'), { detail: t('shell.pasteHint') }),
    )

  const menu = (event: React.MouseEvent) =>
    showContextMenu(event, [
      { label: t('shell.copy'), icon: 'copy', disabled: !session.hasSelection(), onClick: () => void copy() },
      { label: t('shell.paste'), icon: 'clippy', onClick: () => void paste() },
      { type: 'separator' },
      { label: t('shell.selectAll'), onClick: () => session.selectAll() },
      { label: t('shell.clear'), icon: 'clear-all', onClick: () => session.clear() },
    ])

  const starting = status.state === 'loading' || status.state === 'connecting'
  return (
    <div className="kb-terminal">
      <div className="kb-terminal__toolbar">
        <StatusText status={status} />
        {mock && (
          <span className="kb-terminal__mock" title={t('shell.mockHint')}>
            <Badge tone="info">{t('shell.mock')}</Badge>
          </span>
        )}
        <div className="kb-terminal__actions">
          <Select
            className="kb-terminal__container"
            aria-label={t('shell.container')}
            title={t('shell.container')}
            value={target.container}
            options={containers.map((name) => ({ value: name, label: name }))}
            disabled={status.state === 'loading' || status.state === 'loadFailed'}
            onChange={(container) => session.reconnect({ ...target, container })}
          />
          <ToolButton
            icon="debug-restart"
            title={t('shell.reconnect')}
            disabled={starting || status.state === 'loadFailed'}
            onClick={() => session.reconnect()}
          />
          <ToolButton icon="copy" title={t('shell.copy')} onClick={() => void copy()} />
          <ToolButton icon="clippy" title={t('shell.paste')} onClick={() => void paste()} />
          <ToolButton icon="clear-all" title={t('shell.clear')} onClick={() => session.clear()} />
        </div>
      </div>
      <div
        className={clsx('kb-terminal__body', status.state === 'loadFailed' && 'kb-terminal__body--hidden')}
        ref={bodyRef}
        onContextMenu={menu}
      />
      {status.state === 'loadFailed' && (
        <Unavailable
          compact
          icon="warning"
          title={t('shell.loadFailed')}
          error={status.error}
          onRetry={() => session.retryLoad()}
        />
      )}
    </div>
  )
}

function StatusText({ status }: { status: TerminalStatus }) {
  const { t } = useTranslation(NS)
  const view = statusView(status)
  return (
    <span className={clsx('kb-terminal__status', `kb-terminal__status--${view.tone}`)} role="status">
      <Icon name={view.icon} size={14} className={view.spin ? 'codicon-modifier-spin' : undefined} />
      {t(`shell.status.${status.state}`, view.values ?? {})}
    </span>
  )
}

function statusView(status: TerminalStatus): {
  icon: string
  tone: 'ok' | 'busy' | 'ended' | 'bad'
  spin?: boolean
  values?: Record<string, string | number>
} {
  switch (status.state) {
    case 'loading':
    case 'connecting':
      return { icon: 'loading', tone: 'busy', spin: true }
    case 'open':
      return { icon: 'circle-filled', tone: 'ok' }
    case 'exited':
      return { icon: status.code === 0 ? 'pass' : 'warning', tone: status.code === 0 ? 'ended' : 'bad', values: { code: status.code } }
    case 'failed':
      return { icon: 'error', tone: 'bad', values: { message: status.message } }
    case 'refused':
    case 'loadFailed':
      return { icon: 'error', tone: 'bad' }
    case 'lost':
      return { icon: 'debug-disconnect', tone: 'bad' }
  }
}
