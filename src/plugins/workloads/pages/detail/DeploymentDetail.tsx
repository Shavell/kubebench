import { useState } from 'react'
import { usePermission } from '@/core/permissions'
import { ResourceActionButtons } from '@/core/plugin/ResourceActionButtons'
import { errorMessage } from '@/data/apiClient'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import type { Deployment } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { EventsTable } from '@/shared/EventsTable'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import { ResourceEditButtons } from '../../editing/ResourceEditButtons'
import { useWorkloadsTranslation } from '../../i18n'
import { DeploymentOpsActions } from '../../ops/DeploymentOpsActions'
import { RolloutHistory } from '../../ops/RolloutHistory'
import { WorkloadOpsDialogs } from '../../ops/WorkloadOpsDialogs'
import { DetailRow, DetailSection } from './DetailBits'
import '@/shared/tabPage.css'
import './DeploymentDetail.css'

export function DeploymentDetail({ deployment: d, onDeleted }: { deployment: Deployment; onDeleted: () => void }) {
  const { t } = useWorkloadsTranslation()
  // Scale and restart are mutations; without the permission the controls stay visible but disabled.
  const write = usePermission('write')
  const denied = write.reason ?? undefined
  const providers = useProviders()
  const [busy, setBusy] = useState(false)
  const [opError, setOpError] = useState<string | null>(null)

  const app = d.labels.app ?? d.labels['k8s-app'] ?? d.name
  const { data: pods } = useProviderQuery((p) => p.cluster.listPods(d.namespace), [d.namespace], {
    topics: ['pods'],
    namespace: d.namespace,
  })
  const ownPods = (pods ?? []).filter((p) => p.labels.app === app)

  const runOp = async (op: () => Promise<void>) => {
    if (busy) return
    setBusy(true)
    setOpError(null)
    try {
      await op()
      refreshData()
    } catch (err) {
      setOpError(errorMessage(err))
    } finally {
      setBusy(false)
    }
  }

  const scale = (replicas: number) => {
    if (replicas < 0) return
    return runOp(() => providers.cluster.scaleDeployment(d.namespace, d.name, replicas))
  }

  const restart = () => runOp(() => providers.cluster.restartDeployment(d.namespace, d.name))

  const resource = { kind: 'Deployment', apiVersion: 'apps/v1', namespace: d.namespace, name: d.name }

  return (
    <div className="page">
      <Toolbar title={`${d.namespace} / ${d.name}`}>
        <div className="detail__scale">
          <button
            className="kb-button kb-button--secondary"
            title={denied ?? t('actions.scaleDown')}
            onClick={() => scale(d.replicas - 1)}
            disabled={busy || d.replicas === 0 || !write.allowed}
          >
            <Icon name="remove" size={14} />
          </button>
          <span className="detail__scale-count" title={t('detail.desiredReplicas')}>
            {d.replicas}
          </span>
          <button
            className="kb-button kb-button--secondary"
            title={denied ?? t('actions.scaleUp')}
            onClick={() => scale(d.replicas + 1)}
            disabled={busy || !write.allowed}
          >
            <Icon name="add" size={14} />
          </button>
        </div>
        <button
          className="kb-button kb-button--secondary"
          onClick={restart}
          disabled={busy || !write.allowed}
          title={denied}
        >
          <Icon name="sync" size={14} /> {t('common.restart')}
        </button>
        <DeploymentOpsActions deployment={d} />
        <ResourceEditButtons resource={resource} onDeleted={onDeleted} />
        <ResourceActionButtons resource={resource} />
      </Toolbar>
      <WorkloadOpsDialogs />
      <div className="detail">
        {opError && (
          <div className="detail__error">
            <Icon name="error" size={14} /> {opError}
          </div>
        )}
        <div className="detail__grid">
          <DetailRow
            label={t('columns.replicas')}
            value={t('detail.replicasSummary', { ready: d.readyReplicas, desired: d.replicas })}
          />
          <DetailRow label={t('columns.upToDate')} value={String(d.upToDate)} />
          <DetailRow label={t('columns.available')} value={String(d.available)} />
          <DetailRow label={t('columns.age')} value={d.age} />
        </div>

        <DetailSection title={t('common.labels')} />
        {Object.entries(d.labels).map(([k, v]) => (
          <span key={k} className="detail__chip">
            {k}={v}
          </span>
        ))}

        <DetailSection title={t('detail.pods', { total: ownPods.length })} />
        <table className="kb-table detail__env">
          <thead>
            <tr>
              <th>{t('columns.name')}</th>
              <th>{t('columns.status')}</th>
              <th>{t('columns.ready')}</th>
              <th>{t('columns.restarts')}</th>
              <th>{t('columns.age')}</th>
              <th>{t('columns.node')}</th>
            </tr>
          </thead>
          <tbody>
            {ownPods.map((p) => (
              <tr key={p.name}>
                <td>{p.name}</td>
                <td>
                  <Badge>{p.status}</Badge>
                </td>
                <td>
                  {p.readyContainers}/{p.totalContainers}
                </td>
                <td>{p.restarts}</td>
                <td>{p.age}</td>
                <td>{p.node || '—'}</td>
              </tr>
            ))}
          </tbody>
        </table>

        <DetailSection title={t('workloadOps.rollout.history')} />
        <RolloutHistory deployment={d} />

        <DetailSection title={t('common.events')} />
        <EventsTable kind="Deployment" namespace={d.namespace} name={d.name} />
      </div>
    </div>
  )
}
