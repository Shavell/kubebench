import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePluginQuery } from '@/data/pluginData'
import { formatNumber } from '@/i18n/format'
import { StatTile } from '@/shared/charts/StatTile'
import { EmptyState } from '@/shared/EmptyState'
import { Checkbox, Select } from '@/shared/form'
import { ListFilterBar } from '@/shared/ListFilterBar'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { accessData } from '../data'
import type { RiskyRole, Severity } from '../data/types'
import { NS } from '../i18n'
import { isSystemName, refText, ruleSummary } from '../model/format'
import { openResource } from '../tabs'
import { RefLink, SeverityBadge } from './parts'
import '@/shared/tabPage.css'
import '../access.css'

const SEVERITIES: Severity[] = ['critical', 'high', 'medium']
const ACCENTS: Record<Severity, string> = {
  critical: 'var(--vscode-errorForeground)',
  high: 'var(--vscode-editorWarning-foreground)',
  medium: 'var(--vscode-editorInfo-foreground)',
}
const roleKey = (r: RiskyRole) => `${r.role.kind}/${r.role.namespace ?? ''}/${r.role.name}`

/** Roles that grant dangerous permissions, most severe first. */
export function RiskyRolesPage() {
  const { t } = useTranslation(NS)
  const query = usePluginQuery(() => accessData().listRiskyRoles(), [])
  const [search, setSearch] = useState('')
  const [severity, setSeverity] = useState<Severity | 'all'>('all')
  const [hideSystem, setHideSystem] = useState(true)
  const [boundOnly, setBoundOnly] = useState(false)

  const visible = useMemo(
    () => (query.data ?? []).filter((r) => !(hideSystem && isSystemName(r.role.name)) && !(boundOnly && r.bindings.length === 0)),
    [query.data, hideSystem, boundOnly],
  )
  const rows = useMemo(() => {
    const needle = search.trim().toLowerCase()
    return visible.filter(
      (r) => (severity === 'all' || r.severity === severity) && (!needle || refText(r.role).toLowerCase().includes(needle)),
    )
  }, [visible, search, severity])

  const columns = useMemo<Column<RiskyRole>[]>(
    () => [
      {
        key: 'severity',
        header: t('columns.severity'),
        sortValue: (r) => SEVERITIES.length - SEVERITIES.indexOf(r.severity),
        render: (r) => <SeverityBadge severity={r.severity} />,
      },
      {
        key: 'role',
        header: t('columns.role'),
        sortValue: (r) => r.role.name,
        render: (r) => (
          <>
            <RefLink refValue={r.role} />
            {r.aggregated && <span className="access-muted"> · {t('risky.aggregated')}</span>}
          </>
        ),
      },
      {
        key: 'risks',
        header: t('columns.risks'),
        render: (r) => (
          <ul className="access-risks">
            {r.risks.map((risk) => (
              <li
                key={risk.id}
                className={`access-risk access-risk--${risk.severity}`}
                title={`${t(`severity.${risk.severity}`)} · ${t(`risks.${risk.id}.description`)}\n${ruleSummary(risk.rule)}`}
              >
                {t(`risks.${risk.id}.title`)}
              </li>
            ))}
          </ul>
        ),
      },
      { key: 'bindings', header: t('columns.bindings'), sortValue: (r) => r.bindings.length, render: (r) => formatNumber(r.bindings.length) },
      { key: 'subjects', header: t('columns.subjects'), sortValue: (r) => r.subjects, render: (r) => formatNumber(r.subjects) },
    ],
    [t],
  )

  const count = (s: Severity) => visible.filter((r) => r.severity === s).length

  const body = () => {
    if (query.loading && !query.data) return <div className="page-loading">{t('risky.loading')}</div>
    if (query.error) return <Unavailable code={query.errorCode} reason={query.error} title={t('risky.unavailable')} />
    return (
      <div className="page__scroll access-page">
        <div className="access-tiles">
          {SEVERITIES.map((s) => (
            <StatTile key={s} icon="warning" accent={ACCENTS[s]} label={t(`severity.${s}`)} value={formatNumber(count(s))} detail={t('risky.rolesDetail')} />
          ))}
        </div>
        <p className="access-muted">{t('risky.intro')}</p>
        {rows.length ? (
          <Table columns={columns} rows={rows} rowKey={roleKey} onRowClick={(r) => openResource(r.role.kind, r.role.namespace, r.role.name)} />
        ) : (
          <EmptyState icon="pass" title={t('risky.empty')} hint={t('risky.emptyHint')} />
        )}
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={t('risky.title')}>
        <ListFilterBar
          search={search}
          onSearch={setSearch}
          filtersActive={severity !== 'all' || !hideSystem || boundOnly}
          summary={query.data ? t('risky.summary', { shown: rows.length, total: query.data.length }) : undefined}
        >
          <Select
            aria-label={t('columns.severity')}
            value={severity}
            onChange={setSeverity}
            options={[{ value: 'all', label: t('risky.allSeverities') }, ...SEVERITIES.map((s) => ({ value: s, label: t(`severity.${s}`) }))]}
          />
          <Checkbox className="access-toolbar-check" checked={hideSystem} onChange={setHideSystem} label={t('risky.hideSystem')} />
          <Checkbox className="access-toolbar-check" checked={boundOnly} onChange={setBoundOnly} label={t('risky.boundOnly')} />
        </ListFilterBar>
      </Toolbar>
      {body()}
    </div>
  )
}
