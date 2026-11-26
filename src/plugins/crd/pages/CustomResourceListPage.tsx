import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { openYamlTab } from '@/core/plugin/editorTabs'
import { resourceActionItems } from '@/core/plugin/resourceActions'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { CORE_NS } from '@/i18n'
import { Badge } from '@/shared/Badge'
import { Checkbox } from '@/shared/form'
import { showContextMenu } from '@/shared/contextMenu'
import { copyMenuItems } from '@/shared/copyActions'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { EmptyState } from '@/shared/EmptyState'
import { Unavailable } from '@/shared/Unavailable'
import type { CustomResource } from '@/data/types'
import { NS } from '../i18n'
import { printerColumns, printerSortValue } from '../printerColumns'
import '@/shared/tabPage.css'
import '../crd.css'

export function CustomResourceListPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const crdName = tab.props?.crdName as string
  const providers = useProviders()
  // Like `kubectl get -o wide`: printer columns of priority > 0 on request.
  const [wide, setWide] = useState(false)

  const { data: crds } = useProviderQuery((p) => p.crd.listCrds(), [], { topics: ['customresourcedefinitions.apiextensions.k8s.io'] })
  const { data, loading, error, errorCode, errorReason } = useProviderQuery(
    (p) => p.crd.listCustomResources(crdName),
    [crdName],
    // A CRD's name (<plural>.<group>) is the watch topic of its resources.
    { topics: [crdName] },
  )

  const crd = crds?.find((c) => c.name === crdName)
  if (loading) return <div className="page-loading">{t('instances.loading', { crd: crdName })}</div>

  const rows = data ?? []
  // The CRD's printer columns; their headers are the CRD's own column names.
  const { shown: shownColumns, hasWide } = printerColumns(crd, rows, wide)

  const columns: Column<CustomResource>[] = [
    { key: 'name', header: t('common.name', { ns: CORE_NS }), render: (r) => r.name, sortValue: (r) => r.name },
    {
      key: 'ns',
      header: t('common.namespace', { ns: CORE_NS }),
      render: (r) => r.namespace ?? '—',
      sortValue: (r) => r.namespace ?? '',
    },
    { key: 'status', header: t('common.status', { ns: CORE_NS }), render: (r) => <Badge>{r.status}</Badge> },
    ...shownColumns.map((col): Column<CustomResource> => {
      const sortKey = printerSortValue(col)
      return {
        key: `extra:${col.name}`,
        header: col.name,
        render: (r) => r.extra[col.name] ?? '—',
        sortValue: sortKey && ((r) => sortKey(r.extra[col.name])),
      }
    }),
    { key: 'age', header: t('common.age', { ns: CORE_NS }), render: (r) => r.age },
  ]

  // The editor's YAML tab (view, edit, apply), once the CRD's group and version are known.
  const viewYaml = (r: CustomResource) => {
    if (!crd) return
    openYamlTab({ apiVersion: `${crd.group}/${crd.versions[0]}`, kind: crd.kind, namespace: r.namespace, name: r.name })
  }

  // YAML, copy name/YAML and the actions other plugins contribute for this kind (design §6.4).
  const rowMenu = (r: CustomResource, e: React.MouseEvent) => {
    const ref = {
      kind: crd?.kind ?? r.crdName,
      apiVersion: crd ? `${crd.group}/${crd.versions[0]}` : undefined,
      namespace: r.namespace,
      name: r.name,
    }
    showContextMenu(e, [
      { label: t('instances.viewYaml'), icon: 'file-code', onClick: () => viewYaml(r) },
      ...(ref.apiVersion ? copyMenuItems({ ...ref, apiVersion: ref.apiVersion }, providers.cluster) : []),
      ...resourceActionItems(ref),
    ])
  }

  const content = error ? (
    <Unavailable code={errorCode} reasonCode={errorReason} reason={error} title={t('instances.unavailable')} />
  ) : rows.length === 0 ? (
    <EmptyState icon="inbox" title={t('instances.empty')} />
  ) : (
    <div className="page__scroll">
      <Table
        columns={columns}
        rows={rows}
        rowKey={(r) => `${r.namespace ?? '-'}/${r.name}`}
        onRowClick={viewYaml}
        onRowContextMenu={rowMenu}
      />
    </div>
  )

  return (
    <div className="page">
      <Toolbar title={`${crd?.kind ?? crdName} · ${crdName}`}>
        {hasWide && <Checkbox className="crd-toolbar-check" checked={wide} onChange={setWide} label={t('instances.allColumns')} />}
      </Toolbar>
      {content}
    </div>
  )
}

