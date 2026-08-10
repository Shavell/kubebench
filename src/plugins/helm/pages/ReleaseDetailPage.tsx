import { useLayoutStore } from '@/core/store/layoutStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { TabDescriptor } from '@/core/plugin/types'
import { Badge } from '@/shared/Badge'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import '@/plugins/workloads/pages/pages.css'

export function ReleaseDetailPage({ tab }: { tab: TabDescriptor }) {
  const name = tab.props?.name as string
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)
  const { data: releases } = useProviderQuery((p) => p.helm.listReleases(), [])

  const release = releases?.find((r) => r.name === name)
  if (!release) return <div className="page-loading">Loading {name}…</div>

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

  return (
    <div className="page">
      <Toolbar title={`${release.namespace} / ${release.name}`}>
        <button className="kb-button" onClick={openValues}>
          <Icon name="settings" size={14} /> Edit Values
        </button>
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
        <table className="kb-table" style={{ maxWidth: 720 }}>
          <thead>
            <tr>
              <th>Revision</th>
              <th>Status</th>
              <th>Updated</th>
              <th>Description</th>
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
              </tr>
            ))}
          </tbody>
        </table>
      </div>
    </div>
  )
}
