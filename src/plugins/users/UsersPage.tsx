import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { errorMessage, isApiError } from '@/data/apiClient'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import { useDataMode } from '@/data/mode'
import type { AppUser } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { EmptyState } from '@/shared/EmptyState'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Page } from '@/shared/Page'
import { Table, type Column } from '@/shared/Table'
import { Unavailable } from '@/shared/Unavailable'
import { CreateUserDialog } from './CreateUserDialog'
import { EditAccessDialog } from './EditAccessDialog'
import { USERS_NS } from './namespace'
import { ResetPasswordDialog } from './ResetPasswordDialog'
import { assignmentRoleKey, globalRoleKey } from './roles'
import './users.css'

type DialogState =
  | { kind: 'create' }
  | { kind: 'edit'; user: AppUser }
  | { kind: 'reset'; user: AppUser }
  | { kind: 'delete'; user: AppUser }

function AccessCell({ user }: { user: AppUser }) {
  const { t } = useTranslation(USERS_NS)
  if (user.role === 'admin') return <span className="users__muted">{t('access.allClusters')}</span>
  if (user.assignments.length === 0) return <span className="users__muted">{t('access.noClusters')}</span>
  return (
    <span className="users__chips">
      {user.assignments.map((a) => (
        <span key={a.clusterId} className="users__chip">
          {a.cluster}
          <span className="users__chip-role">{t(assignmentRoleKey(a.role))}</span>
        </span>
      ))}
    </span>
  )
}

/** Admin user management: global roles, per-cluster roles, password resets (design §4.2). */
export function UsersPage() {
  const { t } = useTranslation(USERS_NS)
  const { users: usersProvider } = useProviders()
  const me = useDataMode().user
  const users = useProviderQuery((p) => p.users.listUsers(), [])
  const clusters = useProviderQuery((p) => p.inventory.listClusters(), [])
  const [dialog, setDialog] = useState<DialogState | null>(null)
  const close = () => setDialog(null)

  const deleteUser = async (user: AppUser) => {
    try {
      await usersProvider.deleteUser(user.id)
    } catch (err) {
      // The server refuses to delete your own account or the last local
      // admin (409); its translated reason says which and what to do.
      throw isApiError(err, 'conflict') ? new Error(errorMessage(err)) : err
    }
    refreshData()
    notify.success(t('delete.done', { name: user.username }))
  }

  const columns: Column<AppUser>[] = [
    {
      key: 'user',
      header: t('columns.user'),
      sortValue: (u) => u.username,
      render: (u) => (
        <span className="users__name">
          <Icon name="account" size={14} />
          {u.username}
          {u.username === me && <span className="users__muted">{t('table.you')}</span>}
          {u.source !== 'local' && (
            <span title={t('external.sourceHint')}>
              <Badge tone="info">{t(`external.source.${u.source}`)}</Badge>
            </span>
          )}
          {(u.displayName || u.email) && (
            <span className="users__muted">{[u.displayName, u.email].filter(Boolean).join(' · ')}</span>
          )}
        </span>
      ),
    },
    {
      key: 'role',
      header: t('columns.role'),
      sortValue: (u) => u.role,
      render: (u) => <Badge tone={u.role === 'admin' ? 'info' : 'neutral'}>{t(globalRoleKey(u.role))}</Badge>,
    },
    { key: 'access', header: t('columns.access'), render: (u) => <AccessCell user={u} /> },
    {
      key: 'status',
      header: t('columns.status'),
      render: (u) =>
        u.mustChangePassword ? <Badge tone="warning">{t('table.mustChangePassword')}</Badge> : null,
    },
    {
      key: 'actions',
      header: t('columns.actions'),
      width: '1%',
      render: (u) => {
        const self = u.username === me
        const external = u.source !== 'local'
        return (
          <span className="users__actions">
            <Button
              variant="secondary"
              icon="shield"
              // The editor lists the clusters, so it waits for the inventory.
              disabled={!clusters.data}
              title={clusters.error ?? undefined}
              onClick={() => setDialog({ kind: 'edit', user: u })}
            >
              {t('actions.editAccess')}
            </Button>
            <Button
              variant="secondary"
              icon="key"
              disabled={self || external}
              title={external ? t('external.resetHint') : self ? t('actions.resetOwnHint') : undefined}
              onClick={() => setDialog({ kind: 'reset', user: u })}
            >
              {t('actions.resetPassword')}
            </Button>
            <Button
              variant="danger"
              icon="trash"
              aria-label={t('actions.deleteNamed', { name: u.username })}
              title={self ? t('actions.deleteOwnHint') : t('actions.deleteNamed', { name: u.username })}
              disabled={self}
              onClick={() => setDialog({ kind: 'delete', user: u })}
            />
          </span>
        )
      },
    },
  ]

  const renderBody = () => {
    if (users.error) return <Unavailable code={users.errorCode} reasonCode={users.errorReason} reason={users.error} onRetry={refreshData} />
    if (!users.data) return <div className="users__loading">{t('table.loading')}</div>
    if (users.data.length === 0) return <EmptyState icon="organization" title={t('table.empty')} />
    return <Table columns={columns} rows={users.data} rowKey={(u) => String(u.id)} />
  }

  return (
    <Page
      title={t('page.title')}
      actions={
        <>
          <Button variant="secondary" icon="refresh" onClick={refreshData}>
            {t('actions.refresh')}
          </Button>
          <Button icon="person-add" onClick={() => setDialog({ kind: 'create' })}>
            {t('actions.add')}
          </Button>
        </>
      }
    >
      {renderBody()}
      {dialog?.kind === 'create' && <CreateUserDialog onClose={close} />}
      {dialog?.kind === 'edit' && (
        <EditAccessDialog user={dialog.user} clusters={clusters.data ?? []} onClose={close} />
      )}
      {dialog?.kind === 'reset' && <ResetPasswordDialog user={dialog.user} onClose={close} />}
      <ConfirmDialog
        open={dialog?.kind === 'delete'}
        danger
        title={t('delete.title', { name: dialog?.kind === 'delete' ? dialog.user.username : '' })}
        message={t('delete.message')}
        confirmLabel={t('delete.confirm')}
        onConfirm={() => (dialog?.kind === 'delete' ? deleteUser(dialog.user) : undefined)}
        onClose={close}
      />
    </Page>
  )
}
