import { useTranslation } from 'react-i18next'
import { usePanelRequest } from '@/core/plugin/panelRequests'
import { dataMode } from '@/data/mode'
import { NS } from '../i18n'
import { LogsView } from '../LogsView'
import { podTarget } from './podTarget'
import './panels.css'

/**
 * Bottom-panel Logs. Shows a pod's logs once a `pod-logs` request arrives
 * (e.g. a pod row's "Logs" action), otherwise a hint plus, in mock mode
 * only, a static sample (live mode never shows made-up lines).
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
  const { t } = useTranslation(NS)
  const target = podTarget(usePanelRequest('pod-logs'))

  if (target) {
    return <LogsView namespace={target.namespace} pod={target.pod} container={target.container} compact />
  }

  return (
    <pre className="term">
      <div className="log__hint">{t('logs.hint')}</div>
      {dataMode.mode === 'mock' &&
        SAMPLE_LINES.map(([ts, level, source, message], i) => (
          <div key={i}>
            <span className="log__ts">{ts}</span>{' '}
            <span className={LEVEL_CLASS[level]}>{level.padEnd(5)}</span>{' '}
            <span className="log__source">[{source}]</span> {message}
          </div>
        ))}
    </pre>
  )
}
