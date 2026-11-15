import { useTranslation } from 'react-i18next'
import { formatDateTime } from '@/i18n/format'
import { EmptyState } from '@/shared/EmptyState'
import type { ArgoCondition } from '../data/types'
import { NS } from '../i18n'

/** Application or ApplicationSet conditions (errors and warnings Argo CD reports). */
export function ConditionsTable({ conditions }: { conditions: ArgoCondition[] }) {
  const { t } = useTranslation(NS)
  if (conditions.length === 0) return <EmptyState icon="pass" title={t('conditions.empty')} />
  return (
    <table className="kb-table argocd-table">
      <thead>
        <tr>
          <th>{t('conditions.type')}</th>
          <th>{t('conditions.message')}</th>
          <th>{t('conditions.since')}</th>
        </tr>
      </thead>
      <tbody>
        {conditions.map((c, i) => (
          <tr key={`${c.type}-${i}`}>
            <td className={/Error$/.test(c.type) ? 'argocd-error' : undefined}>{c.type}</td>
            <td className="argocd-message">{c.message || c.reason || '—'}</td>
            <td>{c.lastTransitionTime ? formatDateTime(c.lastTransitionTime) : '—'}</td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
