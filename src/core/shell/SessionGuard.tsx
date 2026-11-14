import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { onSessionLost, type ApiErrorCode } from '@/data/apiClient'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'

/**
 * Tells the user when the server ends the session mid-use (it expired, was
 * revoked by a password change elsewhere, or a password change is now
 * required) instead of letting every request fail one by one. Continuing
 * reloads the app, which then shows the sign-in or password screen.
 */
export function SessionGuard() {
  const { t } = useTranslation(PLATFORM_NS)
  const [lost, setLost] = useState<ApiErrorCode | null>(null)

  useEffect(() => onSessionLost((code) => setLost((current) => current ?? code)), [])

  if (!lost) return null
  const passwordChange = lost === 'password_change_required'
  return (
    <Dialog
      open
      role="alertdialog"
      size="small"
      icon={passwordChange ? 'key' : 'account'}
      title={passwordChange ? t('session.passwordChangeTitle') : t('session.lostTitle')}
      description={passwordChange ? t('session.passwordChangeHint') : t('session.lostHint')}
      onClose={() => setLost(null)}
      footer={
        <>
          <Button variant="secondary" onClick={() => setLost(null)}>
            {t('session.later')}
          </Button>
          <Button onClick={() => window.location.reload()} data-autofocus="">
            {passwordChange ? t('session.continue') : t('session.signInAgain')}
          </Button>
        </>
      }
    />
  )
}
