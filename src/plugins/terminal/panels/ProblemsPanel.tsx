import { useTranslation } from 'react-i18next'
import { dataMode } from '@/data/mode'
import { EmptyState } from '@/shared/EmptyState'
import { Icon } from '@/shared/Icon'
import { NS } from '../i18n'
import './panels.css'

/** Static sample problems for mock mode; nothing computes problems for a live cluster. */
const PROBLEMS = [
  {
    severity: 'error',
    icon: 'error',
    message: 'Pod shop/shop-payments-79c6cd8b96-t2m8j is in CrashLoopBackOff (14 restarts)',
    source: 'workloads',
  },
  {
    severity: 'error',
    icon: 'error',
    message: 'Helm release "shop-app" revision 7 failed: post-upgrade hooks failed',
    source: 'helm',
  },
  {
    severity: 'warning',
    icon: 'warning',
    message: 'Pod shop/shop-frontend-745bd7c77b-w9tqk Pending for 26m (0/3 nodes schedulable: insufficient cpu)',
    source: 'workloads',
  },
  {
    severity: 'warning',
    icon: 'warning',
    message: 'Certificate default/staging-tls not Ready for 2h (order pending)',
    source: 'cert-manager',
  },
  {
    severity: 'warning',
    icon: 'warning',
    message: 'Prometheus target shop-payments:9102 is down',
    source: 'metrics',
  },
]

export function ProblemsPanel() {
  const { t } = useTranslation(NS)

  // Live mode never shows made-up data, and there is no live problem source.
  if (dataMode.mode !== 'mock') {
    return <EmptyState icon="info" title={t('problems.unavailable')} />
  }

  return (
    <div className="problems">
      {PROBLEMS.map((p, i) => (
        <div key={i} className="problems__row">
          <Icon
            name={p.icon}
            size={14}
            className={p.severity === 'error' ? 'problems__icon--error' : 'problems__icon--warning'}
          />
          <span className="problems__message">{p.message}</span>
          <span className="problems__source">{p.source}</span>
        </div>
      ))}
    </div>
  )
}
