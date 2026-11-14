import { useState } from 'react'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { apiPost, errorMessage, isApiError } from '@/data/apiClient'
import { Icon } from '@/shared/Icon'
import './LoginScreen.css'

/** Translated reason for a failed sign-in; unknown failures keep the server's message. */
function loginErrorMessage(err: unknown, t: TFunction): string {
  if (!isApiError(err)) return errorMessage(err)
  if (err.status === 401) return t('login.errors.invalidCredentials')
  if (err.code === 'rate_limited') return t('login.errors.rateLimited')
  if (err.code === 'network') return t('login.errors.network')
  return err.message || t('login.errors.failed')
}

/** Full-screen sign-in shown when the backend requires authentication. */
export function LoginScreen() {
  const { t } = useTranslation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await apiPost('/auth/login', { username, password })
      window.location.reload()
    } catch (err) {
      setError(loginErrorMessage(err, t))
      setBusy(false)
    }
  }

  return (
    <div className="login">
      <form className="login__card" onSubmit={submit}>
        <div className="login__brand">
          <Icon name="dashboard" size={40} />
          <div className="login__title">KubeBench</div>
          <div className="login__subtitle">{t('login.subtitle')}</div>
        </div>
        {error && (
          <div className="login__error" role="alert">
            <Icon name="error" size={13} /> {error}
          </div>
        )}
        <label>
          {t('login.username')}
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoFocus
            autoComplete="username"
            spellCheck={false}
          />
        </label>
        <label>
          {t('login.password')}
          <input
            type="password"
            value={password}
            onChange={(e) => setPassword(e.target.value)}
            autoComplete="current-password"
          />
        </label>
        <button className="kb-button login__submit" disabled={busy || !username || !password}>
          {busy ? t('login.signingIn') : t('login.signIn')}
        </button>
      </form>
    </div>
  )
}
