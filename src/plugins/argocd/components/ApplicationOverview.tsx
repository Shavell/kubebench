import { useTranslation } from 'react-i18next'
import { formatDateTime } from '@/i18n/format'
import type { ArgoApplicationDetail } from '../data/types'
import { NS } from '../i18n'
import { destinationText, shortRevision, sourceText } from '../text'
import { StateBadge } from './StatusBadges'

/** Status and configuration of an application, above its tabs. */
export function ApplicationOverview({ app }: { app: ArgoApplicationDetail }) {
  const { t } = useTranslation(NS)
  const policy = app.syncPolicy
  const auto = policy.automated
    ? [t('overview.autoOn'), policy.prune && t('overview.autoPrune'), policy.selfHeal && t('overview.selfHeal')]
        .filter(Boolean)
        .join(', ')
    : t('overview.autoOff')

  return (
    <div className="detail__grid argocd-overview">
      <span className="detail__label">{t('overview.status')}</span>
      <span className="detail__value argocd-badges">
        <StateBadge value={app.sync} />
        <StateBadge value={app.health} />
        <StateBadge value={app.operationPhase} />
        {app.deleting && <StateBadge value="Deleting" />}
      </span>
      {app.healthMessage && (
        <>
          <span className="detail__label">{t('overview.healthMessage')}</span>
          <span className="detail__value">{app.healthMessage}</span>
        </>
      )}
      <span className="detail__label">{t('overview.project')}</span>
      <span className="detail__value">{app.project}</span>
      <span className="detail__label">{app.sources.length > 1 ? t('overview.sources') : t('overview.source')}</span>
      <span className="detail__value argocd-mono">
        {app.sources.length === 0
          ? '—'
          : app.sources.map((s, i) => (
              <div key={i} title={s.repoURL}>
                {sourceText(s)}
                {s.ref && ` (${s.ref})`}
              </div>
            ))}
      </span>
      <span className="detail__label">{t('overview.destination')}</span>
      <span className="detail__value">{destinationText(app.destination)}</span>
      <span className="detail__label">{t('overview.revision')}</span>
      <span className="detail__value argocd-mono" title={app.revision}>
        {app.revision ? shortRevision(app.revision) : '—'}
      </span>
      <span className="detail__label">{t('overview.autoSync')}</span>
      <span className="detail__value">{auto}</span>
      <span className="detail__label">{t('overview.syncOptions')}</span>
      <span className="detail__value argocd-mono">{policy.syncOptions.length ? policy.syncOptions.join(', ') : '—'}</span>
      <span className="detail__label">{t('overview.lastSynced')}</span>
      <span className="detail__value">{app.lastSyncedAt ? formatDateTime(app.lastSyncedAt) : '—'}</span>
      <span className="detail__label">{t('overview.reconciled')}</span>
      <span className="detail__value">{app.reconciledAt ? formatDateTime(app.reconciledAt) : '—'}</span>
      {app.images.length > 0 && (
        <>
          <span className="detail__label">{t('overview.images')}</span>
          <span className="detail__value argocd-mono">
            {app.images.map((image) => (
              <div key={image}>{image}</div>
            ))}
          </span>
        </>
      )}
      {app.urls.length > 0 && (
        <>
          <span className="detail__label">{t('overview.urls')}</span>
          <span className="detail__value">
            {app.urls.map((url) => (
              <div key={url}>
                <a href={url} target="_blank" rel="noreferrer noopener">
                  {url}
                </a>
              </div>
            ))}
          </span>
        </>
      )}
    </div>
  )
}
