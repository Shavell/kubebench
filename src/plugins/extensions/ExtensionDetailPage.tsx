import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { describeLoadError, usePluginStatusStore } from '@/core/plugin/pluginStatusStore'
import type { TabDescriptor } from '@/core/plugin/types'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Page, PageSection } from '@/shared/Page'
import { Unavailable } from '@/shared/Unavailable'
import { useExtensions, type Extension } from './catalog'
import { ExtensionBadges } from './ExtensionBadges'
import { ExtensionContributions } from './ExtensionContributions'
import { EXTENSIONS_NS } from './namespace'
import { usePluginToggle } from './usePluginToggle'
import './extensions.css'

/** Enable/disable for admins; core plugins explain why they cannot be turned off. */
function ToggleAction({ extension }: { extension: Extension }) {
  const { t } = useTranslation(EXTENSIONS_NS)
  const admin = usePermission('admin')
  const toggle = usePluginToggle()
  const [confirming, setConfirming] = useState(false)
  const [enabling, setEnabling] = useState(false)

  if (extension.core) return <div className="ext-detail__hint">{t('detail.coreHint')}</div>

  const enable = async () => {
    setEnabling(true)
    try {
      await toggle(extension, true)
    } catch (err) {
      notify.error(err)
    } finally {
      setEnabling(false)
    }
  }

  return (
    <>
      {extension.status.enabled ? (
        <Button
          variant="secondary"
          icon="circle-slash"
          disabled={!admin.allowed}
          title={admin.reason ?? undefined}
          onClick={() => setConfirming(true)}
        >
          {t('detail.disable')}
        </Button>
      ) : (
        <Button
          icon="check"
          busy={enabling}
          disabled={!admin.allowed}
          title={admin.reason ?? undefined}
          onClick={() => void enable()}
        >
          {t('detail.enable')}
        </Button>
      )}
      {!admin.allowed && <div className="ext-detail__hint">{t('detail.adminOnly')}</div>}
      <ConfirmDialog
        open={confirming}
        danger
        title={t('detail.disableConfirm.title', { name: extension.name })}
        message={t('detail.disableConfirm.message', { name: extension.name })}
        confirmLabel={t('detail.disable')}
        onConfirm={() => toggle(extension, false)}
        onClose={() => setConfirming(false)}
      />
    </>
  )
}

function StatusFacts({ extension }: { extension: Extension }) {
  const { t } = useTranslation(EXTENSIONS_NS)
  const backendId = extension.manifest?.backendPlugin
  const backend = usePluginStatusStore((s) => (backendId ? s.serverPlugins[backendId] : undefined))
  const { status, server, loadError } = extension

  return (
    <dl className="ext-detail__facts">
      <dt>{t('detail.state')}</dt>
      <dd>
        {status.enabled ? (
          <Badge tone="success">{t('status.enabled')}</Badge>
        ) : (
          <Badge tone="neutral">{t('status.disabled')}</Badge>
        )}
      </dd>
      <dt>{t('detail.availability')}</dt>
      <dd>
        {status.available ? (
          <Badge tone="success">{t('status.available')}</Badge>
        ) : (
          <>
            <Badge tone="warning">{t('status.unavailable')}</Badge>
            {status.reason && <span className="ext-detail__reason">{status.reason}</span>}
          </>
        )}
      </dd>
      {extension.conflict && (
        <>
          <dt>{t('detail.conflict')}</dt>
          <dd className="ext-detail__reason">{t('detail.conflictHint', { entry: extension.conflict.entry })}</dd>
        </>
      )}
      {backendId && (
        <>
          <dt>{t('detail.requires')}</dt>
          <dd>
            <code className="ext-detail__code">{backendId}</code>
            {!backend && <span className="ext-detail__reason">{t('detail.backendMissing')}</span>}
          </dd>
        </>
      )}
      {extension.source === 'external' && (
        <>
          <dt>{t('detail.script')}</dt>
          <dd>
            {server?.entry ? <code className="ext-detail__code">{server.entry}</code> : t('detail.noScript')}
          </dd>
          <dt>{t('detail.loaded')}</dt>
          <dd>{extension.manifest ? t('detail.loadedYes') : t('detail.loadedNo')}</dd>
          {loadError && (
            <>
              <dt>{t('detail.loadError')}</dt>
              <dd className="ext-detail__reason">{describeLoadError(loadError)}</dd>
            </>
          )}
        </>
      )}
    </dl>
  )
}

/** Details of one plugin: what it is, whether it is active and why not, what it contributes. */
export function ExtensionDetailPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(EXTENSIONS_NS)
  const pluginId = String(tab.props?.pluginId ?? '')
  const extension = useExtensions().find((e) => e.id === pluginId)

  if (!extension) {
    return <Unavailable code="not_found" reason={t('detail.unknown', { id: pluginId })} />
  }
  return (
    <Page title={extension.name}>
      <header className="ext-detail__header">
        <Icon name={extension.icon} size={44} className="ext-detail__icon" />
        <div className="ext-detail__heading">
          <h1 className="ext-detail__name">{extension.name}</h1>
          <div className="ext-detail__meta">
            <span className="ext-detail__id">{extension.id}</span>
            {extension.version && <span>{t('detail.version', { version: extension.version })}</span>}
            <span>{t(`categories.${extension.category}`, { defaultValue: extension.category })}</span>
            <span>{t(`sources.${extension.source}`)}</span>
            {extension.core && <Badge tone="info">{t('status.core')}</Badge>}
            <ExtensionBadges extension={extension} />
          </div>
          {extension.description && <p className="ext-detail__description">{extension.description}</p>}
        </div>
        <div className="ext-detail__actions">
          <ToggleAction extension={extension} />
        </div>
      </header>
      <PageSection title={t('detail.statusSection')}>
        <StatusFacts extension={extension} />
      </PageSection>
      <PageSection title={t('detail.contributesSection')}>
        <ExtensionContributions extension={extension} />
      </PageSection>
    </Page>
  )
}
