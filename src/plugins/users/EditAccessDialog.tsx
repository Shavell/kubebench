import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { errorMessage, isApiError } from '@/data/apiClient'
import { refreshData, useProviders } from '@/data/DataContext'
import type { AppUser, AssignmentRole, ClusterEntry, GlobalRole } from '@/data/types'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, Select } from '@/shared/form'
import { notify } from '@/shared/notify'
import { USERS_NS } from './namespace'
import { ASSIGNMENT_ROLES, assignmentRoleKey, GLOBAL_ROLES, globalRoleKey } from './roles'

type Access = AssignmentRole | 'none'

interface EditAccessDialogProps {
  user: AppUser
  /** Every cluster of the inventory (as an admin sees it). */
  clusters: ClusterEntry[]
  onClose: () => void
}

function initialAccess(user: AppUser): Record<number, Access> {
  return Object.fromEntries(user.assignments.map((a) => [a.clusterId, a.role]))
}

/**
 * Global role and per-cluster roles of one user (PATCH /api/users/{id},
 * PUT /api/users/{id}/clusters). Viewers read; operators may also change
 * resources and open shells; admins have full access everywhere.
 */
export function EditAccessDialog({ user, clusters, onClose }: EditAccessDialogProps) {
  const { t } = useTranslation(USERS_NS)
  const { users } = useProviders()
  const [role, setRole] = useState<GlobalRole>(user.role)
  const [access, setAccess] = useState<Record<number, Access>>(() => initialAccess(user))
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)
  const assignable = clusters.filter((c): c is ClusterEntry & { id: number } => c.id !== undefined)

  const assignments = assignable.flatMap((c) => {
    const value = access[c.id] ?? 'none'
    return value === 'none' ? [] : [{ clusterId: c.id, role: value }]
  })
  const before = initialAccess(user)
  const assignmentsChanged =
    assignments.length !== user.assignments.length || assignments.some((a) => before[a.clusterId] !== a.role)
  const roleChanged = role !== user.role

  const submit = async () => {
    if (!roleChanged && !assignmentsChanged) {
      onClose()
      return
    }
    setBusy(true)
    setFailure(null)
    try {
      // The role first: if it is refused (last admin), no assignment changes half-apply.
      if (roleChanged) await users.updateUser(user.id, { role })
      if (assignmentsChanged) await users.assignClusters(user.id, assignments)
    } catch (err) {
      setBusy(false)
      // A refused demotion (the last local admin) shows the server's translated reason.
      setFailure(isApiError(err, 'conflict') && roleChanged ? new Error(errorMessage(err)) : err)
      refreshData()
      return
    }
    refreshData()
    notify.success(t('edit.done', { name: user.username }))
    onClose()
  }

  const accessOptions = [
    { value: 'none' as Access, label: t('access.none') },
    ...ASSIGNMENT_ROLES.map((value) => ({ value: value as Access, label: t(assignmentRoleKey(value)) })),
  ]

  return (
    <Dialog
      open
      icon="shield"
      title={t('edit.title', { name: user.username })}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void submit()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('actions.cancel')}
          </Button>
          <Button type="submit" busy={busy}>
            {t('edit.submit')}
          </Button>
        </>
      }
    >
      {user.source !== 'local' && <p className="users-access__notice">{t('external.mappedNote')}</p>}
      <Field label={t('fields.role')} help={t('fields.roleHelp')}>
        <Select
          value={role}
          onChange={setRole}
          disabled={busy}
          options={GLOBAL_ROLES.map((value) => ({ value, label: t(globalRoleKey(value)) }))}
        />
      </Field>
      <div className="users-access">
        <div className="users-access__title">{t('edit.clusterAccess')}</div>
        <p className="users-access__help">{role === 'admin' ? t('edit.adminAccess') : t('edit.accessHelp')}</p>
        {assignable.length === 0 ? (
          <p className="users-access__help">{t('edit.noClusters')}</p>
        ) : (
          <div className="users-access__rows">
            {assignable.map((cluster) => (
              <Field key={cluster.id} layout="inline" label={cluster.name}>
                <Select
                  value={access[cluster.id] ?? 'none'}
                  onChange={(value) => setAccess((current) => ({ ...current, [cluster.id]: value }))}
                  disabled={busy || role === 'admin'}
                  options={accessOptions}
                />
              </Field>
            ))}
          </div>
        )}
      </div>
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
