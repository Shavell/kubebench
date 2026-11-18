import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { useProviders } from '@/data/DataContext'
import { USERNAME_PATTERN } from '@/data/rules'
import type { AuthProviderInfo, ExternalAuthSource, SignInPreview } from '@/data/types'
import { formatDateTime } from '@/i18n/format'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { Field, Select, Textarea, TextInput } from '@/shared/form'
import { Icon } from '@/shared/Icon'
import { failureKey, roleKey, sourceKey } from './labels'
import { AUTH_NS } from './namespace'

/** Splits the groups field: one group per line (DNs contain commas). */
function parseGroups(text: string): string[] | undefined {
  const groups = text
    .split('\n')
    .map((g) => g.trim())
    .filter(Boolean)
  return groups.length > 0 ? groups : undefined
}

function PreviewResult({ preview }: { preview: SignInPreview }) {
  const { t } = useTranslation(AUTH_NS)
  return (
    <div className="auth-card auth-preview" aria-live="polite">
      <div className="auth-card__header">
        <Icon name={preview.allowed ? 'pass' : 'error'} size={16} />
        <span className="auth-card__name">{preview.username}</span>
        <span className="auth-card__status">
          <Badge tone={preview.allowed ? 'success' : 'error'}>
            {t(preview.allowed ? 'preview.allowed' : 'preview.refused')}
          </Badge>
        </span>
      </div>
      {preview.reason && <p className="auth-card__error">{t(failureKey(preview.reason))}</p>}
      <dl className="auth-card__settings">
        <div className="auth-card__setting">
          <dt>{t('preview.groups')}</dt>
          <dd>
            {preview.groups.length > 0 ? (
              <span className="auth__chips">
                {preview.groups.map((g) => (
                  <code key={g} className="auth__chip">
                    {g}
                  </code>
                ))}
              </span>
            ) : (
              <span className="auth__muted">{t('preview.noGroups')}</span>
            )}
            <span className="auth__muted">
              {' '}
              {t(`preview.groupsFrom.${preview.groupsFrom}`, {
                time: preview.lastSignIn ? formatDateTime(preview.lastSignIn) : '',
              })}
            </span>
          </dd>
        </div>
        <div className="auth-card__setting">
          <dt>{t('preview.globalRole')}</dt>
          <dd>{t(preview.admin ? 'roles.admin' : 'roles.user')}</dd>
        </div>
        <div className="auth-card__setting">
          <dt>{t('preview.clusters')}</dt>
          <dd>
            {preview.admin ? (
              t('preview.allClusters')
            ) : preview.clusters.length === 0 ? (
              <span className="auth__muted">{t('preview.noClusters')}</span>
            ) : (
              <span className="auth__chips">
                {preview.clusters.map((c) => (
                  <span key={c.cluster} className="auth__chip" title={c.exists ? undefined : t('mappings.unknownClusterHint')}>
                    {c.cluster} · {t(roleKey(c.role))}
                    {!c.exists && <Icon name="warning" size={12} />}
                  </span>
                ))}
              </span>
            )}
          </dd>
        </div>
        <div className="auth-card__setting">
          <dt>{t('preview.matched')}</dt>
          <dd>{t('preview.matchedCount', { count: preview.matched.length })}</dd>
        </div>
        <div className="auth-card__setting">
          <dt>{t('preview.account')}</dt>
          <dd>
            {preview.existingSource === ''
              ? t('preview.accountNew')
              : preview.existingSource === preview.source
                ? t('preview.accountExisting')
                : t('preview.accountOther', { source: t(`sources.${preview.existingSource}`) })}
          </dd>
        </div>
      </dl>
    </div>
  )
}

/**
 * "Who would I be": what signing in would grant a user, computed by the
 * server from the current mappings without creating a user or a session.
 */
export function PreviewSection({ providers }: { providers: AuthProviderInfo[] }) {
  const { t } = useTranslation(AUTH_NS)
  const { identity } = useProviders()
  const sources: ExternalAuthSource[] = providers.length > 0 ? providers.map((p) => p.source) : ['oidc', 'ldap']
  const [source, setSource] = useState<ExternalAuthSource>(sources[0])
  const [username, setUsername] = useState('')
  const [groupsText, setGroupsText] = useState('')
  const [busy, setBusy] = useState(false)
  const [failure, setFailure] = useState<unknown>(null)
  const [preview, setPreview] = useState<SignInPreview | null>(null)
  const [attempted, setAttempted] = useState(false)

  const name = username.trim()
  const selected = sources.includes(source) ? source : sources[0]
  // A directory resolves its login forms (DOMAIN\user, user@domain).
  const directory = selected === 'ldap'
  const nameValid = directory ? name.length > 0 : USERNAME_PATTERN.test(name)
  const usernameError =
    attempted && !nameValid ? t(directory ? 'preview.errors.login' : 'preview.errors.username') : null

  const run = async () => {
    setAttempted(true)
    if (!nameValid) return
    setBusy(true)
    setFailure(null)
    try {
      setPreview(await identity.preview(selected, name, parseGroups(groupsText)))
    } catch (err) {
      setPreview(null)
      setFailure(err)
    } finally {
      setBusy(false)
    }
  }

  return (
    <div className="auth-preview-form">
      <p className="auth__help">{t('preview.help')}</p>
      <form
        className="auth-preview-form__fields"
        onSubmit={(e) => {
          e.preventDefault()
          void run()
        }}
      >
        <Field label={t('preview.provider')}>
          <Select
            value={selected}
            onChange={setSource}
            disabled={busy}
            options={sources.map((value) => ({
              value,
              label: providers.find((p) => p.source === value)?.name ?? t(sourceKey(value)),
            }))}
          />
        </Field>
        <Field
          label={t('preview.username')}
          help={directory ? t('preview.loginHelp') : undefined}
          error={usernameError}
          required
        >
          <TextInput value={username} onChange={setUsername} monospace autoComplete="off" disabled={busy} />
        </Field>
        <Field label={t('preview.groupsInput')} help={t('preview.groupsHelp')}>
          <Textarea value={groupsText} onChange={setGroupsText} monospace rows={3} disabled={busy} />
        </Field>
        <div>
          <Button type="submit" icon="play" busy={busy}>
            {t('preview.run')}
          </Button>
        </div>
      </form>
      <ErrorNotice error={failure} />
      {preview && <PreviewResult preview={preview} />}
    </div>
  )
}
