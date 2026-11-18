import { useMemo, useState } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useContextStore } from '@/core/store/contextStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { formatNumber } from '@/i18n/format'
import { EmptyState } from '@/shared/EmptyState'
import { byLastSeen, isWarning, toEventRow, type EventRow } from '@/shared/eventModel'
import { Checkbox } from '@/shared/form'
import { ListFilterBar } from '@/shared/ListFilterBar'
import { Table } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { useWorkloadsTranslation } from '../i18n'
import { catalogByKey, resourceLabel, withHeaders } from '../resourceCatalog'
import { ResourceDialogs } from '../editing/ResourceDialogs'
import { genericMenu } from '../rowActions'
import { WorkloadOpsDialogs } from '../ops/WorkloadOpsDialogs'
import { EventGroups } from './events/EventGroups'
import { eventColumns } from './events/eventColumns'
import { initialLocalNamespace, listNamespace } from './namespaceFilter'
import { NamespaceFilterSelect } from './NamespaceFilterSelect'
import '@/shared/tabPage.css'
import './events/events.css'

const KEY = 'events'

function matches(row: EventRow, search: string): boolean {
  const needle = search.toLowerCase()
  return [row.reason, row.message, row.object, row.name].some((text) => text.toLowerCase().includes(needle))
}

/**
 * The cluster's Events: newest first, with a Warning-only filter and a
 * view grouped by reason. `tab.props.warningsOnly` starts with the filter
 * on (the overview's "recent warnings" card opens it that way).
 */
export function EventsListPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useWorkloadsTranslation()
  const providers = useProviders()
  const descriptor = catalogByKey.get(KEY)!
  const globalNamespace = useContextStore((s) => s.namespace)
  const [search, setSearch] = useState('')
  /** Page-local namespace filter (namespaceFilter.ts); palette `-n` / `-A` preset it. */
  const [localNs, setLocalNs] = useState(() => initialLocalNamespace(tab.props))
  const [warningsOnly, setWarningsOnly] = useState(tab.props?.warningsOnly === true)
  const [grouped, setGrouped] = useState(false)

  const nsFilter = listNamespace(localNs, globalNamespace)
  const { data: namespaces } = useProviderQuery((p) => p.cluster.listNamespaces(), [], { topics: ['namespaces'] })
  // Ages are relative to when the list was read.
  const query = useProviderQuery(
    async (p) => ({ items: await p.cluster.listResource(KEY, nsFilter), readAt: Date.now() }),
    [nsFilter],
    { topics: [KEY], namespace: nsFilter },
  )

  const all = useMemo(() => {
    const read = query.data
    return read ? read.items.map((o) => toEventRow(o, read.readAt)).sort(byLastSeen) : []
  }, [query.data])

  const label = resourceLabel(KEY)
  if (query.loading && !query.data) {
    return <div className="page-loading">{t('list.loading', { resource: label })}</div>
  }

  const rows = all.filter((row) => (!warningsOnly || isWarning(row)) && (!search || matches(row, search)))
  const warnings = all.filter(isWarning).length

  const openDetail = (row: EventRow) =>
    useLayoutStore.getState().openTab({
      id: `res:${KEY}:${row.namespace || '-'}/${row.name}`,
      title: row.name,
      icon: descriptor.icon,
      kind: 'generic-detail',
      props: { resourceKey: KEY, namespace: row.namespace || null, name: row.name },
    })
  const rowMenu = (row: EventRow, e: React.MouseEvent) =>
    genericMenu(
      e,
      { kind: 'Event', apiVersion: row.source.apiVersion ?? 'v1', namespace: row.namespace || undefined, name: row.name },
      providers,
      row.source,
    )

  const body = () => {
    // A failed request must not pass for an empty list (live mode never shows stand-in data).
    if (query.error) {
      return <Unavailable code={query.errorCode} reason={query.error} title={t('list.loadFailed', { resource: label })} />
    }
    if (rows.length === 0) {
      return (
        <EmptyState
          icon={warningsOnly ? 'pass' : 'inbox'}
          title={warningsOnly ? t('eventsView.noWarnings') : t('list.empty', { resource: label })}
          hint={search || nsFilter || warningsOnly ? t('list.clearFiltersHint') : undefined}
        />
      )
    }
    return (
      <div className="page__scroll">
        {grouped ? (
          <EventGroups rows={rows} onOpen={openDetail} onContextMenu={rowMenu} />
        ) : (
          <Table
            columns={withHeaders(eventColumns)}
            rows={rows}
            rowKey={(row) => row.key}
            onRowClick={openDetail}
            onRowContextMenu={rowMenu}
          />
        )}
      </div>
    )
  }

  const scopedTitle = nsFilter ? `${label} · ${nsFilter}` : label

  return (
    <div className="page">
      <Toolbar title={scopedTitle}>
        <ListFilterBar
          search={search}
          onSearch={setSearch}
          filtersActive={Boolean(localNs)}
          summary={t('list.summary', { shown: formatNumber(rows.length), total: formatNumber(all.length) })}
        >
          <NamespaceFilterSelect
            value={localNs}
            onChange={setLocalNs}
            globalNamespace={globalNamespace}
            namespaces={(namespaces ?? []).map((ns) => ns.name)}
          />
        </ListFilterBar>
        <Checkbox
          className="events-toolbar__toggle"
          checked={warningsOnly}
          onChange={setWarningsOnly}
          label={t('eventsView.warningsOnly', { count: warnings, value: formatNumber(warnings) })}
        />
        <Checkbox
          className="events-toolbar__toggle"
          checked={grouped}
          onChange={setGrouped}
          label={t('eventsView.groupByReason')}
        />
      </Toolbar>
      {body()}
      <ResourceDialogs />
      <WorkloadOpsDialogs />
    </div>
  )
}
