import clsx from 'clsx'
import { useEffect, useState, useSyncExternalStore } from 'react'
import { useTranslation } from 'react-i18next'
import { usePanelRequest } from '@/core/plugin/panelRequests'
import { useProviders } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { Icon } from '@/shared/Icon'
import { Unavailable } from '@/shared/Unavailable'
import { NS } from '../i18n'
import { NewSessionDialog } from '../shell/NewSessionDialog'
import {
  activatePanelSession,
  closePanelSession,
  getSession,
  openPanelSession,
  usePanelSessions,
} from '../shell/sessions'
import type { TerminalSession } from '../shell/TerminalSession'
import { TerminalView } from '../shell/TerminalView'
import { useTerminalBlocker } from '../shell/useTerminalBlocker'
import { ToolButton } from '../ToolButton'
import { podTarget } from './podTarget'
import './panels.css'

/**
 * Bottom-panel Terminal: one sub-tab per session, a button for a new one.
 * Every `pod-exec` request (e.g. a pod row's "Shell" action) opens a
 * session in a new sub-tab. Sessions keep running while the panel shows
 * another tab or is hidden.
 */
export function TerminalPanel() {
  const { t } = useTranslation(NS)
  const providers = useProviders()
  const blocker = useTerminalBlocker()
  const blocked = blocker !== null
  const request = usePanelRequest('pod-exec')
  const ids = usePanelSessions((s) => s.ids)
  const activeId = usePanelSessions((s) => s.activeId)
  const [picking, setPicking] = useState(false)
  const active = activeId ? getSession(activeId) : undefined

  useEffect(() => {
    // The panel remounts whenever it is shown again: every request is
    // handled once, and without exec access it opens nothing.
    if (!request || request.seq <= usePanelSessions.getState().handledRequest) return
    usePanelSessions.setState({ handledRequest: request.seq })
    const target = podTarget(request.props)
    if (target && !blocked) openPanelSession(target, providers.cluster)
  }, [request, blocked, providers])

  if (blocker) {
    return <Unavailable compact code={blocker.code} reason={blocker.reason} />
  }

  return (
    <div className="term-panel">
      <div className="term-panel__tabs" role="tablist" aria-label={t('shell.sessions')}>
        {ids.map((id) => {
          const session = getSession(id)
          return session ? <SessionTab key={id} session={session} active={id === activeId} /> : null
        })}
        <ToolButton icon="add" title={t('shell.newSession')} onClick={() => setPicking(true)} />
      </div>
      {active ? (
        <TerminalView key={active.id} session={active} />
      ) : (
        <div className="term-panel__empty">
          <EmptyState icon="terminal" title={t('shell.noSessions')} hint={t('shell.hint')} />
          <Button variant="secondary" icon="add" onClick={() => setPicking(true)}>
            {t('shell.newSession')}
          </Button>
        </div>
      )}
      {picking && (
        <NewSessionDialog
          initial={active?.target}
          onClose={() => setPicking(false)}
          onOpen={(target) => openPanelSession(target, providers.cluster)}
        />
      )}
    </div>
  )
}

function SessionTab({ session, active }: { session: TerminalSession; active: boolean }) {
  const { t } = useTranslation(NS)
  const status = useSyncExternalStore(session.subscribe, () => session.status)
  const { namespace, pod, container } = session.target
  const live = status.state === 'open' || status.state === 'connecting' || status.state === 'loading'
  return (
    <div className={clsx('term-panel__tab', active && 'term-panel__tab--active', !live && 'term-panel__tab--ended')}>
      <button
        type="button"
        role="tab"
        aria-selected={active}
        className="term-panel__tab-label"
        title={`${namespace}/${pod} · ${container}`}
        onClick={() => activatePanelSession(session.id)}
      >
        <Icon name="terminal" size={14} />
        <span className="term-panel__tab-pod">{pod}</span>
        <span className="term-panel__tab-container">{container}</span>
      </button>
      <button
        type="button"
        className="term-panel__tab-close"
        title={t('shell.closeSession')}
        aria-label={t('shell.closeSessionOf', { pod, container })}
        onClick={() => closePanelSession(session.id)}
      >
        <Icon name="close" size={14} />
      </button>
    </div>
  )
}
