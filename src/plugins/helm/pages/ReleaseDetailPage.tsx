import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import type { TabDescriptor } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { isApiError } from '@/data/apiClient'
import { refreshData } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { CORE_NS } from '@/i18n'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { EmptyState } from '@/shared/EmptyState'
import { notify } from '@/shared/notify'
import { Tabs } from '@/shared/Tabs'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { helmData } from '../data'
import { NS } from '../i18n'
import { openUpgradeTab } from '../tabs'
import { HistoryTab } from './release/HistoryTab'
import { ManifestTab } from './release/ManifestTab'
import { OverviewTab } from './release/OverviewTab'
import { UninstallDialog } from './release/UninstallDialog'
import { ValuesTab } from './release/ValuesTab'
import '@/shared/tabPage.css'
import '../helm.css'

type DetailTab = 'overview' | 'values' | 'manifest' | 'notes' | 'history'

/** The confirmation that is open: uninstall, or a rollback to a revision. */
type Confirmation = { action: 'uninstall' } | { action: 'rollback'; revision: number }

/** One release (design §7.4): overview, values, manifest, notes and history with rollback. */
export function ReleaseDetailPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const namespace = tab.props?.namespace as string
  const name = tab.props?.name as string
  const qualifiedName = `${namespace}/${name}`
  const closeTab = useLayoutStore((s) => s.closeTab)
  // Mutations stay visible but disabled without write permission, with the reason as tooltip.
  const write = usePermission('write')
  const [view, setView] = useState<DetailTab>('overview')
  const [confirming, setConfirming] = useState<Confirmation | null>(null)
  const query = usePluginQuery(() => helmData().getRelease(namespace, name), [namespace, name])
  const release = query.data

  if (!release) {
    if (query.errorCode === 'not_found') {
      return <EmptyState icon="trash" title={t('release.gone', { name: qualifiedName })} hint={t('release.goneHint')} />
    }
    if (query.error) {
      return <Unavailable code={query.errorCode} reasonCode={query.errorReason} reason={query.error} title={t('release.unavailable')} onRetry={refreshData} />
    }
    return <div className="page-loading">{t('release.loading', { name: qualifiedName })}</div>
  }

  // Failures reject into ConfirmDialog, which keeps itself open and shows the reason.
  const rollback = async (revision: number) => {
    const result = await helmData().rollback(namespace, name, revision)
    notify.success(t('release.rollbackDialog.done', { name, revision, newRevision: result.revision }))
    refreshData()
  }

  const uninstall = async (keepHistory: boolean) => {
    try {
      await helmData().uninstall(namespace, name, keepHistory)
    } catch (err) {
      // Already gone: nothing left to do but close the tab.
      if (!isApiError(err, 'not_found')) throw err
    }
    notify.success(t(keepHistory ? 'release.uninstallDialog.doneKept' : 'release.uninstallDialog.done', { name }))
    refreshData()
    if (!keepHistory) closeTab(tab.id)
  }

  const uninstalled = release.status === 'uninstalled'
  const tabs = [
    { id: 'overview' as const, label: t('release.tabs.overview'), icon: 'info' },
    { id: 'values' as const, label: t('release.tabs.values'), icon: 'settings' },
    { id: 'manifest' as const, label: t('release.tabs.manifest'), icon: 'file-code' },
    { id: 'notes' as const, label: t('release.tabs.notes'), icon: 'note' },
    { id: 'history' as const, label: t('release.tabs.history'), icon: 'history', badge: release.history.length },
  ]

  return (
    <div className="page">
      <Toolbar title={`${release.namespace} / ${release.name}`}>
        <Button
          variant="secondary"
          icon="refresh"
          onClick={refreshData}
          title={t('common.refresh', { ns: CORE_NS })}
          aria-label={t('common.refresh', { ns: CORE_NS })}
        />
        <Button
          variant="secondary"
          icon="arrow-circle-up"
          disabled={!write.allowed || uninstalled}
          title={write.reason ?? (uninstalled ? t('release.upgradeUninstalled') : undefined)}
          onClick={() => openUpgradeTab(namespace, name)}
        >
          {t('release.upgrade')}
        </Button>
        <Button
          variant="danger"
          icon="trash"
          disabled={!write.allowed}
          title={write.reason ?? undefined}
          onClick={() => setConfirming({ action: 'uninstall' })}
        >
          {t('release.uninstall')}
        </Button>
      </Toolbar>
      {confirming?.action === 'uninstall' && (
        <UninstallDialog namespace={namespace} name={name} onConfirm={uninstall} onClose={() => setConfirming(null)} />
      )}
      {confirming?.action === 'rollback' && (
        <ConfirmDialog
          open
          title={t('release.rollbackDialog.title', { name, revision: confirming.revision })}
          message={t('release.rollbackDialog.message', { revision: confirming.revision })}
          confirmLabel={t('release.history.rollback')}
          confirmPhrase={qualifiedName}
          onConfirm={() => rollback(confirming.revision)}
          onClose={() => setConfirming(null)}
        />
      )}
      <div className="helm-detail">
        <Tabs tabs={tabs} active={view} onChange={setView} ariaLabel={qualifiedName}>
          {view === 'overview' && <OverviewTab release={release} />}
          {view === 'values' && <ValuesTab release={release} />}
          {view === 'manifest' && <ManifestTab release={release} />}
          {view === 'notes' &&
            (release.notes ? (
              <pre className="helm-notes">{release.notes}</pre>
            ) : (
              <EmptyState icon="note" title={t('release.notes.none')} />
            ))}
          {view === 'history' && (
            <HistoryTab release={release} onRollback={(revision) => setConfirming({ action: 'rollback', revision })} />
          )}
        </Tabs>
      </div>
    </div>
  )
}
