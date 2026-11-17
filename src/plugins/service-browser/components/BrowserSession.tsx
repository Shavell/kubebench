import { useEffect, useMemo, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Unavailable } from '@/shared/Unavailable'
import { serviceBrowserData } from '../data'
import type { BrowserPort, BrowserTarget, ProxySession, Scheme } from '../data/types'
import { NS } from '../i18n'
import { frameLocation } from '../model/frameLocation'
import { canGoBack, canGoForward, currentPath, goBack, goForward, startHistory, visit } from '../model/history'
import { targetAddress } from '../model/ports'
import { AddressBar } from './AddressBar'
import { SessionNotices } from './SessionNotices'

/**
 * The sandbox of proxied content on KubeBench's own origin; the backend
 * sends the same flags in the pages' Content-Security-Policy
 * (svcproxy.SandboxFlags). Without allow-same-origin the page runs in an
 * opaque origin and cannot reach KubeBench's storage or act as the user.
 */
const SANDBOX = 'allow-scripts allow-forms allow-popups allow-downloads'
/** On the separate proxy origin the page keeps its (foreign) origin; top navigation stays blocked. */
const PROXY_ORIGIN_SANDBOX = `${SANDBOX} allow-same-origin allow-modals allow-popups-to-escape-sandbox`

interface BrowserSessionProps {
  target: BrowserTarget
  ports: BrowserPort[]
  onTargetChange: (port: number, scheme: Scheme) => void
}

type SessionState = { session: ProxySession; error: null } | { session: null; error: unknown } | null

/**
 * One browsing session of a target: opens it on mount, ends it on unmount,
 * and keeps the tab's own back/forward history from the documents the
 * backend saw the frame load (the frame is cross-origin).
 */
export function BrowserSession({ target, ports, onTargetChange }: BrowserSessionProps) {
  const { t } = useTranslation(NS)
  const provider = serviceBrowserData()
  const [state, setState] = useState<SessionState>(null)
  const [attempt, setAttempt] = useState(0)
  const [history, setHistory] = useState(() => startHistory('/'))
  // Every navigation KubeBench starts remounts the frame (n), so going to
  // the path the frame was opened with still loads it again.
  const [frame, setFrame] = useState({ path: '/', n: 0 })
  // The newest navigation of the session seen at a load of the frame.
  const seen = useRef({ token: '', seq: 0 })

  useEffect(() => {
    let cancelled = false
    let token: string | null = null
    provider.open(target).then(
      (session) => {
        if (cancelled) {
          provider.close(session.token).catch(() => undefined)
          return
        }
        token = session.token
        setState({ session, error: null })
      },
      (error: unknown) => {
        if (!cancelled) setState({ session: null, error })
      },
    )
    return () => {
      cancelled = true
      // Ending the session is best effort: it also expires on its own.
      if (token) provider.close(token).catch(() => undefined)
    }
    // The target's fields, not its identity, select the session.
  }, [provider, target.kind, target.namespace, target.name, target.port, target.scheme, attempt]) // eslint-disable-line react-hooks/exhaustive-deps

  const session = state?.session ?? null
  const source = useMemo(
    () => (session ? provider.frame(session, frame.path) : null),
    // frame.n reloads the same path.
    [provider, session, frame.path, frame.n], // eslint-disable-line react-hooks/exhaustive-deps
  )

  if (state?.error) {
    return <Unavailable error={state.error} onRetry={() => setAttempt((n) => n + 1)} />
  }

  const navigate = (path: string) => {
    setHistory((h) => visit(h, path))
    setFrame((f) => ({ path, n: f.n + 1 }))
  }
  const show = (next: typeof history) => {
    setHistory(next)
    setFrame((f) => ({ path: currentPath(next), n: f.n + 1 }))
  }
  const onFrameLoad = () => {
    if (!session) return
    provider.session(session.token).then(
      (now) => {
        const since = seen.current.token === now.token ? seen.current.seq : 0
        const load = frameLocation(now.navigations, since)
        seen.current = { token: now.token, seq: load.seen }
        const location = load.location
        if (location) setHistory((h) => visit(h, location))
      },
      // The session ended (expired, signed out): the frame shows why.
      () => undefined,
    )
  }
  const path = currentPath(history)
  const external = session && session.base !== '' ? session.base + path : null

  return (
    <div className="sb-page">
      <AddressBar
        address={targetAddress(target)}
        path={path}
        ports={ports}
        target={target}
        canBack={canGoBack(history)}
        canForward={canGoForward(history)}
        externalUrl={external}
        onBack={() => show(goBack(history))}
        onForward={() => show(goForward(history))}
        onReload={() => setFrame((f) => ({ path, n: f.n + 1 }))}
        onNavigate={navigate}
        onTargetChange={onTargetChange}
      />
      {session && <SessionNotices session={session} />}
      <div className="sb-frame-host">
        {session && source ? (
          <iframe
            key={frame.n}
            className="sb-frame"
            title={t('frame.title', { address: targetAddress(target) + path })}
            sandbox={session.sandboxed ? SANDBOX : PROXY_ORIGIN_SANDBOX}
            referrerPolicy="no-referrer"
            onLoad={onFrameLoad}
            {...source}
          />
        ) : (
          <div className="page-loading">{t('page.opening')}</div>
        )}
      </div>
    </div>
  )
}
