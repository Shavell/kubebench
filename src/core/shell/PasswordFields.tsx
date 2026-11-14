import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import type { PasswordChangeForm } from '@/core/session/usePasswordChange'
import { MIN_PASSWORD_LENGTH } from '@/data/rules'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, TextInput } from '@/shared/form'

/** Current / new / confirm fields of a change-password form, plus its error. */
export function PasswordFields({ form, username }: { form: PasswordChangeForm; username?: string }) {
  const { t } = useTranslation(PLATFORM_NS)
  return (
    <>
      {/* Lets password managers file the new password under the right account. */}
      {username && <input type="text" name="username" autoComplete="username" value={username} readOnly hidden />}
      <Field label={t('password.current')} error={form.errors.current} required>
        <TextInput
          type="password"
          value={form.current}
          onChange={form.setCurrent}
          autoComplete="current-password"
          disabled={form.busy}
        />
      </Field>
      <Field
        label={t('password.new')}
        help={t('password.newHelp', { count: MIN_PASSWORD_LENGTH })}
        error={form.errors.next}
        required
      >
        <TextInput
          type="password"
          value={form.next}
          onChange={form.setNext}
          autoComplete="new-password"
          disabled={form.busy}
        />
      </Field>
      <Field label={t('password.confirm')} error={form.errors.confirm} required>
        <TextInput
          type="password"
          value={form.confirm}
          onChange={form.setConfirm}
          autoComplete="new-password"
          disabled={form.busy}
        />
      </Field>
      <ErrorNotice error={form.failure} />
    </>
  )
}
