import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData, useProviderQuery, useProviders } from '@/data/DataContext'
import type { Pod } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { ConfirmButton } from '@/shared/ConfirmButton'
import { EventsTable } from '@/shared/EventsTable'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import { TimeSeriesChart } from '@/shared/charts/TimeSeriesChart'
import { chartColor } from '@/shared/charts/chartTheme'
import { DetailRow, DetailSection } from './DetailBits'
import '../pages.css'

export function PodDetail({ pod, tabId }: { pod: Pod; tabId: string }) {
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)
  const closeTab = useLayoutStore((s) => s.closeTab)
  const container = pod.containers[0]

  const { data: cpu } = useProviderQuery(
    (p) => p.metrics.podSeries(pod.namespace, pod.name, 'cpu', 60),
    [pod.namespace, pod.name],
  )
  const { data: memory } = useProviderQuery(
    (p) => p.metrics.podSeries(pod.namespace, pod.name, 'memory', 60),
    [pod.namespace, pod.name],
  )

  const openLogs = () =>
    openTab({
      id: `logs:${pod.namespace}/${pod.name}`,
      title: `${pod.name} · logs`,
      icon: 'output',
      kind: 'pod-logs',
      props: { namespace: pod.namespace, pod: pod.name, container: container.name },
    })

  const openShell = () =>
    openTab({
      id: `exec:${pod.namespace}/${pod.name}`,
      title: `${pod.name} · sh`,
      icon: 'terminal',
      kind: 'pod-exec',
      props: {
        namespace: pod.namespace,
        pod: pod.name,
        container: container.name,
        env: container.env,
      },
    })

  const viewYaml = async () => {
    const text = await providers.cluster.getResourceYaml('Pod', pod.namespace, pod.name)
    openTab({
      id: `yaml:pod:${pod.namespace}/${pod.name}`,
      title: `${pod.name}.yaml`,
      icon: 'file-code',
      kind: 'yaml',
      props: { text, readOnly: true },
    })
  }

  const killPod = async () => {
    await providers.cluster.deletePod(pod.namespace, pod.name)
    refreshData()
    closeTab(tabId)
  }

  return (
    <div className="page">
      <Toolbar title={`${pod.namespace} / ${pod.name}`}>
        <button className="kb-button kb-button--secondary" onClick={openLogs}>
          <Icon name="output" size={14} /> Logs
        </button>
        <button className="kb-button kb-button--secondary" onClick={openShell}>
          <Icon name="terminal" size={14} /> Shell
        </button>
        <button className="kb-button kb-button--secondary" onClick={viewYaml}>
          <Icon name="file-code" size={14} /> YAML
        </button>
        <ConfirmButton icon="trash" label="Delete" confirmLabel="Confirm delete?" onConfirm={killPod} />
      </Toolbar>
      <div className="detail">
        <div className="detail__grid">
          <DetailRow label="Status" value={<Badge>{pod.status}</Badge>} />
          <DetailRow label="Ready" value={`${pod.readyContainers}/${pod.totalContainers}`} />
          <DetailRow label="Restarts" value={String(pod.restarts)} />
          <DetailRow label="Controlled by" value={pod.controlledBy ?? '—'} />
          <DetailRow label="Node" value={pod.node || '—'} />
          <DetailRow label="Pod IP" value={pod.podIP || '—'} />
          <DetailRow label="Age" value={pod.age} />
        </div>

        <DetailSection title="Utilization · last 60m" />
        <div className="detail__charts">
          <TimeSeriesChart title="CPU" data={cpu ?? []} color={chartColor('blue')} height={150} />
          <TimeSeriesChart title="Memory" data={memory ?? []} color={chartColor('purple')} height={150} />
        </div>

        <DetailSection title="Labels" />
        {Object.entries(pod.labels).map(([k, v]) => (
          <span key={k} className="detail__chip">
            {k}={v}
          </span>
        ))}

        {pod.containers.map((c) => (
          <div key={c.name}>
            <DetailSection title={`Container · ${c.name}`} />
            <div className="detail__grid">
              <DetailRow label="Image" value={c.image} />
              <DetailRow
                label="Command"
                value={c.command ? <code className="detail__code">{c.command.join(' ')}</code> : '—'}
              />
              <DetailRow
                label="Args"
                value={c.args ? <code className="detail__code">{c.args.join(' ')}</code> : '—'}
              />
              <DetailRow label="Ports" value={c.ports ?? '—'} />
              <DetailRow label="Requests" value={`cpu ${c.requests.cpu} · memory ${c.requests.memory}`} />
              <DetailRow label="Limits" value={`cpu ${c.limits.cpu} · memory ${c.limits.memory}`} />
            </div>
            <div className="detail__subheading">Environment</div>
            <table className="kb-table detail__env">
              <thead>
                <tr>
                  <th>Name</th>
                  <th>Value</th>
                </tr>
              </thead>
              <tbody>
                {c.env.map((e) => (
                  <tr key={e.name}>
                    <td>
                      <code className="detail__code">{e.name}</code>
                    </td>
                    <td>{e.value}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ))}

        <DetailSection title="Events" />
        <EventsTable kind="Pod" namespace={pod.namespace} name={pod.name} />
      </div>
    </div>
  )
}
