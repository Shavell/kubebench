import { useMemo } from 'react'
import { useTranslation } from 'react-i18next'
import { usePluginQuery } from '@/data/pluginData'
import { formatDateTime, formatNumber } from '@/i18n/format'
import { StatTile } from '@/shared/charts/StatTile'
import { EmptyState } from '@/shared/EmptyState'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { securityData } from '../data'
import { SEVERITIES, type Finding, type SecurityReport, type Severity } from '../data/types'
import { NS } from '../i18n'
import { SEVERITY_COLORS, scoreColor } from '../model/severity'
import { openFindings } from '../tabs'
import { CategoryMultiples } from './CategoryMultiples'
import { ExportButtons, ScanControls, SeverityBadge } from './parts'
import { ReportNotices } from './ReportNotices'
import { useSecurityTexts } from './texts'
import '@/shared/tabPage.css'
import '../security.css'

interface CheckRow {
  checkId: string
  title: string
  severity: Severity
  open: number
}

/** The checks with the most open findings, most severe first. */
function topChecks(findings: Finding[], limit: number): CheckRow[] {
  const rows = new Map<string, CheckRow>()
  for (const f of findings) {
    if (f.accepted) continue
    const row = rows.get(f.checkId) ?? { checkId: f.checkId, title: f.title, severity: f.severity, open: 0 }
    row.open++
    rows.set(f.checkId, row)
  }
  return [...rows.values()]
    .sort((a, b) => SEVERITIES.indexOf(a.severity) - SEVERITIES.indexOf(b.severity) || b.open - a.open)
    .slice(0, limit)
}

/** Posture at a glance: score, open findings by severity, per-category multiples and the top failing checks. */
export function OverviewPage() {
  const { t } = useTranslation(NS)
  const texts = useSecurityTexts()
  const query = usePluginQuery(() => securityData().report(), [])
  const report = query.data

  const columns = useMemo<Column<CheckRow>[]>(
    () => [
      { key: 'severity', header: t('columns.severity'), width: '100px', render: (r) => <SeverityBadge severity={r.severity} /> },
      { key: 'check', header: t('columns.check'), render: (r) => texts.title(r.checkId, r.title) },
      { key: 'open', header: t('columns.open'), width: '90px', render: (r) => formatNumber(r.open) },
    ],
    [t, texts],
  )

  const toolbar = (rep: SecurityReport | null) => (
    <Toolbar title={t('overview.title')}>
      {rep && <ExportButtons report={rep} />}
      <ScanControls />
    </Toolbar>
  )

  if (query.loading && !query.data) return <div className="page-loading">{t('overview.loading')}</div>
  if (query.error) {
    return (
      <div className="page">
        {toolbar(null)}
        <Unavailable code={query.errorCode} reason={query.error} title={t('overview.unavailable')} />
      </div>
    )
  }
  if (!report) {
    return (
      <div className="page">
        {toolbar(null)}
        <EmptyState icon="shield" title={t('overview.noReport')} hint={t('overview.noReportHint')} />
      </div>
    )
  }

  const { counts, score } = report
  const top = topChecks(report.findings, 8)
  return (
    <div className="page">
      {toolbar(report)}
      <div className="page__scroll security-page">
        <div className="security-hero">
          <div className="security-hero__score" style={{ color: scoreColor(score.overall) }}>
            <span className="security-hero__value">{score.overall === null ? t('overview.noScore') : formatNumber(score.overall)}</span>
            <span className="security-hero__unit">/100</span>
          </div>
          <div className="security-hero__meta">
            <div className="security-hero__label">{t('overview.score')}</div>
            <div className="security-muted">
              {t('overview.generated', { cluster: report.cluster, time: formatDateTime(report.generatedAt) })}
            </div>
            <div className="security-muted">
              {report.options.includeSystem
                ? t('overview.systemIncluded')
                : t('overview.systemExcluded', { namespaces: report.systemNamespaces.join(', ') })}
            </div>
          </div>
        </div>

        <div className="security-tiles">
          {SEVERITIES.map((s) => (
            <button key={s} type="button" className="security-tile-button" onClick={() => openFindings({ severity: s })}>
              <StatTile
                icon="warning"
                accent={SEVERITY_COLORS[s]}
                label={texts.severity(s)}
                value={formatNumber(counts[s])}
                detail={t('overview.openFindings', { count: counts[s] })}
              />
            </button>
          ))}
          <button type="button" className="security-tile-button" onClick={() => openFindings({ status: 'accepted' })}>
            <StatTile
              icon="pass"
              label={t('overview.accepted')}
              value={formatNumber(counts.accepted)}
              detail={t('overview.acceptedDetail', { total: counts.total })}
            />
          </button>
        </div>

        <section>
          <h2 className="security-section-title">{t('overview.byCategory')}</h2>
          <CategoryMultiples categories={score.byCategory} />
        </section>

        <section>
          <h2 className="security-section-title">{t('overview.topChecks')}</h2>
          {top.length === 0 ? (
            <EmptyState icon="pass" title={t('overview.allPassing')} />
          ) : (
            <Table columns={columns} rows={top} rowKey={(r) => r.checkId} onRowClick={(r) => openFindings({ checkId: r.checkId })} />
          )}
        </section>

        <ReportNotices report={report} />
      </div>
    </div>
  )
}
