import { useEffect, useState } from 'react'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { fetchSignInMethods } from '@/data/api/identity'
import { apiPost, errorMessage, isApiError } from '@/data/apiClient'
import type { SignInFailure, SignInMethod } from '@/data/types'
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

/**
 * Full-screen sign-in shown when the backend requires authentication:
 * "Sign in with <provider>" for redirect providers (OIDC), and the
 * username/password form for local accounts and directory providers
 * (design §3.6). Local accounts always work (break-glass admin).
 */
export function LoginScreen({ failure }: { failure?: SignInFailure | 'unknown' | null }) {
  const { t } = useTranslation()
  const [username, setUsername] = useState('')
  const [password, setPassword] = useState('')
  const [error, setError] = useState<string | null>(() =>
    !failure ? null : failure === 'unknown' ? t('login.errors.failed') : t(`login.external.failures.${failure}`),
  )
  const [busy, setBusy] = useState(false)
  const [providers, setProviders] = useState<SignInMethod[]>([])
  const [source, setSource] = useState('local')

  useEffect(() => {
    let cancelled = false
    // Without the list (an older server) the local form still works.
    fetchSignInMethods()
      .then((methods) => {
        if (!cancelled) setProviders(methods.providers)
      })
      .catch(() => undefined)
    return () => {
      cancelled = true
    }
  }, [])

  const redirects = providers.filter((p) => p.type === 'redirect' && p.loginUrl)
  const directories = providers.filter((p) => p.type === 'password')

  const submit = async (e: React.FormEvent) => {
    e.preventDefault()
    setBusy(true)
    setError(null)
    try {
      await apiPost('/auth/login', { username, password, ...(source === 'local' ? {} : { source }) })
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
        {redirects.map((p) => (
          <button
            key={p.source}
            type="button"
            className="kb-button login__provider"
            disabled={busy}
            onClick={() => {
              setBusy(true)
              window.location.assign(p.loginUrl!)
            }}
          >
            <Icon name="shield" size={14} />
            {t('login.external.signInWith', { name: p.name })}
          </button>
        ))}
        {redirects.length > 0 && (
          <div className="login__divider" role="separator">
            <span>{t('login.external.or')}</span>
          </div>
        )}
        {directories.length > 0 && (
          <label>
            {t('login.external.account')}
            <select value={source} onChange={(e) => setSource(e.target.value)} disabled={busy}>
              <option value="local">{t('login.external.localAccount')}</option>
              {directories.map((p) => (
                <option key={p.source} value={p.source}>
                  {p.name}
                </option>
              ))}
            </select>
          </label>
        )}
        <label>
          {t('login.username')}
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoFocus={redirects.length === 0}
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
        <button
          className={redirects.length > 0 ? 'kb-button login__submit login__submit--quiet' : 'kb-button login__submit'}
          disabled={busy || !username || !password}
        >
          {busy ? t('login.signingIn') : t('login.signIn')}
        </button>
      </form>
    </div>
  )
}
