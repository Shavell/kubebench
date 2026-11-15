import { useMemo, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { refreshData } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { CORE_NS } from '@/i18n'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { EmptyState } from '@/shared/EmptyState'
import { notify } from '@/shared/notify'
import { Table, type Column } from '@/shared/Table'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { helmData, type HelmRepo } from '../data'
import { DateTime } from '../DateTime'
import { NS } from '../i18n'
import { openInstallTab } from '../tabs'
import { AddRepoDialog } from './AddRepoDialog'
import '@/shared/tabPage.css'

/**
 * Chart repositories (design §7.4), shared by every cluster: admins add
 * and remove them, operators update their indexes.
 */
export function RepoListPage() {
  const { t } = useTranslation(NS)
  const admin = usePermission('admin')
  const write = usePermission('write')
  const query = usePluginQuery(() => helmData().listRepos(), [])
  const [adding, setAdding] = useState(false)
  const [removing, setRemoving] = useState<HelmRepo | null>(null)
  const [updating, setUpdating] = useState(false)

  const update = async () => {
    setUpdating(true)
    try {
      const results = await helmData().updateRepos()
      const failed = results.filter((r) => !r.ok)
      if (failed.length === 0) notify.success(t('repos.updated', { count: results.length }))
      else notify.error(t('repos.updateFailed', { names: failed.map((r) => r.name).join(', ') }), { detail: failed.map((r) => `${r.name}: ${r.error}`).join('\n') })
      refreshData()
    } catch (err) {
      notify.error(err)
    } finally {
      setUpdating(false)
    }
  }

  const columns = useMemo<Column<HelmRepo>[]>(
    () => [
      { key: 'name', header: t('common.name', { ns: CORE_NS }), render: (r) => r.name, sortValue: (r) => r.name },
      { key: 'url', header: t('repos.columns.url'), render: (r) => r.url, sortValue: (r) => r.url },
      {
        key: 'charts',
        header: t('repos.columns.charts'),
        render: (r) => (r.error ? <span className="text-warning" title={r.error}>{t('repos.noIndex')}</span> : String(r.chartCount)),
        sortValue: (r) => r.chartCount,
      },
      { key: 'updated', header: t('repos.columns.updated'), render: (r) => <DateTime value={r.updated} />, sortValue: (r) => r.updated },
      {
        key: 'auth',
        header: t('repos.columns.auth'),
        render: (r) => <Badge tone={r.authenticated ? 'info' : 'neutral'}>{r.authenticated ? t('repos.authYes') : t('repos.authNo')}</Badge>,
      },
      {
        key: 'actions',
        header: '',
        width: '48px',
        render: (r) => (
          <Button
            variant="secondary"
            icon="trash"
            aria-label={t('repos.remove', { name: r.name })}
            title={admin.reason ?? t('repos.remove', { name: r.name })}
            disabled={!admin.allowed}
            onClick={(event) => {
              event.stopPropagation()
              setRemoving(r)
            }}
          />
        ),
      },
    ],
    [t, admin.allowed, admin.reason],
  )

  return (
    <div className="page">
      <Toolbar title={t('repos.title')}>
        <Button variant="secondary" icon="cloud-download" onClick={openInstallTab} disabled={!write.allowed} title={write.reason ?? undefined}>
          {t('releases.install')}
        </Button>
        <Button variant="secondary" icon="sync" busy={updating} disabled={!write.allowed} title={write.reason ?? t('repos.updateHint')} onClick={() => void update()}>
          {t('repos.update')}
        </Button>
        <Button icon="add" disabled={!admin.allowed} title={admin.reason ?? undefined} onClick={() => setAdding(true)}>
          {t('repos.add.open')}
        </Button>
      </Toolbar>
      {adding && (
        <AddRepoDialog
          onAdded={(repo) => {
            notify.success(t('repos.add.done', { name: repo.name, count: repo.chartCount }))
            refreshData()
          }}
          onClose={() => setAdding(false)}
        />
      )}
      {removing && (
        <ConfirmDialog
          open
          danger
          title={t('repos.removeDialog.title', { name: removing.name })}
          message={t('repos.removeDialog.message')}
          confirmLabel={t('repos.removeDialog.confirm')}
          confirmPhrase={removing.name}
          onConfirm={async () => {
            await helmData().removeRepo(removing.name)
            notify.success(t('repos.removeDialog.done', { name: removing.name }))
            refreshData()
          }}
          onClose={() => setRemoving(null)}
        />
      )}
      {query.error ? (
        <Unavailable code={query.errorCode} reason={query.error} title={t('repos.unavailable')} onRetry={refreshData} />
      ) : query.loading && !query.data ? (
        <div className="page-loading">{t('repos.loading')}</div>
      ) : (query.data ?? []).length === 0 ? (
        <EmptyState icon="repo" title={t('repos.empty')} hint={t('repos.emptyHint')} />
      ) : (
        <div className="page__scroll">
          <Table columns={columns} rows={query.data ?? []} rowKey={(r) => r.name} />
        </div>
      )}
    </div>
  )
}
