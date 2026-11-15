import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Can, usePermission } from '@/core/permissions'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import { useDataMode } from '@/data/mode'
import type { ClusterEntry } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { ProviderBadge } from '@/shared/cloud/ProviderBadge'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { EmptyState } from '@/shared/EmptyState'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Page } from '@/shared/Page'
import { Table, type Column } from '@/shared/Table'
import { Unavailable } from '@/shared/Unavailable'
import { AddClusterDialog } from './AddClusterDialog'
import { EnvironmentBadge, ReadOnlyBadge, RoleBadge } from './ClusterBadges'
import { EditClusterDialog } from './EditClusterDialog'
import { CLUSTERS_NS } from './namespace'
import './clusters.css'

type StoredCluster = ClusterEntry & { id: number }
type DialogState = { kind: 'add' } | { kind: 'edit'; cluster: StoredCluster } | { kind: 'remove'; cluster: StoredCluster }

const hasId = (c: ClusterEntry): c is StoredCluster => c.id !== undefined

function StatusCell({ cluster }: { cluster: ClusterEntry }) {
  const { t } = useTranslation(CLUSTERS_NS)
  if (cluster.error) {
    return (
      <span title={cluster.error}>
        <Badge tone="error">{t('status.error')}</Badge>
      </span>
    )
  }
  if (cluster.active) return <Badge tone="success">{t('status.active')}</Badge>
  if (cluster.source === 'address') return <Badge tone="neutral">{t('status.bookmark')}</Badge>
  return <Badge tone="neutral">{t('status.registered')}</Badge>
}

/**
 * The cluster inventory (design §4.3): environment, read-only flag and the
 * caller's role on each cluster; switching the active cluster; admins add,
 * edit and remove entries.
 */
export function ClustersPage() {
  const { t } = useTranslation(CLUSTERS_NS)
  const { inventory } = useProviders()
  const { mode } = useDataMode()
  const admin = usePermission('admin')
  const clusters = useProviderQuery((p) => p.inventory.listClusters(), [])
  const [dialog, setDialog] = useState<DialogState | null>(null)
  const [activating, setActivating] = useState<string | null>(null)
  const close = () => setDialog(null)

  const activate = async (cluster: ClusterEntry) => {
    setActivating(cluster.name)
    try {
      await inventory.activateCluster(cluster.name)
    } catch (err) {
      setActivating(null)
      notify.error(err)
      return
    }
    // Every view and permission depends on the active cluster: start over.
    window.location.reload()
  }

  const remove = async (cluster: StoredCluster) => {
    await inventory.removeCluster(cluster.name)
    if (cluster.active) {
      // The server picks another cluster for this session; boot again to follow it.
      window.location.reload()
      return
    }
    refreshData()
    notify.success(t('remove.done', { name: cluster.name }))
  }

  const columns: Column<ClusterEntry>[] = [
    {
      key: 'name',
      header: t('columns.name'),
      sortValue: (c) => c.name,
      render: (c) => (
        <span className="clusters__name">
          <Icon name={c.active ? 'pass-filled' : 'vm'} size={14} className={c.active ? 'clusters__active' : undefined} />
          {c.name}
        </span>
      ),
    },
    {
      key: 'environment',
      header: t('columns.environment'),
      sortValue: (c) => c.environment ?? '',
      render: (c) => <EnvironmentBadge environment={c.environment} />,
    },
    {
      key: 'access',
      header: t('columns.access'),
      render: (c) => (c.readOnly ? <ReadOnlyBadge /> : <span className="clusters__muted">{t('readOnly.writable')}</span>),
    },
    { key: 'role', header: t('columns.role'), render: (c) => <RoleBadge role={c.role} /> },
    {
      key: 'platform',
      header: t('columns.platform'),
      sortValue: (c) => c.platform?.provider ?? '',
      render: (c) =>
        c.platform && (c.platform.provider !== 'kubernetes' || c.platform.cloud) ? (
          <ProviderBadge platform={c.platform} showVersion />
        ) : (
          <span className="clusters__muted" title={c.platform?.version ? undefined : t('platform.unknownHint')}>
            {c.platform?.version || '—'}
          </span>
        ),
    },
    {
      key: 'server',
      header: t('columns.server'),
      render: (c) => (
        <span className="clusters__server" title={c.server}>
          {c.server ?? '—'}
        </span>
      ),
    },
    { key: 'status', header: t('columns.status'), render: (c) => <StatusCell cluster={c} /> },
    {
      key: 'actions',
      header: t('columns.actions'),
      width: '1%',
      render: (c) => (
        <span className="clusters__actions">
          {!c.active && (
            <Button
              variant="secondary"
              icon="plug"
              busy={activating === c.name}
              disabled={activating !== null || c.source === 'address' || Boolean(c.error)}
              title={c.source === 'address' ? t('actions.bookmarkHint') : t('actions.activateHint')}
              onClick={() => void activate(c)}
            >
              {t('actions.activate')}
            </Button>
          )}
          <Can perm="admin">
            {hasId(c) && (
              <>
                <Button variant="secondary" icon="edit" onClick={() => setDialog({ kind: 'edit', cluster: c })}>
                  {t('actions.edit')}
                </Button>
                <Button
                  variant="danger"
                  icon="trash"
                  aria-label={t('actions.removeNamed', { name: c.name })}
                  title={t('actions.removeNamed', { name: c.name })}
                  onClick={() => setDialog({ kind: 'remove', cluster: c })}
                />
              </>
            )}
          </Can>
        </span>
      ),
    },
  ]

  const renderBody = () => {
    if (clusters.error) return <Unavailable code={clusters.errorCode} reason={clusters.error} onRetry={refreshData} />
    if (!clusters.data) return <div className="clusters__loading">{t('table.loading')}</div>
    if (clusters.data.length === 0) {
      return (
        <EmptyState
          icon="server-environment"
          title={t('table.empty')}
          hint={admin.allowed ? t('table.emptyAdminHint') : t('table.emptyUserHint')}
        />
      )
    }
    return <Table columns={columns} rows={clusters.data} rowKey={(c) => c.name} />
  }

  const removing = dialog?.kind === 'remove' ? dialog.cluster : null

  return (
    <Page
      title={t('page.title')}
      actions={
        <>
          <Button variant="secondary" icon="refresh" onClick={refreshData}>
            {t('actions.refresh')}
          </Button>
          <Can perm="admin">
            <Button icon="add" onClick={() => setDialog({ kind: 'add' })}>
              {t('actions.add')}
            </Button>
          </Can>
        </>
      }
    >
      {renderBody()}
      <p className="clusters__note">
        <Icon name="info" size={13} />
        {mode === 'live' ? t('note.live') : t('note.mock')}
      </p>
      {dialog?.kind === 'add' && <AddClusterDialog onClose={close} />}
      {dialog?.kind === 'edit' && <EditClusterDialog cluster={dialog.cluster} onClose={close} />}
      <ConfirmDialog
        open={removing !== null}
        danger
        title={t('remove.title', { name: removing?.name ?? '' })}
        message={removing?.active ? t('remove.messageActive') : t('remove.message')}
        confirmLabel={t('remove.confirm')}
        confirmPhrase={removing?.name}
        // A production entry is always confirmed by name, whichever cluster is active.
        requirePhrase={removing?.environment === 'production'}
        onConfirm={() => (removing ? remove(removing) : undefined)}
        onClose={close}
      />
    </Page>
  )
}
