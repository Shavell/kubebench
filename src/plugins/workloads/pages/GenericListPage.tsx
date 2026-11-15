import { useState } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useContextStore } from '@/core/store/contextStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import type { KubeObject } from '@/data/types'
import { formatNumber } from '@/i18n/format'
import { ListFilterBar } from '@/shared/ListFilterBar'
import { Table } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { EmptyState } from '@/shared/EmptyState'
import { Unavailable } from '@/shared/Unavailable'
import { useWorkloadsTranslation } from '../i18n'
import { catalogByKey, objName, objNamespace, resourceLabel, withHeaders } from '../resourceCatalog'
import { ResourceDialogs } from '../editing/ResourceDialogs'
import { NewResourceButton } from '../editing/ResourceEditButtons'
import { genericMenu } from '../rowActions'
import { WorkloadOpsDialogs } from '../ops/WorkloadOpsDialogs'
import { EventsListPage } from './EventsListPage'
import { initialLocalNamespace, listNamespace } from './namespaceFilter'
import { NamespaceFilterSelect } from './NamespaceFilterSelect'
import '@/shared/tabPage.css'

/** A catalog resource list; Events have their own view (warning filter, grouping). */
export function GenericListPage({ tab }: { tab: TabDescriptor }) {
  return tab.props?.resourceKey === 'events' ? <EventsListPage tab={tab} /> : <CatalogListPage tab={tab} />
}

function CatalogListPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useWorkloadsTranslation()
  const providers = useProviders()
  const key = tab.props?.resourceKey as string
  const descriptor = catalogByKey.get(key)
  const globalNamespace = useContextStore((s) => s.namespace)
  const [search, setSearch] = useState('')
  /** Page-local namespace filter (namespaceFilter.ts); palette `-n` / `-A` preset it. */
  const [localNs, setLocalNs] = useState(() => initialLocalNamespace(tab.props))

  const nsFilter = descriptor?.namespaced ? listNamespace(localNs, globalNamespace) : undefined

  const { data: namespaces } = useProviderQuery((p) => p.cluster.listNamespaces(), [], { topics: ['namespaces'] })
  const query = useProviderQuery(
    (p) => (descriptor ? p.cluster.listResource(key, nsFilter) : Promise.resolve([])),
    [key, nsFilter],
    // Catalog keys are resource names, i.e. watch topics.
    { topics: [key], namespace: nsFilter },
  )

  if (!descriptor) return <div className="page-loading">{t('list.unknownResource', { key })}</div>
  const label = resourceLabel(key)
  if (query.loading && !query.data) {
    return <div className="page-loading">{t('list.loading', { resource: label })}</div>
  }

  const all = query.data ?? []
  const rows = search
    ? all.filter((o) => objName(o).toLowerCase().includes(search.toLowerCase()))
    : all

  const openDetail = (o: KubeObject) =>
    useLayoutStore.getState().openTab({
      id: `res:${key}:${objNamespace(o) || '-'}/${objName(o)}`,
      title: objName(o),
      icon: descriptor.icon,
      kind: 'generic-detail',
      props: { resourceKey: key, namespace: objNamespace(o) || null, name: objName(o) },
    })

  const scopedTitle = descriptor.namespaced && nsFilter ? `${label} · ${nsFilter}` : label

  const body = () => {
    // A failed request must not pass for an empty list (live mode never shows stand-in data).
    if (query.error) {
      return (
        <Unavailable
          code={query.errorCode}
          reason={query.error}
          title={t('list.loadFailed', { resource: label })}
        />
      )
    }
    if (rows.length === 0) {
      return (
        <EmptyState
          icon="inbox"
          title={t('list.empty', { resource: label })}
          hint={search || nsFilter ? t('list.clearFiltersHint') : undefined}
        />
      )
    }
    return (
      <div className="page__scroll">
        <Table
          columns={withHeaders(descriptor.columns)}
          rows={rows}
          rowKey={(o) => `${objNamespace(o)}/${objName(o)}`}
          onRowClick={openDetail}
          onRowContextMenu={(o, e) =>
            genericMenu(
              e,
              { kind: descriptor.kind, apiVersion: o.apiVersion, namespace: objNamespace(o) || undefined, name: objName(o) },
              providers,
              o,
            )
          }
        />
      </div>
    )
  }

  return (
    <div className="page">
      <Toolbar title={scopedTitle}>
        <ListFilterBar
          search={search}
          onSearch={setSearch}
          filtersActive={Boolean(localNs)}
          summary={t('list.summary', { shown: formatNumber(rows.length), total: formatNumber(all.length) })}
        >
          {descriptor.namespaced && (
            <NamespaceFilterSelect
              value={localNs}
              onChange={setLocalNs}
              globalNamespace={globalNamespace}
              namespaces={(namespaces ?? []).map((ns) => ns.name)}
            />
          )}
        </ListFilterBar>
        <NewResourceButton kind={descriptor.kind} namespace={nsFilter} />
      </Toolbar>
      {body()}
      <ResourceDialogs />
      <WorkloadOpsDialogs />
    </div>
  )
}
