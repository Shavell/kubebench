import { useEffect, useState } from 'react'
import { useTranslation } from 'react-i18next'
import { usePermission } from '@/core/permissions'
import type { TabDescriptor } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData } from '@/data/DataContext'
import { usePluginQuery } from '@/data/pluginData'
import { Button } from '@/shared/Button'
import { ConfirmDialog } from '@/shared/ConfirmDialog'
import { Checkbox } from '@/shared/form'
import { notify } from '@/shared/notify'
import { Tabs, type TabItem } from '@/shared/Tabs'
import { Toolbar } from '@/shared/Toolbar'
import { Unavailable } from '@/shared/Unavailable'
import { ApplicationOverview } from '../components/ApplicationOverview'
import { ConditionsTable } from '../components/ConditionsTable'
import { HistoryTable } from '../components/HistoryTable'
import { OperationPanel } from '../components/OperationPanel'
import { ResourceTree } from '../components/ResourceTree'
import { SyncDialog } from '../components/SyncDialog'
import { argoCd } from '../data'
import type { ArgoHistoryEntry } from '../data/types'
import { NS } from '../i18n'
import { shortRevision } from '../text'
import '@/shared/tabPage.css'
import '../argocd.css'

type Section = 'resources' | 'operation' | 'history' | 'conditions'
type OpenDialog = { type: 'sync' } | { type: 'delete' } | { type: 'rollback'; entry: ArgoHistoryEntry } | null

/** How often an application with a running operation is re-read. */
const OPERATION_POLL_MS = 3000

export function ApplicationDetailPage({ tab }: { tab: TabDescriptor }) {
  const { t } = useTranslation(NS)
  const namespace = tab.props?.namespace as string
  const name = tab.props?.name as string
  const qualifiedName = `${namespace}/${name}`
  const closeTab = useLayoutStore((s) => s.closeTab)
  const write = usePermission('write')
  const [section, setSection] = useState<Section>('resources')
  const [dialog, setDialog] = useState<OpenDialog>(null)
  const [cascade, setCascade] = useState(true)
  const [refreshing, setRefreshing] = useState(false)
  const query = usePluginQuery(() => argoCd().getApplication(namespace, name), [namespace, name], {
    topics: ['applications.argoproj.io'],
    namespace,
  })
  const app = query.data
  const operating = app?.operating ?? false

  // Follow a running operation until the controller reports its end.
  useEffect(() => {
    if (!operating) return
    const timer = setInterval(refreshData, OPERATION_POLL_MS)
    return () => clearInterval(timer)
  }, [operating])

  if (!app) {
    if (query.error) {
      return (
        <Unavailable
          code={query.errorCode}
          reason={query.error}
          title={t('detail.unavailable', { name: qualifiedName })}
          onRetry={refreshData}
        />
      )
    }
    return <div className="page-loading">{t('detail.loading', { name: qualifiedName })}</div>
  }

  const denied = write.reason ?? undefined
  const syncBlocked = denied ?? (app.operating ? t('detail.operationRunning') : undefined)

  const refresh = async (hard: boolean) => {
    setRefreshing(true)
    try {
      await argoCd().refreshApplication(namespace, name, hard)
      notify.success(t(hard ? 'refresh.hardDone' : 'refresh.done', { name }))
    } catch (err) {
      notify.error(err)
    }
    setRefreshing(false)
    refreshData()
  }

  // Failures reject into ConfirmDialog, which stays open and shows the reason.
  const rollback = async (entry: ArgoHistoryEntry) => {
    await argoCd().rollbackApplication(namespace, name, entry.id)
    notify.success(t('rollback.started', { name, id: entry.id }))
    refreshData()
  }

  const remove = async () => {
    await argoCd().deleteApplication(namespace, name, cascade)
    notify.success(t(cascade ? 'delete.doneCascade' : 'delete.done', { name }))
    refreshData()
    closeTab(tab.id)
  }

  const sections: TabItem<Section>[] = [
    { id: 'resources', label: t('detail.tabs.resources'), icon: 'list-tree', badge: app.resources.length },
    { id: 'operation', label: t('detail.tabs.operation'), icon: 'sync' },
    { id: 'history', label: t('detail.tabs.history'), icon: 'history', badge: app.history.length },
    { id: 'conditions', label: t('detail.tabs.conditions'), icon: 'warning', badge: app.conditions.length },
  ]

  return (
    <div className="page">
      <Toolbar title={qualifiedName}>
        <Button icon="sync" disabled={syncBlocked !== undefined} title={syncBlocked} onClick={() => setDialog({ type: 'sync' })}>
          {t('actions.sync')}
        </Button>
        <Button variant="secondary" icon="refresh" busy={refreshing} disabled={!write.allowed} title={denied ?? t('refresh.hint')} onClick={() => refresh(false)}>
          {t('actions.refresh')}
        </Button>
        <Button variant="secondary" icon="repo-sync" disabled={!write.allowed || refreshing} title={denied ?? t('refresh.hardHint')} onClick={() => refresh(true)}>
          {t('actions.hardRefresh')}
        </Button>
        <Button variant="danger" icon="trash" disabled={!write.allowed} title={denied} onClick={() => {
            setCascade(true)
            setDialog({ type: 'delete' })
          }}>
          {t('actions.delete')}
        </Button>
      </Toolbar>
      <div className="detail argocd-detail">
        <ApplicationOverview app={app} />
        <Tabs tabs={sections} active={section} onChange={setSection} ariaLabel={qualifiedName} className="argocd-sections">
          {section === 'resources' && <ResourceTree appName={app.name} resources={app.resources} />}
          {section === 'operation' && <OperationPanel operation={app.operation} />}
          {section === 'history' && <HistoryTable app={app} onRollback={(entry) => setDialog({ type: 'rollback', entry })} />}
          {section === 'conditions' && <ConditionsTable conditions={app.conditions} />}
        </Tabs>
      </div>

      {dialog?.type === 'sync' && <SyncDialog app={app} onClose={() => setDialog(null)} />}
      {dialog?.type === 'rollback' && (
        <ConfirmDialog
          open
          title={t('rollback.title', { name, id: dialog.entry.id })}
          message={t('rollback.message', {
            revision: shortRevision(dialog.entry.revision || dialog.entry.revisions.join(', ')),
          })}
          confirmLabel={t('detail.history.rollback')}
          confirmPhrase={qualifiedName}
          onConfirm={() => rollback(dialog.entry)}
          onClose={() => setDialog(null)}
        />
      )}
      {dialog?.type === 'delete' && (
        <ConfirmDialog
          open
          danger
          requirePhrase
          title={t('delete.title', { name })}
          message={t('delete.message')}
          confirmLabel={t('actions.delete')}
          confirmPhrase={qualifiedName}
          onConfirm={remove}
          onClose={() => setDialog(null)}
        >
          <Checkbox
            checked={cascade}
            onChange={setCascade}
            label={t('delete.cascade')}
            description={t(cascade ? 'delete.cascadeOn' : 'delete.cascadeOff', { namespace: app.destination.namespace })}
          />
        </ConfirmDialog>
      )}
    </div>
  )
}
