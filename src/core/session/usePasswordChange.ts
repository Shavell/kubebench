import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { isApiError } from '@/data/apiClient'
import { useProviders } from '@/data/DataContext'
import { MIN_PASSWORD_LENGTH, passwordLength } from '@/data/rules'

export interface PasswordChangeForm {
  current: string
  next: string
  confirm: string
  setCurrent: (value: string) => void
  setNext: (value: string) => void
  setConfirm: (value: string) => void
  /** Field errors (translated); shown once the user tried to submit. */
  errors: { current: string | null; next: string | null; confirm: string | null }
  busy: boolean
  /** A failure that is not about a field (rate limit, network, …). */
  failure: unknown
  submit: () => Promise<void>
}

/**
 * State of the change-password form (POST /api/auth/password). Validates
 * like the server (§4.2: at least 8 characters, different from the
 * current one) and maps the server's "current password is incorrect"
 * (403 forbidden) onto the current-password field.
 */
export function usePasswordChange(onChanged: () => void): PasswordChangeForm {
  const { t } = useTranslation(PLATFORM_NS)
  const { session } = useProviders()
  const [current, setCurrentValue] = useState('')
  const [next, setNext] = useState('')
  const [confirm, setConfirm] = useState('')
  const [attempted, setAttempted] = useState(false)
  const [currentRejected, setCurrentRejected] = useState(false)
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)

  const problems = {
    current: !current
      ? t('password.errors.currentRequired')
      : currentRejected
        ? t('password.errors.currentIncorrect')
        : null,
    next:
      passwordLength(next) < MIN_PASSWORD_LENGTH
        ? t('password.errors.tooShort', { count: MIN_PASSWORD_LENGTH })
        : next === current
          ? t('password.errors.unchanged')
          : null,
    confirm: confirm !== next ? t('password.errors.mismatch') : null,
  }
  // Problems are only pointed out after a first submit, not while typing.
  const errors = attempted ? problems : { current: null, next: null, confirm: null }

  const submit = async () => {
    setAttempted(true)
    if (problems.current || problems.next || problems.confirm || busy) return
    setBusy(true)
    setFailure(null)
    try {
      await session.changePassword(current, next)
    } catch (err) {
      setBusy(false)
      if (isApiError(err) && err.status === 403 && err.code === 'forbidden') setCurrentRejected(true)
      else setFailure(err)
      return
    }
    setBusy(false)
    onChanged()
  }

  return {
    current,
    next,
    confirm,
    setCurrent: (value) => {
      setCurrentValue(value)
      setCurrentRejected(false)
    },
    setNext,
    setConfirm,
    errors,
    busy,
    failure,
    submit,
  }
}
