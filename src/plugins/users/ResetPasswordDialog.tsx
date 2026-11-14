import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { refreshData, useProviders } from '@/data/DataContext'
import { MIN_PASSWORD_LENGTH, passwordLength } from '@/data/rules'
import type { AppUser } from '@/data/types'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, TextInput } from '@/shared/form'
import { notify } from '@/shared/notify'
import { USERS_NS } from './namespace'

/**
 * Admin password reset (PATCH /api/users/{id} with a password). The server
 * signs the user out everywhere and makes them choose a new password at
 * the next sign-in, since the admin knows this one.
 */
export function ResetPasswordDialog({ user, onClose }: { user: AppUser; onClose: () => void }) {
  const { t } = useTranslation(USERS_NS)
  const { users } = useProviders()
  const [password, setPassword] = useState('')
  const [confirm, setConfirm] = useState('')
  const [attempted, setAttempted] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const problems = {
    password:
      passwordLength(password) < MIN_PASSWORD_LENGTH ? t('reset.errors.password', { count: MIN_PASSWORD_LENGTH }) : null,
    confirm: confirm !== password ? t('reset.errors.mismatch') : null,
  }
  const errors = attempted ? problems : { password: null, confirm: null }

  const submit = async () => {
    setAttempted(true)
    if (problems.password || problems.confirm) return
    setBusy(true)
    setFailure(null)
    try {
      await users.updateUser(user.id, { password })
    } catch (err) {
      setBusy(false)
      setFailure(err)
      return
    }
    refreshData()
    notify.success(t('reset.done', { name: user.username }))
    onClose()
  }

  return (
    <Dialog
      open
      size="small"
      icon="key"
      title={t('reset.title', { name: user.username })}
      description={t('reset.description', { name: user.username })}
      busy={busy}
      onClose={onClose}
      onSubmit={() => void submit()}
      footer={
        <>
          <Button variant="secondary" onClick={onClose} disabled={busy}>
            {t('actions.cancel')}
          </Button>
          <Button type="submit" busy={busy}>
            {t('reset.submit')}
          </Button>
        </>
      }
    >
      <Field label={t('fields.newPassword')} help={t('reset.help', { count: MIN_PASSWORD_LENGTH })} error={errors.password} required>
        <TextInput type="password" value={password} onChange={setPassword} autoComplete="new-password" disabled={busy} />
      </Field>
      <Field label={t('fields.confirmPassword')} error={errors.confirm} required>
        <TextInput type="password" value={confirm} onChange={setConfirm} autoComplete="new-password" disabled={busy} />
      </Field>
      <ErrorNotice error={failure} />
    </Dialog>
  )
}
