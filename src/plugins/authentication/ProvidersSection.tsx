import { useTranslation } from 'react-i18next'
import type { AuthProviderInfo, AuthProviderSetting, AuthSettings } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { Icon } from '@/shared/Icon'
import { settingKey, sourceKey } from './labels'
import { AUTH_NS } from './namespace'

function SettingValue({ setting }: { setting: AuthProviderSetting }) {
  const { t } = useTranslation(AUTH_NS)
  if (setting.secretSet !== undefined) {
    return <span className="auth__muted">{t(setting.secretSet ? 'providers.secretSet' : 'providers.secretUnset')}</span>
  }
  if (!setting.value) return <span className="auth__muted">{t('providers.notSet')}</span>
  return <code className="auth__value">{setting.value}</code>
}

function ProviderCard({ provider }: { provider: AuthProviderInfo }) {
  const { t } = useTranslation(AUTH_NS)
  return (
    <div className="auth-card">
      <div className="auth-card__header">
        <Icon name="shield" size={16} />
        <span className="auth-card__name">{provider.name}</span>
        <span className="auth__muted">{t(sourceKey(provider.source))}</span>
        <span className="auth-card__status">
          <Badge tone={provider.status.ok ? 'success' : 'error'}>
            {t(provider.status.ok ? 'providers.reachable' : 'providers.unreachable')}
          </Badge>
        </span>
      </div>
      {!provider.status.ok && provider.status.error && <p className="auth-card__error">{provider.status.error}</p>}
      <dl className="auth-card__settings">
        {provider.settings.map((setting) => (
          <div key={setting.key} className="auth-card__setting">
            <dt>{t(settingKey(setting.key), { defaultValue: setting.key })}</dt>
            <dd>
              <SettingValue setting={setting} />
            </dd>
          </div>
        ))}
      </dl>
    </div>
  )
}

/**
 * The sign-in methods: local accounts (always on — the local admin is the
 * break-glass account) and the configured identity providers, whose
 * settings come from the server's environment and are read-only here.
 */
export function ProvidersSection({ settings }: { settings: AuthSettings }) {
  const { t } = useTranslation(AUTH_NS)
  return (
    <div className="auth-providers">
      <div className="auth-card">
        <div className="auth-card__header">
          <Icon name="account" size={16} />
          <span className="auth-card__name">{t('methods.local')}</span>
          <span className="auth-card__status">
            <Badge tone="success">{t('providers.enabled')}</Badge>
          </span>
        </div>
        <p className="auth__help">{t('providers.breakGlass')}</p>
      </div>
      {settings.providers.map((provider) => (
        <ProviderCard key={provider.source} provider={provider} />
      ))}
      {settings.providers.length === 0 && <p className="auth__help">{t('providers.none')}</p>}
      <dl className="auth-card__settings auth-providers__general">
        <div className="auth-card__setting">
          <dt>{t('settings.keys.publicUrl')}</dt>
          <dd>
            <code className="auth__value">{settings.publicUrl}</code>
            {!settings.publicUrlConfigured && <span className="auth__muted"> {t('providers.publicUrlDerived')}</span>}
          </dd>
        </div>
        <div className="auth-card__setting">
          <dt>{t('settings.keys.requireGroup')}</dt>
          <dd>{t(settings.requireGroup ? 'providers.requireGroupOn' : 'providers.requireGroupOff')}</dd>
        </div>
      </dl>
      <p className="auth__help">{t('providers.configHint')}</p>
    </div>
  )
}
