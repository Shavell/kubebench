import { useEffect, useState } from 'react'
import type { TFunction } from 'i18next'
import { useTranslation } from 'react-i18next'
import { fetchSignInMethods } from '@/data/api/identity'
import { apiPost, errorMessage, isApiError } from '@/data/apiClient'
import type { SignInFailure, SignInMethod } from '@/data/types'
import { Icon } from '@/shared/Icon'
import './LoginScreen.css'

/** Reason prefix of a refused external sign-in: `auth.signin.<failure class>`. */
const SIGN_IN_REFUSAL = 'auth.signin.'

/** Translated reason for a failed sign-in; unknown failures keep the server's message. */
function loginErrorMessage(err: unknown, t: TFunction): string {
  if (!isApiError(err)) return errorMessage(err)
  // A refused directory sign-in names its failure class, shown like the
  // failures of an OIDC sign-in.
  if (err.reason?.startsWith(SIGN_IN_REFUSAL)) {
    const failure = err.reason.slice(SIGN_IN_REFUSAL.length)
    return t(`login.external.failures.${failure}`, { defaultValue: errorMessage(err) })
  }
  if (err.status === 401) return t('login.errors.invalidCredentials')
  if (err.code === 'rate_limited') return t('login.errors.rateLimited')
  if (err.code === 'network') return t('login.errors.network')
  return errorMessage(err) || t('login.errors.failed')
}

/**
 * Full-screen sign-in shown when the backend requires authentication:
 * "Sign in with <provider>" for redirect providers (OIDC), and the
 * username/password form for directory providers (LDAP / Active Directory)
 * and local accounts (design §3.6). With a directory, the form signs in
 * there unless the user picks the KubeBench account; local accounts always
 * work (break-glass admin).
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
  // null until the user picks: then the first directory, else local.
  const [picked, setPicked] = useState<string | null>(null)

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
  const source = picked ?? directories[0]?.source ?? 'local'
  const sourceOptions = [
    ...directories.map((p) => ({ value: p.source, label: p.name })),
    { value: 'local', label: t('login.external.localAccount') },
  ]

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
            <span>{t(directories.length > 0 ? 'login.external.orPassword' : 'login.external.or')}</span>
          </div>
        )}
        {directories.length > 0 && (
          <div className="login__sources" role="radiogroup" aria-label={t('login.external.account')}>
            {sourceOptions.map((option) => (
              <button
                key={option.value}
                type="button"
                role="radio"
                aria-checked={source === option.value}
                className={source === option.value ? 'login__source login__source--active' : 'login__source'}
                disabled={busy}
                onClick={() => {
                  setPicked(option.value)
                  setError(null)
                }}
              >
                {option.label}
              </button>
            ))}
          </div>
        )}
        <label>
          {t('login.username')}
          <input
            value={username}
            onChange={(e) => setUsername(e.target.value)}
            autoFocus={redirects.length === 0}
            autoComplete="username"
            spellCheck={false}
            aria-describedby={source !== 'local' ? 'login-username-hint' : undefined}
          />
          {source !== 'local' && (
            <span id="login-username-hint" className="login__hint">
              {t('login.external.directoryHint')}
            </span>
          )}
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
