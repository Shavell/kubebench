import './panels.css'

const LOG_LINES = [
  ['14:29:41.102', 'INFO', 'shop-payments', 'starting payment service v3.1.0-rc.2'],
  ['14:29:41.288', 'INFO', 'shop-payments', 'connecting to gateway https://gw.pay.example.com'],
  ['14:29:44.301', 'WARN', 'shop-payments', 'gateway handshake slow (3.0s > 2.5s budget)'],
  ['14:29:47.312', 'ERROR', 'shop-payments', 'gateway healthcheck failed: context deadline exceeded'],
  ['14:29:47.315', 'ERROR', 'shop-payments', 'startup probe failed, exiting'],
  ['14:30:12.001', 'INFO', 'shop-frontend', 'GET /api/products 200 12ms'],
  ['14:30:12.410', 'INFO', 'shop-cart', 'cart 8f3a2c updated (3 items)'],
  ['14:30:13.518', 'INFO', 'shop-frontend', 'GET /api/cart 200 4ms'],
  ['14:30:15.223', 'WARN', 'shop-frontend', 'POST /api/checkout 502 upstream shop-payments unavailable'],
  ['14:30:15.224', 'INFO', 'shop-frontend', 'retrying checkout via fallback queue'],
  ['14:30:18.930', 'INFO', 'prometheus', 'scrape target shop-payments:9102 down'],
  ['14:30:22.104', 'INFO', 'shop-cart', 'session gc: 214 expired carts removed'],
]

const LEVEL_CLASS: Record<string, string> = {
  INFO: 'log__level--info',
  WARN: 'log__level--warn',
  ERROR: 'log__level--error',
}

export function LogsPanel() {
  return (
    <pre className="term">
      {LOG_LINES.map(([ts, level, source, message], i) => (
        <div key={i}>
          <span className="log__ts">{ts}</span>{' '}
          <span className={LEVEL_CLASS[level]}>{level.padEnd(5)}</span>{' '}
          <span className="log__source">[{source}]</span> {message}
        </div>
      ))}
    </pre>
  )
}
