import { useTranslation } from 'react-i18next'
import { refreshData, useProviderQuery } from '@/data/DataContext'
import { Button } from '@/shared/Button'
import { Page, PageSection } from '@/shared/Page'
import { Unavailable } from '@/shared/Unavailable'
import { MappingsSection } from './MappingsSection'
import { AUTH_NS } from './namespace'
import { PreviewSection } from './PreviewSection'
import { ProvidersSection } from './ProvidersSection'
import './authentication.css'

/**
 * Settings > Authentication (design §3.6): how users sign in, the group
 * mappings that give external users their roles at every sign-in, and a
 * preview of what a sign-in would grant.
 */
export function AuthenticationPage() {
  const { t } = useTranslation(AUTH_NS)
  const settings = useProviderQuery((p) => p.identity.settings(), [])

  return (
    <Page
      title={t('page.title')}
      actions={
        <Button variant="secondary" icon="refresh" onClick={refreshData}>
          {t('actions.refresh')}
        </Button>
      }
    >
      <PageSection title={t('providers.title')}>
        {settings.error ? (
          <Unavailable compact code={settings.errorCode} reason={settings.error} onRetry={refreshData} />
        ) : settings.data ? (
          <ProvidersSection settings={settings.data} />
        ) : (
          <div className="auth__loading">{t('page.loading')}</div>
        )}
      </PageSection>
      <PageSection title={t('mappings.title')}>
        <MappingsSection />
      </PageSection>
      <PageSection title={t('preview.title')}>
        <PreviewSection providers={settings.data?.providers ?? []} />
      </PageSection>
    </Page>
  )
}
