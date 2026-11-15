import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery } from '@/data/DataContext'
import { i18n } from '@/i18n'
import { formatNumber } from '@/i18n/format'
import { Button } from '@/shared/Button'
import { formatAge, recentWarnings, toEventRow } from '@/shared/eventModel'
import { Icon } from '@/shared/Icon'
import { Unavailable } from '@/shared/Unavailable'
import { NS } from '../i18n'

/** Warnings shown on the card; the Events view lists all of them. */
const LIMIT = 8

/** The workloads plugin's Events list (tab kind `generic-list`), filtered to warnings. */
function openWarnings(): void {
  useLayoutStore.getState().openTab({
    id: 'reslist:events:warnings',
    title: i18n.t('overview.warnings.tab', { ns: NS }),
    titleKey: 'overview.warnings.tab',
    titleNs: NS,
    icon: 'warning',
    kind: 'generic-list',
    props: { resourceKey: 'events', warningsOnly: true },
  })
}

/** Overview card: the cluster's most recent Warning events, newest first. */
export function RecentWarnings() {
  const { t } = useTranslation(NS)
  const query = useProviderQuery((p) => p.cluster.listResource('events'), [])

  const { shown, total } = useMemo(() => {
    const now = Date.now()
    const rows = (query.data ?? []).map((o) => toEventRow(o, now))
    const warnings = recentWarnings(rows, Infinity)
    return { shown: warnings.slice(0, LIMIT), total: warnings.length }
  }, [query.data])

  const body = () => {
    if (query.error) return <Unavailable compact code={query.errorCode} reason={query.error} title={t('overview.warnings.unavailable')} />
    if (query.loading && !query.data) return <div className="metrics-warnings__empty">{t('overview.warnings.loading')}</div>
    if (shown.length === 0) {
      return (
        <div className="metrics-warnings__empty">
          <Icon name="pass" size={14} /> {t('overview.warnings.none')}
        </div>
      )
    }
    return (
      <ul className="metrics-warnings__list">
        {shown.map((w) => (
          <li key={w.key} className="metrics-warnings__item">
            <Icon name="warning" size={14} className="metrics-warnings__icon" />
            <div className="metrics-warnings__text">
              <div className="metrics-warnings__line">
                <span className="metrics-warnings__reason">{w.reason}</span>
                <span className="metrics-warnings__object">
                  {w.namespace ? `${w.namespace}/${w.object}` : w.object}
                </span>
              </div>
              <div className="metrics-warnings__message" title={w.message}>
                {w.message}
              </div>
            </div>
            <div className="metrics-warnings__meta">
              {w.count > 1 && <span>×{formatNumber(w.count)}</span>}
              <span>{formatAge(w.lastSeenMinutes)}</span>
            </div>
          </li>
        ))}
      </ul>
    )
  }

  return (
    <section className="metrics-warnings" aria-labelledby="metrics-warnings-title">
      <div className="metrics-warnings__header">
        <h2 id="metrics-warnings-title" className="metrics-allocation__title">
          {t('overview.warnings.title')}
        </h2>
        {total > 0 && (
          <span className="metrics-warnings__count">
            {t('overview.warnings.count', { count: total, value: formatNumber(total) })}
          </span>
        )}
        <Button variant="secondary" icon="list-flat" onClick={openWarnings}>
          {t('overview.warnings.viewAll')}
        </Button>
      </div>
      {body()}
    </section>
  )
}
