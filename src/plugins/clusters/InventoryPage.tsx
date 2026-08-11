import { useState } from 'react'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import { dataMode } from '@/data/mode'
import type { ClusterEntry } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { ConfirmButton } from '@/shared/ConfirmButton'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'
import './inventory.css'

export function InventoryPage() {
  const providers = useProviders()
  const { data: clusters, loading } = useProviderQuery((p) => p.inventory.listClusters(), [])
  const [name, setName] = useState('')
  const [path, setPath] = useState('')
  const [address, setAddress] = useState('')
  const [error, setError] = useState<string | null>(null)
  const [busy, setBusy] = useState(false)

  const run = async (op: () => Promise<void>, reloadAfter = false) => {
    if (busy) return
    setBusy(true)
    setError(null)
    try {
      await op()
      refreshData()
      if (reloadAfter) window.location.reload()
    } catch (err) {
      setError(err instanceof Error ? err.message : String(err))
    } finally {
      setBusy(false)
    }
  }

  const addCluster = () =>
    run(async () => {
      await providers.inventory.addCluster({
        name,
        kubeconfigPath: path || undefined,
        address: address || undefined,
      })
      setName('')
      setPath('')
      setAddress('')
    })

  return (
    <div className="page">
      <Toolbar title="Cluster Inventory" />
      <div className="detail">
        {error && (
          <div className="detail__error">
            <Icon name="error" size={14} /> {error}
          </div>
        )}

        <table className="kb-table detail__env">
          <thead>
            <tr>
              <th>Name</th>
              <th>Source</th>
              <th>Server</th>
              <th>Status</th>
              <th></th>
            </tr>
          </thead>
          <tbody>
            {(clusters ?? []).map((c: ClusterEntry) => (
              <tr key={c.name}>
                <td>
                  <Icon name="vm" size={14} style={{ marginRight: 6, verticalAlign: 'middle' }} />
                  {c.name}
                </td>
                <td>
                  <code className="detail__code">{c.kubeconfigPath ?? c.address ?? '—'}</code>
                </td>
                <td className="inventory__server">{c.server ?? '—'}</td>
                <td>
                  {c.error ? (
                    <Badge tone="error">Error</Badge>
                  ) : c.active ? (
                    <Badge tone="success">Active</Badge>
                  ) : (
                    <Badge tone="neutral">Registered</Badge>
                  )}
                </td>
                <td>
                  <div className="inventory__actions">
                    {!c.active && c.kubeconfigPath && (
                      <button
                        className="kb-button"
                        disabled={busy}
                        title="Switch to this cluster (reloads the app)"
                        onClick={() => run(() => providers.inventory.activateCluster(c.name), true)}
                      >
                        <Icon name="plug" size={13} /> Activate
                      </button>
                    )}
                    <ConfirmButton
                      icon="trash"
                      label="Remove"
                      confirmLabel="Confirm?"
                      onConfirm={() => run(() => providers.inventory.removeCluster(c.name))}
                    />
                  </div>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
        {loading && <div className="page-loading">Loading inventory…</div>}

        <div className="detail__section">Add Cluster</div>
        <div className="inventory__form">
          <label>
            Name
            <input
              value={name}
              onChange={(e) => setName(e.target.value)}
              placeholder="prod-eu-west-1"
              spellCheck={false}
            />
          </label>
          <label>
            Kubeconfig path
            <input
              value={path}
              onChange={(e) => setPath(e.target.value)}
              placeholder="/Users/you/.kube/prod-config"
              spellCheck={false}
            />
          </label>
          <label>
            API address <span className="inventory__hint">(alternative to kubeconfig)</span>
            <input
              value={address}
              onChange={(e) => setAddress(e.target.value)}
              placeholder="https://cluster.example.com:6443"
              spellCheck={false}
            />
          </label>
          <button
            className="kb-button"
            disabled={busy || !name || (!path && !address)}
            onClick={addCluster}
          >
            <Icon name="add" size={14} /> Add Cluster
          </button>
        </div>

        <div className="inventory__note">
          <Icon name="info" size={13} />
          {dataMode.mode === 'live'
            ? 'Inventory is stored by the proxy in ~/.kubebench/clusters.json (paths only, never credentials). Address-only entries are bookmarks and cannot be activated.'
            : 'Mock mode: inventory is stored in the browser (localStorage). Start the proxy (npm run server) to manage real clusters.'}
        </div>
      </div>
    </div>
  )
}
