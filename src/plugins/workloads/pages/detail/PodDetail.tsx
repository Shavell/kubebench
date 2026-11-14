import { useState } from 'react'
import { usePermission } from '@/core/permissions'
import { ResourceActionButtons } from '@/core/plugin/ResourceActionButtons'
import { useLayoutStore } from '@/core/store/layoutStore'
import { useSettingsStore } from '@/core/store/settingsStore'
import { useProviderQuery, useProviders } from '@/data/DataContext'
import type { Pod } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { EventsTable } from '@/shared/EventsTable'
import { notify } from '@/shared/notify'
import { Icon } from '@/shared/Icon'
import { Toolbar } from '@/shared/Toolbar'
import { TimeSeriesChart } from '@/shared/charts/TimeSeriesChart'
import { chartColor } from '@/shared/charts/chartTheme'
import { DeletePodDialog } from '../../DeletePodDialog'
import { useWorkloadsTranslation, WORKLOADS_NS } from '../../i18n'
import { DetailRow, DetailSection } from './DetailBits'
import '@/shared/tabPage.css'

export function PodDetail({ pod, tabId }: { pod: Pod; tabId: string }) {
  const { t } = useWorkloadsTranslation()
  const exec = usePermission('exec')
  const write = usePermission('write')
  const providers = useProviders()
  const openTab = useLayoutStore((s) => s.openTab)
  const closeTab = useLayoutStore((s) => s.closeTab)
  const [deleting, setDeleting] = useState(false)
  const windowMinutes = useSettingsStore((s) => s.metricsWindow)
  const container = pod.containers[0]

  const { data: cpu } = useProviderQuery(
    (p) => p.metrics.podSeries(pod.namespace, pod.name, 'cpu', windowMinutes),
    [pod.namespace, pod.name, windowMinutes],
  )
  const { data: memory } = useProviderQuery(
    (p) => p.metrics.podSeries(pod.namespace, pod.name, 'memory', windowMinutes),
    [pod.namespace, pod.name, windowMinutes],
  )

  const openLogs = () =>
    openTab({
      id: `logs:${pod.namespace}/${pod.name}`,
      title: t('detail.logsTabTitle', { name: pod.name }),
      titleKey: 'detail.logsTabTitle',
      titleNs: WORKLOADS_NS,
      titleValues: { name: pod.name },
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
    let text: string
    try {
      text = await providers.cluster.getResourceYaml('Pod', pod.namespace, pod.name)
    } catch (err) {
      notify.error(err)
      return
    }
    openTab({
      id: `yaml:pod:${pod.namespace}/${pod.name}`,
      title: `${pod.name}.yaml`,
      icon: 'file-code',
      kind: 'yaml',
      props: { text, readOnly: true },
    })
  }

  return (
    <div className="page">
      <Toolbar title={`${pod.namespace} / ${pod.name}`}>
        <button className="kb-button kb-button--secondary" onClick={openLogs}>
          <Icon name="output" size={14} /> {t('common.logs')}
        </button>
        <button
          className="kb-button kb-button--secondary"
          onClick={openShell}
          disabled={!exec.allowed}
          title={exec.reason ?? undefined}
        >
          <Icon name="terminal" size={14} /> {t('actions.shell')}
        </button>
        <button className="kb-button kb-button--secondary" onClick={viewYaml}>
          <Icon name="file-code" size={14} /> {t('actions.yaml')}
        </button>
        <Button
          variant="danger"
          icon="trash"
          onClick={() => setDeleting(true)}
          disabled={!write.allowed}
          title={write.reason ?? undefined}
        >
          {t('common.delete')}
        </Button>
        <ResourceActionButtons resource={{ kind: 'Pod', apiVersion: 'v1', namespace: pod.namespace, name: pod.name }} />
      </Toolbar>
      <DeletePodDialog
        pod={deleting ? pod : null}
        onClose={() => setDeleting(false)}
        onDeleted={() => closeTab(tabId)}
      />
      <div className="detail">
        <div className="detail__grid">
          <DetailRow label={t('columns.status')} value={<Badge>{pod.status}</Badge>} />
          <DetailRow label={t('columns.ready')} value={`${pod.readyContainers}/${pod.totalContainers}`} />
          <DetailRow label={t('columns.restarts')} value={String(pod.restarts)} />
          <DetailRow label={t('detail.fields.controlledBy')} value={pod.controlledBy ?? '—'} />
          <DetailRow label={t('columns.node')} value={pod.node || '—'} />
          <DetailRow label={t('detail.fields.podIP')} value={pod.podIP || '—'} />
          <DetailRow label={t('columns.age')} value={pod.age} />
        </div>

        <DetailSection title={t('detail.utilization', { minutes: windowMinutes })} />
        <div className="detail__charts">
          <TimeSeriesChart title={t('columns.cpu')} data={cpu ?? []} color={chartColor('blue')} height={150} />
          <TimeSeriesChart title={t('columns.memory')} data={memory ?? []} color={chartColor('purple')} height={150} />
        </div>

        <DetailSection title={t('common.labels')} />
        {Object.entries(pod.labels).map(([k, v]) => (
          <span key={k} className="detail__chip">
            {k}={v}
          </span>
        ))}

        {pod.containers.map((c) => (
          <div key={c.name}>
            <DetailSection title={t('detail.container', { name: c.name })} />
            <div className="detail__grid">
              <DetailRow label={t('detail.fields.image')} value={c.image} />
              <DetailRow
                label={t('detail.fields.command')}
                value={c.command ? <code className="detail__code">{c.command.join(' ')}</code> : '—'}
              />
              <DetailRow
                label={t('detail.fields.args')}
                value={c.args ? <code className="detail__code">{c.args.join(' ')}</code> : '—'}
              />
              <DetailRow label={t('columns.ports')} value={c.ports ?? '—'} />
              <DetailRow
                label={t('detail.fields.requests')}
                value={t('detail.resourceAmounts', { cpu: c.requests.cpu, memory: c.requests.memory })}
              />
              <DetailRow
                label={t('detail.fields.limits')}
                value={t('detail.resourceAmounts', { cpu: c.limits.cpu, memory: c.limits.memory })}
              />
            </div>
            <div className="detail__subheading">{t('detail.environment')}</div>
            <table className="kb-table detail__env">
              <thead>
                <tr>
                  <th>{t('common.name')}</th>
                  <th>{t('common.value')}</th>
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

        <DetailSection title={t('common.events')} />
        <EventsTable kind="Pod" namespace={pod.namespace} name={pod.name} />
      </div>
    </div>
  )
}
