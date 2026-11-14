import type { TableColumn } from '@kubebench/sdk'
import { PLUGIN_ID } from './plugin'

/** The parts of a Kubernetes NamespaceList this view shows. */
interface Namespace {
  metadata: { name: string; creationTimestamp?: string; labels?: Record<string, string> }
  status?: { phase?: string }
}
interface NamespaceList {
  items: Namespace[]
}

/** Creation time in the UI language (KubeBench keeps <html lang> in sync with it). */
function created(ns: Namespace): string {
  const timestamp = ns.metadata.creationTimestamp
  return timestamp ? new Date(timestamp).toLocaleString(document.documentElement.lang) : '—'
}

/** Lists the namespaces of the active cluster through the read-only Kubernetes API. */
export function NamespacesTab() {
  const { api, data, i18n, ui } = window.KubeBench
  const t = i18n.useTranslation(PLUGIN_ID)
  const namespaces = data.usePluginQuery(() => api.k8sGet<NamespaceList>('/api/v1/namespaces'), [])

  if (namespaces.error) {
    return <ui.EmptyState icon="error" title={t('namespaces.failed')} hint={namespaces.error} />
  }
  if (!namespaces.data) return <ui.EmptyState icon="loading" title={t('namespaces.loading')} />

  const rows = namespaces.data.items
  const columns: TableColumn<Namespace>[] = [
    { key: 'name', header: t('columns.name'), render: (ns) => ns.metadata.name, sortValue: (ns) => ns.metadata.name },
    { key: 'status', header: t('columns.status'), render: (ns) => <ui.Badge>{ns.status?.phase ?? 'Unknown'}</ui.Badge> },
    { key: 'labels', header: t('columns.labels'), render: (ns) => Object.keys(ns.metadata.labels ?? {}).length },
    { key: 'created', header: t('columns.created'), render: created },
  ]

  return (
    <div style={{ height: '100%', display: 'flex', flexDirection: 'column' }}>
      <ui.Toolbar title={t('namespaces.title', { count: rows.length })}>
        <button type="button" className="kb-button kb-button--secondary" onClick={data.refreshData}>
          {t('namespaces.refresh')}
        </button>
      </ui.Toolbar>
      <div style={{ flex: 1, minHeight: 0, overflow: 'auto' }}>
        <ui.Table columns={columns} rows={rows} rowKey={(ns) => ns.metadata.name} />
      </div>
    </div>
  )
}
