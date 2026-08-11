import { useState } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import type { Deployment } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { EventsTable } from '@/shared/EventsTable'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import { DetailRow, DetailSection } from './DetailBits'
import '../pages.css'

export function DeploymentDetail({ deployment: d }: { deployment: Deployment }) {
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)
  const [busy, setBusy] = useState(false)
  const [opError, setOpError] = useState<string | null>(null)

  const app = d.labels.app ?? d.labels['k8s-app'] ?? d.name
  const { data: pods } = useProviderQuery((p) => p.cluster.listPods(d.namespace), [d.namespace])
  const ownPods = (pods ?? []).filter((p) => p.labels.app === app)

  const runOp = async (op: () => Promise<void>) => {
    if (busy) return
    setBusy(true)
    setOpError(null)
    try {
      await op()
      refreshData()
    } catch (err) {
      setOpError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const scale = (replicas: number) => {
    if (replicas < 0) return
    return runOp(() => providers.cluster.scaleDeployment(d.namespace, d.name, replicas))
  }

  const restart = () => runOp(() => providers.cluster.restartDeployment(d.namespace, d.name))

  const viewYaml = async () => {
    const text = await providers.cluster.getResourceYaml('Deployment', d.namespace, d.name)
    openTab({
      id: `yaml:deployment:${d.namespace}/${d.name}`,
      title: `${d.name}.yaml`,
      icon: 'file-code',
      kind: 'yaml',
      props: { text, readOnly: true },
    })
  }

  return (
    <div className="page">
      <Toolbar title={`${d.namespace} / ${d.name}`}>
        <div className="detail__scale">
          <button
            className="kb-button kb-button--secondary"
            title="Scale down"
            onClick={() => scale(d.replicas - 1)}
            disabled={busy || d.replicas === 0}
          >
            <Icon name="remove" size={14} />
          </button>
          <span className="detail__scale-count" title="Desired replicas">
            {d.replicas}
          </span>
          <button
            className="kb-button kb-button--secondary"
            title="Scale up"
            onClick={() => scale(d.replicas + 1)}
            disabled={busy}
          >
            <Icon name="add" size={14} />
          </button>
        </div>
        <button className="kb-button kb-button--secondary" onClick={restart} disabled={busy}>
          <Icon name="sync" size={14} /> Restart
        </button>
        <button className="kb-button kb-button--secondary" onClick={viewYaml}>
          <Icon name="file-code" size={14} /> YAML
        </button>
      </Toolbar>
      <div className="detail">
        {opError && (
          <div className="detail__error">
            <Icon name="error" size={14} /> {opError}
          </div>
        )}
        <div className="detail__grid">
          <DetailRow label="Replicas" value={`${d.readyReplicas} ready / ${d.replicas} desired`} />
          <DetailRow label="Up-to-date" value={String(d.upToDate)} />
          <DetailRow label="Available" value={String(d.available)} />
          <DetailRow label="Age" value={d.age} />
        </div>

        <DetailSection title="Labels" />
        {Object.entries(d.labels).map(([k, v]) => (
          <span key={k} className="detail__chip">
            {k}={v}
          </span>
        ))}

        <DetailSection title={`Pods (${ownPods.length})`} />
        <table className="kb-table detail__env">
          <thead>
            <tr>
              <th>Name</th>
              <th>Status</th>
              <th>Ready</th>
              <th>Restarts</th>
              <th>Age</th>
              <th>Node</th>
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

        <DetailSection title="Events" />
        <EventsTable kind="Deployment" namespace={d.namespace} name={d.name} />
      </div>
    </div>
  )
}
