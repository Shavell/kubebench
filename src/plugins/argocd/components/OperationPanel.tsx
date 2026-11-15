import { useTranslation } from 'react-i18next'
import { formatDateTime } from '@/i18n/format'
import { EmptyState } from '@/shared/EmptyState'
import type { ArgoOperationState } from '../data/types'
import { NS } from '../i18n'
import { shortRevision } from '../text'
import { StateBadge } from './StatusBadges'

/** The current or last operation (status.operationState) and its per-resource results. */
export function OperationPanel({ operation }: { operation: ArgoOperationState | null }) {
  const { t } = useTranslation(NS)
  if (!operation) return <EmptyState icon="sync" title={t('detail.operation.none')} />
  const flags = [operation.dryRun && t('detail.operation.dryRun'), operation.prune && t('detail.operation.prune')].filter(Boolean)

  return (
    <>
      <div className="detail__grid">
        <span className="detail__label">{t('detail.operation.phase')}</span>
        <span className="detail__value">
          <StateBadge value={operation.phase} />
        </span>
        <span className="detail__label">{t('detail.operation.message')}</span>
        <span className="detail__value">{operation.message || '—'}</span>
        <span className="detail__label">{t('detail.operation.revision')}</span>
        <span className="detail__value argocd-mono" title={operation.revision}>
          {operation.revision ? shortRevision(operation.revision) : '—'}
        </span>
        <span className="detail__label">{t('detail.operation.initiatedBy')}</span>
        <span className="detail__value">{operation.automated ? t('detail.automated') : operation.initiatedBy || '—'}</span>
        <span className="detail__label">{t('detail.operation.startedAt')}</span>
        <span className="detail__value">{operation.startedAt ? formatDateTime(operation.startedAt) : '—'}</span>
        <span className="detail__label">{t('detail.operation.finishedAt')}</span>
        <span className="detail__value">{operation.finishedAt ? formatDateTime(operation.finishedAt) : '—'}</span>
        <span className="detail__label">{t('detail.operation.options')}</span>
        <span className="detail__value">{flags.length ? flags.join(', ') : '—'}</span>
        {operation.retryCount > 0 && (
          <>
            <span className="detail__label">{t('detail.operation.retries')}</span>
            <span className="detail__value">{operation.retryCount}</span>
          </>
        )}
      </div>
      {operation.resources.length > 0 && (
        <>
          <div className="detail__section">{t('detail.operation.results')}</div>
          <table className="kb-table argocd-table">
            <thead>
              <tr>
                <th>{t('detail.operation.kind')}</th>
                <th>{t('detail.operation.resource')}</th>
                <th>{t('detail.operation.status')}</th>
                <th>{t('detail.operation.phaseColumn')}</th>
                <th>{t('detail.operation.message')}</th>
              </tr>
            </thead>
            <tbody>
              {operation.resources.map((r) => (
                <tr key={`${r.group}/${r.kind}/${r.namespace}/${r.name}/${r.syncPhase}`}>
                  <td>{r.kind}</td>
                  <td>{r.namespace ? `${r.namespace}/${r.name}` : r.name}</td>
                  <td>
                    <StateBadge value={r.status} />
                  </td>
                  <td>{r.syncPhase}</td>
                  <td>{r.message}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </>
      )}
    </>
  )
}
