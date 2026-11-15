import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { formatDateTime } from '@/i18n/format'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import type { ArgoApplicationDetail, ArgoHistoryEntry } from '../data/types'
import { NS } from '../i18n'
import { shortRevision, sourceText } from '../text'

interface HistoryTableProps {
  app: ArgoApplicationDetail
  onRollback: (entry: ArgoHistoryEntry) => void
}

/**
 * Deployment history, newest first. Every entry but the current one can
 * be rolled back to; Argo CD refuses that while automated sync is on, so
 * the button is disabled then, with the reason.
 */
export function HistoryTable({ app, onRollback }: HistoryTableProps) {
  const { t } = useTranslation(NS)
  const write = usePermission('write')
  if (app.history.length === 0) {
    return <EmptyState icon="history" title={t('detail.history.empty')} />
  }
  const blocked = !write.allowed ? write.reason : app.autoSync ? t('detail.history.autoSyncBlocks') : app.operating ? t('detail.operationRunning') : null

  return (
    <table className="kb-table argocd-table">
      <thead>
        <tr>
          <th>{t('detail.history.id')}</th>
          <th>{t('detail.history.revision')}</th>
          <th>{t('detail.history.source')}</th>
          <th>{t('detail.history.deployedAt')}</th>
          <th>{t('detail.history.initiatedBy')}</th>
          <th aria-label={t('detail.history.actions')}></th>
        </tr>
      </thead>
      <tbody>
        {app.history.map((entry, index) => (
          <tr key={entry.id}>
            <td>{entry.id}</td>
            <td className="argocd-mono" title={entry.revision || entry.revisions.join(', ')}>
              {entry.revision ? shortRevision(entry.revision) : entry.revisions.map(shortRevision).join(', ')}
            </td>
            <td className="argocd-mono">{entry.sources.map((s) => sourceText(s)).join(' + ')}</td>
            <td>{entry.deployedAt ? formatDateTime(entry.deployedAt) : '—'}</td>
            <td>{entry.automated ? t('detail.automated') : entry.initiatedBy || '—'}</td>
            <td>
              {index === 0 ? (
                <span className="argocd-muted">{t('detail.history.current')}</span>
              ) : (
                <Button
                  variant="secondary"
                  icon="history"
                  disabled={blocked !== null}
                  title={blocked ?? t('detail.history.rollbackHint', { id: entry.id })}
                  onClick={() => onRollback(entry)}
                >
                  {t('detail.history.rollback')}
                </Button>
              )}
            </td>
          </tr>
        ))}
      </tbody>
    </table>
  )
}
