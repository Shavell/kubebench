import { useState } from 'react'
import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { Badge } from '@/shared/Badge'
import { ConfirmButton } from '@/shared/ConfirmButton'
import { EmptyState } from '@/shared/EmptyState'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'

export function ReleaseDetailPage({ tab }: { tab: TabDescriptor }) {
  const name = tab.props?.name as string
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)
  const closeTab = useLayoutStore((s) => s.closeTab)
  const [busy, setBusy] = useState(false)
  const { data: releases, loading } = useProviderQuery((p) => p.helm.listReleases(), [])

  const release = releases?.find((r) => r.name === name)
  if (!release) {
    if (loading || !releases) return <div className="page-loading">Loading {name}…</div>
    return (
      <EmptyState
        icon="trash"
        title={`Release ${name} no longer exists`}
        hint="It may have been uninstalled"
      />
    )
  }

  const openValues = async () => {
    const text = await providers.helm.getReleaseValuesYaml(name)
    openTab({
      id: `helm-values:${name}`,
      title: `${name} · values.yaml`,
      icon: 'settings',
      kind: 'yaml',
      props: { text, readOnly: false },
    })
  }

  const rollback = async (revision: number) => {
    if (busy) return
    setBusy(true)
    await providers.helm.rollbackRelease(name, revision)
    refreshData()
    setBusy(false)
  }

  const uninstall = async () => {
    await providers.helm.uninstallRelease(name)
    refreshData()
    closeTab(tab.id)
  }

  return (
    <div className="page">
      <Toolbar title={`${release.namespace} / ${release.name}`}>
        <button className="kb-button kb-button--secondary" onClick={openValues}>
          <Icon name="settings" size={14} /> Edit Values
        </button>
        <ConfirmButton
          icon="trash"
          label="Uninstall"
          confirmLabel="Confirm uninstall?"
          onConfirm={uninstall}
        />
      </Toolbar>
      <div className="detail">
        <div className="detail__grid">
          <span className="detail__label">Status</span>
          <span className="detail__value">
            <Badge>{release.status}</Badge>
          </span>
          <span className="detail__label">Chart</span>
          <span className="detail__value">{`${release.chart}-${release.chartVersion}`}</span>
          <span className="detail__label">App version</span>
          <span className="detail__value">{release.appVersion}</span>
          <span className="detail__label">Revision</span>
          <span className="detail__value">{String(release.revision)}</span>
          <span className="detail__label">Last updated</span>
          <span className="detail__value">{release.updated}</span>
        </div>
        <div className="detail__section">Revision History</div>
        <table className="kb-table" style={{ maxWidth: 900 }}>
          <thead>
            <tr>
              <th>Revision</th>
              <th>Status</th>
              <th>Updated</th>
              <th>Description</th>
              <th></th>
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
                      disabled={busy}
                      onClick={() => rollback(h.revision)}
                      title={`helm rollback ${name} ${h.revision}`}
                    >
                      <Icon name="history" size={13} /> Rollback
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
