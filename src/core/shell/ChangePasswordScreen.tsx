import type { FormEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import { signOut } from '@/core/session/signOut'
import { usePasswordChange } from '@/core/session/usePasswordChange'
import { useProviders } from '@/data/DataContext'
import { useDataMode } from '@/data/mode'
import { Button } from '@/shared/Button'
import { Icon } from '@/shared/Icon'
import { PasswordFields } from './PasswordFields'
import './fullscreen.css'

/**
 * Shown instead of the workbench while the server requires a new password
 * (`mustChangePassword`: the default admin password, or one an admin set).
 * Every cluster request is refused until then, so there is no way around
 * it but signing out. After the change the app boots again.
 */
export function ChangePasswordScreen() {
  const { t } = useTranslation(PLATFORM_NS)
  const providers = useProviders()
  const { user } = useDataMode()
  const form = usePasswordChange(() => window.location.reload())

  const onSubmit = (event: FormEvent) => {
    event.preventDefault()
    void form.submit()
  }

  return (
    <main className="kb-fullscreen">
      <form className="kb-fullscreen__card" onSubmit={onSubmit} noValidate>
        <div className="kb-fullscreen__brand">
          <Icon name="key" size={36} />
          <h1 className="kb-fullscreen__title">{t('password.forcedTitle')}</h1>
          <p className="kb-fullscreen__text">{t('password.forcedDescription')}</p>
        </div>
        <PasswordFields form={form} username={user} />
        <div className="kb-fullscreen__actions">
          <Button type="submit" busy={form.busy}>
            {t('password.submit')}
          </Button>
        </div>
        <div className="kb-fullscreen__footer">
          <span>{t('password.signedInAs', { user })}</span>
          <button
            type="button"
            className="kb-fullscreen__link"
            disabled={form.busy}
            onClick={() => void signOut(providers.session)}
          >
            {t('session.signOut')}
          </button>
        </div>
      </form>
    </main>
  )
}
