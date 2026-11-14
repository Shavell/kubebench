import { useTranslation } from 'react-i18next'
import { resourceActionItems } from '@/core/plugin/resourceActions'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { CORE_NS } from '@/i18n'
import { Badge } from '@/shared/Badge'
import { showContextMenu } from '@/shared/contextMenu'
import { notify } from '@/shared/notify'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { EmptyState } from '@/shared/EmptyState'
import { Unavailable } from '@/shared/Unavailable'
import type { CustomResource } from '@/data/types'
import { NS } from '../i18n'
import '@/shared/tabPage.css'

export function CustomResourceListPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const crdName = tab.props?.crdName as string
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)

  const { data: crds } = useProviderQuery((p) => p.crd.listCrds(), [])
  const { data, loading, error, errorCode } = useProviderQuery(
    (p) => p.crd.listCustomResources(crdName),
    [crdName],
  )

  const crd = crds?.find((c) => c.name === crdName)
  if (loading) return <div className="page-loading">{t('instances.loading', { crd: crdName })}</div>

  const rows = data ?? []
  // CRD-specific printer columns: their headers are the CRD's own column names.
  const extraKeys = [...new Set(rows.flatMap((r) => Object.keys(r.extra)))]

  const columns: Column<CustomResource>[] = [
    { key: 'name', header: t('common.name', { ns: CORE_NS }), render: (r) => r.name, sortValue: (r) => r.name },
    {
      key: 'ns',
      header: t('common.namespace', { ns: CORE_NS }),
      render: (r) => r.namespace ?? '—',
      sortValue: (r) => r.namespace ?? '',
    },
    { key: 'status', header: t('common.status', { ns: CORE_NS }), render: (r) => <Badge>{r.status}</Badge> },
    ...extraKeys.map(
      (key): Column<CustomResource> => ({
        key,
        header: key,
        render: (r) => r.extra[key] ?? '—',
      }),
    ),
    { key: 'age', header: t('common.age', { ns: CORE_NS }), render: (r) => r.age },
  ]

  const viewYaml = async (r: CustomResource) => {
    let text: string
    try {
      text = await providers.cluster.getResourceYaml(crd?.kind ?? 'CustomResource', r.namespace ?? null, r.name)
    } catch (err) {
      notify.error(err)
      return
    }
    openTab({
      id: `yaml:cr:${crdName}:${r.namespace ?? '-'}/${r.name}`,
      title: `${r.name}.yaml`,
      icon: 'file-code',
      kind: 'yaml',
      props: { text, readOnly: true },
    })
  }

  // YAML plus the actions other plugins contribute for this kind (design §6.4).
  const rowMenu = (r: CustomResource, e: React.MouseEvent) => {
    const ref = {
      kind: crd?.kind ?? r.crdName,
      apiVersion: crd ? `${crd.group}/${crd.versions[0]}` : undefined,
      namespace: r.namespace,
      name: r.name,
    }
    showContextMenu(e, [
      { label: t('instances.viewYaml'), icon: 'file-code', onClick: () => viewYaml(r) },
      ...resourceActionItems(ref),
    ])
  }

  const content = error ? (
    <Unavailable code={errorCode} reason={error} title={t('instances.unavailable')} />
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
      <Toolbar title={`${crd?.kind ?? crdName} · ${crdName}`} />
      {content}
    </div>
  )
}
