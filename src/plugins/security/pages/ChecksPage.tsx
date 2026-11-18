import { useMemo, useState } from 'react'
import { reasonText } from '@/core/reasons'
import { usePluginQuery } from '@/data/pluginData'
import { formatNumber } from '@/i18n/format'
import { EmptyState } from '@/shared/EmptyState'
import { Select } from '@/shared/form'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { securityData } from '../data'
import { CATEGORIES, SEVERITIES, type Category, type SecurityCheck } from '../data/types'
import { openFindings } from '../tabs'
import { SeverityBadge } from './parts'
import { useSecurityTexts } from './texts'
import '@/shared/tabPage.css'
import '../security.css'

/** The check catalog with each check's open findings in the last report; a row opens its findings. */
export function ChecksPage() {
  const texts = useSecurityTexts()
  const { t } = texts
  const checks = usePluginQuery(() => securityData().checks(), [])
  const report = usePluginQuery(() => securityData().report(), [])
  const [category, setCategory] = useState<Category | 'all'>('all')

  const open = useMemo(() => {
    const counts = new Map<string, number>()
    for (const f of report.data?.findings ?? []) if (!f.accepted) counts.set(f.checkId, (counts.get(f.checkId) ?? 0) + 1)
    return counts
  }, [report.data])
  const skipped = useMemo(() => new Map((report.data?.skipped ?? []).map((s) => [s.checkId, s])), [report.data])

  const columns = useMemo<Column<SecurityCheck>[]>(
    () => [
      {
        key: 'severity',
        header: t('columns.severity'),
        width: '96px',
        sortValue: (c) => SEVERITIES.indexOf(c.severity),
        render: (c) => <SeverityBadge severity={c.severity} />,
      },
      {
        key: 'check',
        header: t('columns.check'),
        sortValue: (c) => texts.title(c.id, c.title),
        render: (c) => (
          <>
            <div>{texts.title(c.id, c.title)}</div>
            <div className="security-muted security-small">{texts.remediation(c.id, c.remediation)}</div>
          </>
        ),
      },
      { key: 'category', header: t('columns.category'), sortValue: (c) => c.category, render: (c) => texts.category(c.category) },
      {
        key: 'frameworks',
        header: t('columns.frameworks'),
        render: (c) => (
          <div className="security-chips">
            {c.frameworks.map((fw) => (
              <span key={fw} className="security-chip">
                {fw}
              </span>
            ))}
          </div>
        ),
      },
      { key: 'target', header: t('columns.target'), render: (c) => t(`targets.${c.target}`) },
      {
        key: 'open',
        header: t('columns.open'),
        width: '110px',
        sortValue: (c) => open.get(c.id) ?? 0,
        render: (c) => {
          const skip = skipped.get(c.id)
          if (skip) {
            const reason = reasonText(skip.reasonCode, skip.reasonParams, skip.reason)
            return <span className="security-muted" title={reason}>{t('catalog.skipped')}</span>
          }
          if (!report.data) return <span className="security-muted">—</span>
          const n = open.get(c.id) ?? 0
          return n ? formatNumber(n) : <span className="security-pass">{t('catalog.passing')}</span>
        },
      },
    ],
    [t, texts, open, skipped, report.data],
  )

  const rows = (checks.data ?? []).filter((c) => category === 'all' || c.category === category)
  return (
    <div className="page">
      <Toolbar title={t('catalog.title')}>
        <Select
          aria-label={t('filters.category')}
          value={category}
          onChange={setCategory}
          options={[{ value: 'all', label: t('filters.allCategories') }, ...CATEGORIES.map((c) => ({ value: c, label: texts.category(c) }))]}
        />
      </Toolbar>
      {checks.loading && !checks.data ? (
        <div className="page-loading">{t('catalog.loading')}</div>
      ) : checks.error ? (
        <Unavailable code={checks.errorCode} reason={checks.error} title={t('catalog.unavailable')} />
      ) : rows.length === 0 ? (
        <EmptyState icon="checklist" title={t('catalog.empty')} />
      ) : (
        <div className="page__scroll">
          <Table columns={columns} rows={rows} rowKey={(c) => c.id} onRowClick={(c) => openFindings({ checkId: c.id, status: 'all' })} />
        </div>
      )}
    </div>
  )
}
