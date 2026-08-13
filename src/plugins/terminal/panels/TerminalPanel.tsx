import { useTerminalStore } from '../terminalStore'
import { ExecView } from '../ExecView'
import './panels.css'

const TRANSCRIPT = [
  { prompt: true, text: 'kubectl get nodes' },
  { text: 'NAME               STATUS   ROLES           AGE   VERSION' },
  { text: 'kb-control-plane   Ready    control-plane   92d   v1.30.2' },
  { text: 'kb-worker-1        Ready    <none>          92d   v1.30.2' },
  { text: 'kb-worker-2        Ready    <none>          61d   v1.30.2' },
]

/**
 * Bottom-panel Terminal. Becomes an interactive container shell when a
 * pod is targeted (via a pod row's "Shell" action), otherwise shows a
 * static sample transcript.
 */
export function TerminalPanel() {
  const target = useTerminalStore((s) => s.execTarget)
  const nonce = useTerminalStore((s) => s.execNonce)

  if (target) {
    return (
      <ExecView
        namespace={target.namespace}
        pod={target.pod}
        container={target.container}
        resetKey={nonce}
      />
    )
  }

  return (
    <pre className="term">
      <div className="log__hint">Right-click a pod → Shell to open a container terminal here.</div>
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
    </pre>
  )
}
