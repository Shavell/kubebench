import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePluginQuery } from '@/data/pluginData'
import { formatNumber } from '@/i18n/format'
import { EmptyState } from '@/shared/EmptyState'
import { Checkbox, Select } from '@/shared/form'
import { ListFilterBar } from '@/shared/ListFilterBar'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { accessData } from '../data'
import { RBAC_SCOPE } from '../data/topics'
import type { SubjectKind, SubjectSummary } from '../data/types'
import { NS } from '../i18n'
import { isSystemName, subjectKey } from '../model/format'
import { openSubject } from '../tabs'
import { Mono, SubjectCell } from './parts'
import '@/shared/tabPage.css'
import '../access.css'

type KindFilter = SubjectKind | 'all'

/** Every subject a binding names, plus unbound ServiceAccounts. */
export function SubjectsPage() {
  const { t } = useTranslation(NS)
  const query = usePluginQuery(() => accessData().listSubjects(), [], RBAC_SCOPE)
  const [search, setSearch] = useState('')
  const [kind, setKind] = useState<KindFilter>('all')
  const [hideSystem, setHideSystem] = useState(true)

  const rows = useMemo(() => {
    const needle = search.trim().toLowerCase()
    return (query.data ?? []).filter(
      (s) =>
        (kind === 'all' || s.kind === kind) &&
        !(hideSystem && (isSystemName(s.name) || s.namespace === 'kube-system')) &&
        (!needle || `${s.namespace ?? ''}/${s.name}`.toLowerCase().includes(needle)),
    )
  }, [query.data, search, kind, hideSystem])

  const columns = useMemo<Column<SubjectSummary>[]>(
    () => [
      { key: 'name', header: t('columns.subject'), sortValue: (s) => s.name, render: (s) => <SubjectCell subject={s} missing={s.missing} /> },
      { key: 'kind', header: t('columns.kind'), sortValue: (s) => s.kind, render: (s) => t(`kinds.${s.kind}`) },
      {
        key: 'cluster',
        header: t('columns.clusterBindings'),
        sortValue: (s) => s.clusterBindings,
        render: (s) => formatNumber(s.clusterBindings),
      },
      {
        key: 'namespaces',
        header: t('columns.namespaces'),
        sortValue: (s) => s.namespaces.length,
        render: (s) => <Mono values={s.namespaces} />,
      },
      {
        key: 'roles',
        header: t('columns.roles'),
        sortValue: (s) => s.roles.length,
        render: (s) => <Mono values={s.roles.map((r) => r.name)} />,
      },
    ],
    [t],
  )

  const body = () => {
    if (query.loading && !query.data) return <div className="page-loading">{t('subjects.loading')}</div>
    if (query.error) return <Unavailable code={query.errorCode} reasonCode={query.errorReason} reason={query.error} title={t('subjects.unavailable')} />
    if (!rows.length) return <EmptyState icon="organization" title={t('subjects.empty')} hint={t('subjects.emptyHint')} />
    return (
      <div className="page__scroll">
        <Table columns={columns} rows={rows} rowKey={subjectKey} onRowClick={openSubject} />
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={t('subjects.title')}>
        <ListFilterBar
          search={search}
          onSearch={setSearch}
          filtersActive={kind !== 'all' || !hideSystem}
          summary={query.data ? t('subjects.summary', { shown: rows.length, total: query.data.length }) : undefined}
        >
          <Select
            aria-label={t('columns.kind')}
            value={kind}
            onChange={setKind}
            options={[
              { value: 'all', label: t('subjects.allKinds') },
              { value: 'User', label: t('kinds.User') },
              { value: 'Group', label: t('kinds.Group') },
              { value: 'ServiceAccount', label: t('kinds.ServiceAccount') },
            ]}
          />
          <Checkbox className="access-toolbar-check" checked={hideSystem} onChange={setHideSystem} label={t('subjects.hideSystem')} />
        </ListFilterBar>
      </Toolbar>
      {body()}
    </div>
  )
}
