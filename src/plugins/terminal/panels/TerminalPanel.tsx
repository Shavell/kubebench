import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { usePanelRequest } from '@/core/plugin/panelRequests'
import { dataMode } from '@/data/mode'
import { NS } from '../i18n'
import { ExecView } from '../ExecView'
import { podTarget } from './podTarget'
import './panels.css'

const TRANSCRIPT = [
  { prompt: true, text: 'kubectl get nodes' },
  { text: 'NAME               STATUS   ROLES           AGE   VERSION' },
  { text: 'kb-control-plane   Ready    control-plane   92d   v1.30.2' },
  { text: 'kb-worker-1        Ready    <none>          92d   v1.30.2' },
  { text: 'kb-worker-2        Ready    <none>          61d   v1.30.2' },
]

/**
 * Bottom-panel Terminal. Becomes an interactive container shell once a
 * `pod-exec` request arrives (e.g. a pod row's "Shell" action), otherwise shows a
 * hint (or why shell access is denied) plus, in mock mode only, a static
 * sample transcript (live mode never shows made-up output).
 */
export function TerminalPanel() {
  const { t } = useTranslation(NS)
  const exec = usePermission('exec')
  const request = usePanelRequest('pod-exec')
  const target = podTarget(request)

  if (target) {
    return (
      <ExecView
        namespace={target.namespace}
        pod={target.pod}
        container={target.container}
        // Every Shell request starts a fresh session, even for the same pod.
        resetKey={request?.seq}
      />
    )
  }

  return (
    <pre className="term">
      <div className="log__hint">
        {exec.allowed ? t('exec.hint') : t('exec.denied', { reason: exec.reason })}
      </div>
      {dataMode.mode === 'mock' && (
        <>
          {TRANSCRIPT.map((line, i) =>
            line.prompt ? (
              <div key={i}>
                <span className="term__prompt">$ </span>
                <span className="term__cmd">{line.text}</span>
              </div>
            ) : (
              <div key={i}>{line.text}</div>
            ),
          )}
          <div>
            <span className="term__prompt">$ </span>
            <span className="term__cursor" />
          </div>
        </>
      )}
    </pre>
  )
}
