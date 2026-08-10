import './panels.css'

const TRANSCRIPT = [
  { prompt: true, text: 'kubectl get nodes' },
  { text: 'NAME               STATUS   ROLES           AGE   VERSION' },
  { text: 'kb-control-plane   Ready    control-plane   92d   v1.30.2' },
  { text: 'kb-worker-1        Ready    <none>          92d   v1.30.2' },
  { text: 'kb-worker-2        Ready    <none>          61d   v1.30.2' },
  { prompt: true, text: 'kubectl get pods -n shop' },
  { text: 'NAME                              READY   STATUS             RESTARTS      AGE' },
  { text: 'shop-frontend-745bd7c77b-2fkxx    1/1     Running            0             3d' },
  { text: 'shop-frontend-745bd7c77b-8pl6m    1/1     Running            0             3d' },
  { text: 'shop-frontend-745bd7c77b-w9tqk    0/1     Pending            0             26m' },
  { text: 'shop-cart-6f8d9b5c44-nr7vd        1/1     Running            0             3d' },
  { text: 'shop-cart-6f8d9b5c44-qh5zs        1/1     Running            0             3d' },
  { text: 'shop-payments-79c6cd8b96-t2m8j    0/1     CrashLoopBackOff   14 (2m ago)   41m' },
  { text: 'shop-postgres-0                   1/1     Running            0             45d' },
]

export function TerminalPanel() {
  return (
    <pre className="term">
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
