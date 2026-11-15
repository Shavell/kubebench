import { useTranslation } from 'react-i18next'
import { CORE_NS } from '@/i18n'
import { Badge } from '@/shared/Badge'
import type { HelmReleaseDetail } from '../../data'
import { DateTime } from '../../DateTime'
import { NS } from '../../i18n'

/** Overview tab: the release, its chart's metadata and its hooks. */
export function OverviewTab({ release }: { release: HelmReleaseDetail }) {
  const { t } = useTranslation(NS)
  const chart = release.chartInfo
  return (
    <div className="helm-pane__scroll">
      <div className="detail__grid helm-grid">
        <span className="detail__label">{t('common.status', { ns: CORE_NS })}</span>
        <span className="detail__value">
          <Badge>{release.status}</Badge>
        </span>
        <span className="detail__label">{t('release.fields.namespace')}</span>
        <span className="detail__value">{release.namespace}</span>
        <span className="detail__label">{t('release.fields.chart')}</span>
        <span className="detail__value">{`${release.chart}-${release.chartVersion}`}</span>
        <span className="detail__label">{t('release.fields.appVersion')}</span>
        <span className="detail__value">{release.appVersion || '—'}</span>
        <span className="detail__label">{t('release.fields.revision')}</span>
        <span className="detail__value">{String(release.revision)}</span>
        <span className="detail__label">{t('release.fields.firstDeployed')}</span>
        <span className="detail__value">
          <DateTime value={release.firstDeployed} />
        </span>
        <span className="detail__label">{t('release.fields.updated')}</span>
        <span className="detail__value">
          <DateTime value={release.updated} />
        </span>
        <span className="detail__label">{t('release.fields.description')}</span>
        <span className="detail__value">{release.description || '—'}</span>
      </div>

      <div className="detail__section">{t('release.chart.title')}</div>
      <div className="detail__grid helm-grid">
        <span className="detail__label">{t('release.chart.description')}</span>
        <span className="detail__value">{chart.description || '—'}</span>
        {chart.home && (
          <>
            <span className="detail__label">{t('release.chart.home')}</span>
            <span className="detail__value">
              <a href={chart.home} target="_blank" rel="noreferrer noopener">
                {chart.home}
              </a>
            </span>
          </>
        )}
        {chart.sources.length > 0 && (
          <>
            <span className="detail__label">{t('release.chart.sources')}</span>
            <span className="detail__value">
              {chart.sources.map((source) => (
                <div key={source}>{source}</div>
              ))}
            </span>
          </>
        )}
        {chart.keywords.length > 0 && (
          <>
            <span className="detail__label">{t('release.chart.keywords')}</span>
            <span className="detail__value">
              {chart.keywords.map((k) => (
                <span key={k} className="detail__chip">
                  {k}
                </span>
              ))}
            </span>
          </>
        )}
        {chart.kubeVersion && (
          <>
            <span className="detail__label">{t('release.chart.kubeVersion')}</span>
            <span className="detail__value">{chart.kubeVersion}</span>
          </>
        )}
        {chart.deprecated && (
          <>
            <span className="detail__label">{t('release.chart.deprecated')}</span>
            <span className="detail__value text-warning">{t('release.chart.deprecatedHint')}</span>
          </>
        )}
      </div>

      <div className="detail__section">{t('release.hooks.title')}</div>
      {release.hooks.length === 0 ? (
        <div className="helm-muted">{t('release.hooks.none')}</div>
      ) : (
        <table className="kb-table" style={{ maxWidth: 980 }}>
          <thead>
            <tr>
              <th>{t('common.name', { ns: CORE_NS })}</th>
              <th>{t('release.hooks.kind')}</th>
              <th>{t('release.hooks.events')}</th>
              <th>{t('release.hooks.lastRun')}</th>
              <th>{t('release.hooks.completed')}</th>
            </tr>
          </thead>
          <tbody>
            {release.hooks.map((hook) => (
              <tr key={`${hook.kind}/${hook.name}`}>
                <td>{hook.name}</td>
                <td>{hook.kind}</td>
                <td>{hook.events.join(', ')}</td>
                <td>{hook.phase ? <Badge>{hook.phase}</Badge> : t('release.hooks.neverRun')}</td>
                <td>
                  <DateTime value={hook.completedAt} />
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
    </div>
  )
}
