import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { CORE_NS } from '@/i18n'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { EmptyState } from '@/shared/EmptyState'
import { Icon } from '@/shared/Icon'
import { notify } from '@/shared/notify'
import { Toolbar } from '@/shared/Toolbar'
import { NS } from '../i18n'
import '@/shared/tabPage.css'

/** The confirmation that is open: uninstall, or a rollback to a revision. */
type Confirmation = { action: 'uninstall' } | { action: 'rollback'; revision: number }

export function ReleaseDetailPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const name = tab.props?.name as string
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)
  const closeTab = useLayoutStore((s) => s.closeTab)
  // Rollback and uninstall mutate the cluster: without write permission they
  // stay visible but disabled, with the reason as tooltip; values open read-only.
  const write = usePermission('write')
  const [confirming, setConfirming] = useState<Confirmation | null>(null)
  const { data: releases, loading, error } = useProviderQuery((p) => p.helm.listReleases(), [])

  const release = releases?.find((r) => r.name === name)
  if (!release) {
    if (error) return <EmptyState icon="warning" title={t('releases.unavailable')} hint={error} />
    if (loading || !releases) return <div className="page-loading">{t('release.loading', { name })}</div>
    return <EmptyState icon="trash" title={t('release.gone', { name })} hint={t('release.goneHint')} />
  }

  const deniedReason = write.reason ?? undefined

  const openValues = async () => {
    let text: string
    try {
      text = await providers.helm.getReleaseValuesYaml(name, release.namespace)
    } catch (err) {
      notify.error(err)
      return
    }
    openTab({
      id: `helm-values:${name}`,
      title: `${name} · values.yaml`,
      icon: 'settings',
      kind: 'yaml',
      props: { text, readOnly: !write.allowed },
    })
  }

  // Failures reject into ConfirmDialog, which keeps itself open and shows the reason.
  const rollback = async (revision: number) => {
    await providers.helm.rollbackRelease(name, release.namespace, revision)
    notify.success(t('release.rollbackDialog.done', { name, revision }))
    refreshData()
  }

  const uninstall = async () => {
    await providers.helm.uninstallRelease(name, release.namespace)
    notify.success(t('release.uninstallDialog.done', { name }))
    refreshData()
    closeTab(tab.id)
  }

  return (
    <div className="page">
      <Toolbar title={`${release.namespace} / ${release.name}`}>
        <button className="kb-button kb-button--secondary" onClick={openValues}>
          <Icon name="settings" size={14} />{' '}
          {write.allowed ? t('release.editValues') : t('release.viewValues')}
        </button>
        <Button
          variant="danger"
          icon="trash"
          onClick={() => setConfirming({ action: 'uninstall' })}
          disabled={!write.allowed}
          title={deniedReason}
        >
          {t('release.uninstall')}
        </Button>
      </Toolbar>
      {confirming?.action === 'uninstall' && (
        <ConfirmDialog
          open
          danger
          title={t('release.uninstallDialog.title', { name })}
          message={t('release.uninstallDialog.message', { namespace: release.namespace })}
          confirmLabel={t('release.uninstall')}
          confirmPhrase={name}
          onConfirm={uninstall}
          onClose={() => setConfirming(null)}
        />
      )}
      {confirming?.action === 'rollback' && (
        <ConfirmDialog
          open
          title={t('release.rollbackDialog.title', { name, revision: confirming.revision })}
          message={t('release.rollbackDialog.message', { revision: confirming.revision })}
          confirmLabel={t('release.history.rollback')}
          confirmPhrase={name}
          onConfirm={() => rollback(confirming.revision)}
          onClose={() => setConfirming(null)}
        />
      )}
      <div className="detail">
        <div className="detail__grid">
          <span className="detail__label">{t('common.status', { ns: CORE_NS })}</span>
          <span className="detail__value">
            <Badge>{release.status}</Badge>
          </span>
          <span className="detail__label">{t('release.fields.chart')}</span>
          <span className="detail__value">{`${release.chart}-${release.chartVersion}`}</span>
          <span className="detail__label">{t('release.fields.appVersion')}</span>
          <span className="detail__value">{release.appVersion}</span>
          <span className="detail__label">{t('release.fields.revision')}</span>
          <span className="detail__value">{String(release.revision)}</span>
          <span className="detail__label">{t('release.fields.updated')}</span>
          <span className="detail__value">{release.updated}</span>
        </div>
        <div className="detail__section">{t('release.history.title')}</div>
        <table className="kb-table" style={{ maxWidth: 900 }}>
          <thead>
            <tr>
              <th>{t('release.history.revision')}</th>
              <th>{t('common.status', { ns: CORE_NS })}</th>
              <th>{t('release.history.updated')}</th>
              <th>{t('release.history.description')}</th>
              <th aria-label={t('common.actions', { ns: CORE_NS })}></th>
            </tr>
          </thead>
          <tbody>
            {release.history.map((h) => (
              <tr key={h.revision}>
                <td>{h.revision}</td>
                <td>
                  <Badge>{h.status}</Badge>
                </td>
                <td>{h.updated}</td>
                <td>{h.description}</td>
                <td>
                  {h.revision !== release.revision && (
                    <button
                      className="kb-button kb-button--secondary"
                      disabled={!write.allowed}
                      onClick={() => setConfirming({ action: 'rollback', revision: h.revision })}
                      title={write.allowed ? `helm rollback ${name} ${h.revision}` : deniedReason}
                    >
                      <Icon name="history" size={13} /> {t('release.history.rollback')}
                    </button>
                  )}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
