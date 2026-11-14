import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { isApiError } from '@/data/apiClient'
import { refreshData, useProviders } from '@/data/DataContext'
import { MIN_PASSWORD_LENGTH, passwordLength, USERNAME_PATTERN } from '@/data/rules'
import type { GlobalRole } from '@/data/types'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, Select, TextInput } from '@/shared/form'
import { notify } from '@/shared/notify'
import { USERS_NS } from './namespace'
import { GLOBAL_ROLES, globalRoleKey } from './roles'

/** New account with an initial password, which the user replaces at first sign-in. */
export function CreateUserDialog({ onClose }: { onClose: () => void }) {
  const { t } = useTranslation(USERS_NS)
  const { users } = useProviders()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [role, setRole] = useState<GlobalRole>('user')
  const [attempted, setAttempted] = useState(false)
  const [taken, setTaken] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const name = username.trim()
  const problems = {
    username: !USERNAME_PATTERN.test(name) ? t('create.errors.username') : taken ? t('create.errors.taken') : null,
    password:
      passwordLength(password) < MIN_PASSWORD_LENGTH ? t('create.errors.password', { count: MIN_PASSWORD_LENGTH }) : null,
  }
  const errors = attempted || taken ? problems : { username: null, password: null }

  const submit = async () => {
    setAttempted(true)
    if (problems.username || problems.password) return
    setBusy(true)
    setFailure(null)
    try {
      await users.createUser(name, password, role)
    } catch (err) {
      setBusy(false)
      if (isApiError(err, 'conflict')) setTaken(true)
      else setFailure(err)
      return
    }
    refreshData()
    notify.success(t('create.done', { name }))
    onClose()
  }

  return (
    <Dialog
      open
      icon="person-add"
      title={t('create.title')}
      description={t('create.description')}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void submit()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('actions.cancel')}
          </Button>
          <Button type="submit" busy={busy}>
            {t('create.submit')}
          </Button>
        </>
      }
    >
      <Field label={t('fields.username')} help={t('create.usernameHelp')} error={errors.username} required>
        <TextInput
          value={username}
          onChange={(value) => {
            setUsername(value)
            setTaken(false)
          }}
          monospace
          autoComplete="off"
          disabled={busy}
        />
      </Field>
      <Field
        label={t('fields.initialPassword')}
        help={t('create.passwordHelp', { count: MIN_PASSWORD_LENGTH })}
        error={errors.password}
        required
      >
        <TextInput type="password" value={password} onChange={setPassword} autoComplete="new-password" disabled={busy} />
      </Field>
      <Field label={t('fields.role')} help={t('fields.roleHelp')}>
        <Select
          value={role}
          onChange={setRole}
          disabled={busy}
          options={GLOBAL_ROLES.map((value) => ({ value, label: t(globalRoleKey(value)) }))}
        />
      </Field>
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
