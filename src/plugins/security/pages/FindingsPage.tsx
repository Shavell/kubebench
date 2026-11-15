import { useMemo, useState } from 'react'
import { usePluginQuery } from '@/data/pluginData'
import { formatNumber } from '@/i18n/format'
import { Button } from '@/shared/Button'
import { EmptyState } from '@/shared/EmptyState'
import { Select } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { ListFilterBar } from '@/shared/ListFilterBar'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { securityData } from '../data'
import { CATEGORIES, SEVERITIES, type Finding } from '../data/types'
import { filterFindings, filterOptions, groupFindings, GROUPINGS, narrowed, type Grouping } from '../model/filters'
import { useSecurityStore } from '../store'
import { AcceptDialog, UnacceptDialog, type RiskTarget } from './AcceptDialogs'
import { FindingDetail } from './FindingDetail'
import { ResourceLink, ScanControls, SeverityBadge, WriteButton } from './parts'
import { resourceLabel, useSecurityTexts } from './texts'
import '@/shared/tabPage.css'
import '../security.css'

/** Findings of the last report: filters, grouping, accept / un-accept and the selected finding's detail. */
export function FindingsPage() {
  const texts = useSecurityTexts()
  const { t } = texts
  const query = usePluginQuery(() => securityData().report(), [])
  const filter = useSecurityStore((s) => s.filter)
  const setFilter = useSecurityStore((s) => s.setFilter)
  const resetFilter = useSecurityStore((s) => s.resetFilter)
  const grouping = useSecurityStore((s) => s.grouping)
  const setGrouping = useSecurityStore((s) => s.setGrouping)
  const [selectedKey, setSelectedKey] = useState<string | null>(null)
  const [collapsed, setCollapsed] = useState<ReadonlySet<string>>(new Set())
  const [accepting, setAccepting] = useState<RiskTarget | null>(null)
  const [unaccepting, setUnaccepting] = useState<RiskTarget | null>(null)

  const report = query.data
  const findings = useMemo(() => report?.findings ?? [], [report])
  const options = useMemo(() => filterOptions(findings), [findings])
  const shown = useMemo(
    () => filterFindings(findings, filter, (f) => `${texts.title(f.checkId, f.title)} ${texts.message(f)}`),
    [findings, filter, texts],
  )
  const groups = useMemo(() => groupFindings(shown, grouping), [shown, grouping])
  const selected = findings.find((f) => f.key === selectedKey) ?? null

  const columns = useMemo<Column<Finding>[]>(
    () => [
      {
        key: 'severity',
        header: t('columns.severity'),
        width: '96px',
        sortValue: (f) => SEVERITIES.indexOf(f.severity),
        render: (f) => <SeverityBadge severity={f.severity} />,
      },
      {
        key: 'check',
        header: t('columns.check'),
        sortValue: (f) => texts.title(f.checkId, f.title),
        render: (f) => texts.title(f.checkId, f.title),
      },
      {
        key: 'resource',
        header: t('columns.resource'),
        sortValue: (f) => resourceLabel(f.resource),
        render: (f) => <ResourceLink resource={f.resource} />,
      },
      {
        key: 'message',
        header: t('columns.message'),
        render: (f) => <span className="security-message">{texts.message(f)}</span>,
      },
      {
        key: 'status',
        header: t('columns.status'),
        width: '120px',
        render: (f) =>
          f.accepted ? (
            <span className="security-accepted" title={f.accepted.reason}>
              <Icon name="pass" size={13} /> {t('status.accepted')}
            </span>
          ) : (
            <span className="security-muted">{t('status.open')}</span>
          ),
      },
      {
        key: 'actions',
        header: '',
        width: '120px',
        render: (f) =>
          f.accepted ? (
            <WriteButton label={t('actions.unaccept')} onClick={() => setUnaccepting(riskTarget(f, texts))} />
          ) : (
            <WriteButton label={t('actions.accept')} onClick={() => setAccepting(riskTarget(f, texts))} />
          ),
      },
    ],
    [t, texts],
  )

  const filters = (
    <ListFilterBar
      search={filter.search}
      onSearch={(search) => setFilter({ search })}
      filtersActive={narrowed(filter)}
      summary={report ? t('findings.summary', { shown: formatNumber(shown.length), total: formatNumber(findings.length) }) : undefined}
    >
      <Select
        aria-label={t('filters.severity')}
        value={filter.severity}
        onChange={(severity) => setFilter({ severity })}
        options={[{ value: 'all', label: t('filters.allSeverities') }, ...SEVERITIES.map((s) => ({ value: s, label: texts.severity(s) }))]}
      />
      <Select
        aria-label={t('filters.category')}
        value={filter.category}
        onChange={(category) => setFilter({ category })}
        options={[{ value: 'all', label: t('filters.allCategories') }, ...CATEGORIES.map((c) => ({ value: c, label: texts.category(c) }))]}
      />
      <Select
        aria-label={t('filters.framework')}
        value={filter.framework}
        onChange={(framework) => setFilter({ framework })}
        options={[{ value: 'all', label: t('filters.allFrameworks') }, ...options.frameworks.map((f) => ({ value: f, label: f }))]}
      />
      <Select
        aria-label={t('filters.namespace')}
        value={filter.namespace}
        onChange={(namespace) => setFilter({ namespace })}
        options={[{ value: 'all', label: t('filters.allNamespaces') }, ...options.namespaces.map((n) => ({ value: n, label: n }))]}
      />
    </ListFilterBar>
  )

  const body = () => {
    if (query.loading && !query.data) return <div className="page-loading">{t('findings.loading')}</div>
    if (query.error) return <Unavailable code={query.errorCode} reason={query.error} title={t('findings.unavailable')} />
    if (!report) return <EmptyState icon="shield" title={t('overview.noReport')} hint={t('overview.noReportHint')} />
    return (
      <div className="security-split">
        <div className="page__scroll security-findings">
          <div className="security-controls">
            <label className="security-control">
              <span>{t('filters.status')}</span>
              <Select
                value={filter.status}
                onChange={(status) => setFilter({ status })}
                options={(['open', 'accepted', 'all'] as const).map((s) => ({ value: s, label: t(`filters.statuses.${s}`) }))}
              />
            </label>
            <label className="security-control">
              <span>{t('filters.groupBy')}</span>
              <Select<Grouping>
                value={grouping}
                onChange={setGrouping}
                options={GROUPINGS.map((g) => ({ value: g, label: t(`groupings.${g}`) }))}
              />
            </label>
            {filter.checkId && (
              <span className="security-chip security-chip--filter">
                {t('filters.check', { title: texts.title(filter.checkId, filter.checkId) })}
                <button type="button" title={t('filters.clear')} onClick={() => setFilter({ checkId: null })}>
                  <Icon name="close" size={11} />
                </button>
              </span>
            )}
            {filter.resource && (
              <span className="security-chip security-chip--filter">
                {t('filters.resource', { resource: resourceLabel({ apiVersion: '', ...filter.resource }) })}
                <button type="button" title={t('filters.clear')} onClick={() => setFilter({ resource: null })}>
                  <Icon name="close" size={11} />
                </button>
              </span>
            )}
            {(narrowed(filter) || filter.search || filter.status !== 'open') && (
              <Button variant="secondary" onClick={() => resetFilter()}>
                {t('filters.reset')}
              </Button>
            )}
          </div>
          {shown.length === 0 ? (
            <EmptyState
              icon="pass"
              title={findings.length === 0 ? t('findings.none') : t('findings.noMatch')}
              hint={findings.length === 0 ? undefined : t('findings.noMatchHint')}
            />
          ) : (
            groups.map((g) => (
              <section key={g.key || 'all'} className="security-group">
                {grouping !== 'none' && (
                  <button
                    type="button"
                    className="security-group__head"
                    aria-expanded={!collapsed.has(g.key)}
                    onClick={() =>
                      setCollapsed((s) => {
                        const next = new Set(s)
                        if (!next.delete(g.key)) next.add(g.key)
                        return next
                      })
                    }
                  >
                    <Icon name={collapsed.has(g.key) ? 'chevron-right' : 'chevron-down'} size={14} />
                    <span>{groupLabel(grouping, g.key, g.findings[0], texts)}</span>
                    <span className="security-group__count">{g.findings.length}</span>
                  </button>
                )}
                {!collapsed.has(g.key) && (
                  <Table
                    columns={columns}
                    rows={g.findings}
                    rowKey={(f) => f.key}
                    onRowClick={(f) => setSelectedKey(f.key)}
                  />
                )}
              </section>
            ))
          )}
        </div>
        {selected && (
          <FindingDetail
            finding={selected}
            onAccept={() => setAccepting(riskTarget(selected, texts))}
            onUnaccept={() => setUnaccepting(riskTarget(selected, texts))}
            onClose={() => setSelectedKey(null)}
          />
        )}
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={t('findings.title')}>
        {filters}
        <ScanControls primary={false} compact />
      </Toolbar>
      {body()}
      {accepting && <AcceptDialog target={accepting} onClose={() => setAccepting(null)} />}
      {unaccepting && <UnacceptDialog target={unaccepting} onClose={() => setUnaccepting(null)} />}
    </div>
  )
}

type Texts = ReturnType<typeof useSecurityTexts>

function riskTarget(f: Finding, texts: Texts): RiskTarget {
  return { key: f.key, title: texts.title(f.checkId, f.title), resource: f.resource }
}

function groupLabel(by: Grouping, key: string, first: Finding, texts: Texts): string {
  switch (by) {
    case 'check':
      return texts.title(first.checkId, first.title)
    case 'severity':
      return texts.severity(key)
    case 'category':
      return texts.category(key)
    case 'namespace':
      return key || texts.t('groupings.clusterScoped')
    case 'resource':
      return resourceLabel(first.resource)
    default:
      return ''
  }
}
