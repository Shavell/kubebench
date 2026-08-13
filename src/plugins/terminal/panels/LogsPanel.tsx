import { useTerminalStore } from '../terminalStore'
import { LogsView } from '../LogsView'
import './panels.css'

/**
 * Bottom-panel Logs. Shows a selected pod's live logs when a target is
 * set (via a pod row's "Logs" action), otherwise a static sample.
 */
const SAMPLE_LINES = [
  ['14:30:12.001', 'INFO', 'shop-frontend', 'GET /api/products 200 12ms'],
  ['14:30:12.410', 'INFO', 'shop-cart', 'cart 8f3a2c updated (3 items)'],
  ['14:30:15.223', 'WARN', 'shop-frontend', 'POST /api/checkout 502 upstream shop-payments unavailable'],
  ['14:30:18.930', 'INFO', 'prometheus', 'scrape target shop-payments:9102 down'],
]

const LEVEL_CLASS: Record<string, string> = {
  INFO: 'log__level--info',
  WARN: 'log__level--warn',
  ERROR: 'log__level--error',
}

export function LogsPanel() {
  const target = useTerminalStore((s) => s.logsTarget)

  if (target) {
    return <LogsView namespace={target.namespace} pod={target.pod} container={target.container} compact />
  }

  return (
    <pre className="term">
      <div className="log__hint">Right-click a pod → Logs to stream its output here.</div>
      {SAMPLE_LINES.map(([ts, level, source, message], i) => (
        <div key={i}>
          <span className="log__ts">{ts}</span>{' '}
          <span className={LEVEL_CLASS[level]}>{level.padEnd(5)}</span>{' '}
          <span className="log__source">[{source}]</span> {message}
        </div>
      ))}
    </pre>
  )
}
