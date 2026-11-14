import { useEffect, useRef, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { useProviders } from '@/data/DataContext'
import { dataMode } from '@/data/mode'
import { NS } from './i18n'
import './PodExecTab.css'

interface TermLine {
  prompt?: boolean
  text: string
}

/**
 * Reusable command shell for one container; used by the editor tab and
 * the bottom panel. Each command runs via execCommand (real one-shot
 * `kubectl exec` in live mode, canned mock otherwise). `resetKey` starts
 * a fresh session when it changes. Needs exec permission: without it the
 * view only explains why (read-only cluster or viewer role).
 */
export function ExecView({
  namespace,
  pod,
  container,
  resetKey,
}: {
  namespace: string
  pod: string
  container: string
  resetKey?: unknown
}) {
  const { t } = useTranslation(NS)
  const exec = usePermission('exec')
  const providers = useProviders()
  const promptText = `${pod.slice(0, 24)}:/#`
  const [lines, setLines] = useState<TermLine[]>([])
  const [input, setInput] = useState('')
  const [busy, setBusy] = useState(false)
  const [closed, setClosed] = useState(false)
  const inputRef = useRef<HTMLInputElement>(null)
  const bodyRef = useRef<HTMLDivElement>(null)

  // (Re)start the session whenever the target or resetKey changes. The
  // banner is part of the transcript, so it keeps the language it started in.
  useEffect(() => {
    const mode = t(dataMode.mode === 'live' ? 'exec.modeLive' : 'exec.modeMock')
    setLines([{ text: `kubectl exec -it ${pod} -c ${container} -- sh   (${mode})` }])
    setClosed(false)
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [namespace, pod, container, resetKey])

  useEffect(() => {
    bodyRef.current?.scrollTo({ top: bodyRef.current.scrollHeight })
  }, [lines])

  if (!exec.allowed) {
    return (
      <div className="podexec">
        <div className="podexec__body">{t('exec.denied', { reason: exec.reason })}</div>
      </div>
    )
  }

  const onKeyDown = async (e: React.KeyboardEvent<HTMLInputElement>) => {
    if (e.key !== 'Enter' || closed || busy) return
    const cmd = input.trim()
    setInput('')
    if (cmd === 'clear') {
      setLines([])
      return
    }
    if (cmd === 'exit') {
      setLines((prev) => [...prev, { prompt: true, text: cmd }, { text: t('exec.ended') }])
      setClosed(true)
      return
    }
    setLines((prev) => [...prev, { prompt: true, text: cmd }])
    if (!cmd) return
    setBusy(true)
    try {
      const output = await providers.cluster.execCommand(namespace, pod, container, cmd)
      setLines((prev) => [...prev, ...output.map((text) => ({ text }))])
    } catch (err) {
      setLines((prev) => [...prev, { text: err instanceof Error ? err.message : String(err) }])
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="podexec" onClick={() => inputRef.current?.focus()}>
      <div className="podexec__body" ref={bodyRef}>
        {lines.map((line, i) =>
          line.prompt ? (
            <div key={i}>
              <span className="podexec__prompt">{promptText} </span>
              <span className="podexec__cmd">{line.text}</span>
            </div>
          ) : (
            <div key={i}>{line.text}</div>
          ),
        )}
        {!closed && (
          <div className="podexec__inputline">
            <span className="podexec__prompt">{busy ? '…' : promptText} </span>
            <input
              ref={inputRef}
              className="podexec__input"
              value={input}
              onChange={(e) => setInput(e.target.value)}
              onKeyDown={onKeyDown}
              disabled={busy}
              spellCheck={false}
              autoComplete="off"
              aria-label={t('exec.inputLabel')}
            />
          </div>
        )}
      </div>
    </div>
  )
}
