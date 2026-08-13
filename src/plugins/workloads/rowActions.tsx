import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData } from '@/data/DataContext'
import type { DataProviders } from '@/data/providers'
import type { Deployment, Pod } from '@/data/types'
import { showContextMenu, type ContextMenuItem } from '@/shared/contextMenu'
import { useTerminalStore } from '@/plugins/terminal/terminalStore'

const ICONS: Record<string, string> = {
  Pod: 'package',
  Deployment: 'rocket',
  Service: 'plug',
  Node: 'server',
}

function openDetail(kind: string, namespace: string | null, name: string) {
  useLayoutStore.getState().openTab({
    id: `${kind.toLowerCase()}:${namespace ?? '-'}/${name}`,
    title: name,
    icon: ICONS[kind] ?? 'symbol-field',
    kind: 'workloads-detail',
    props: { kind, namespace, name },
  })
}

async function openYaml(
  providers: DataProviders,
  kind: string,
  namespace: string | null,
  name: string,
) {
  const text = await providers.cluster.getResourceYaml(kind, namespace, name)
  useLayoutStore.getState().openTab({
    id: `yaml:${kind}:${namespace ?? '-'}/${name}`,
    title: `${name}.yaml`,
    icon: 'file-code',
    kind: 'yaml',
    props: { text, readOnly: true },
  })
}

/** Context menu for a pod row: details, logs & shell (bottom panel), YAML, delete. */
export function podMenu(e: React.MouseEvent, pod: Pod, providers: DataProviders) {
  const container = pod.containers[0]?.name ?? pod.name
  const target = { namespace: pod.namespace, pod: pod.name, container }
  const items: ContextMenuItem[] = [
    { label: 'Open Details', icon: 'go-to-file', onClick: () => openDetail('Pod', pod.namespace, pod.name) },
    { type: 'separator' },
    { label: 'Logs', icon: 'output', onClick: () => useTerminalStore.getState().showLogs(target) },
    { label: 'Shell', icon: 'terminal', onClick: () => useTerminalStore.getState().showExec(target) },
    { type: 'separator' },
    { label: 'View YAML', icon: 'file-code', onClick: () => openYaml(providers, 'Pod', pod.namespace, pod.name) },
    {
      label: 'Delete',
      icon: 'trash',
      danger: true,
      onClick: async () => {
        try {
          await providers.cluster.deletePod(pod.namespace, pod.name)
          refreshData()
        } catch {
          /* surfaced elsewhere; ignore here */
        }
      },
    },
  ]
  showContextMenu(e, items)
}

/** Context menu for a deployment row: details, scale/restart, YAML. */
export function deploymentMenu(e: React.MouseEvent, d: Deployment, providers: DataProviders) {
  showContextMenu(e, [
    { label: 'Open Details', icon: 'go-to-file', onClick: () => openDetail('Deployment', d.namespace, d.name) },
    { type: 'separator' },
    {
      label: 'Restart',
      icon: 'sync',
      onClick: async () => {
        await providers.cluster.restartDeployment(d.namespace, d.name).catch(() => {})
        refreshData()
      },
    },
    {
      label: `Scale up (${d.replicas} → ${d.replicas + 1})`,
      icon: 'add',
      onClick: async () => {
        await providers.cluster.scaleDeployment(d.namespace, d.name, d.replicas + 1).catch(() => {})
        refreshData()
      },
    },
    { type: 'separator' },
    { label: 'View YAML', icon: 'file-code', onClick: () => openYaml(providers, 'Deployment', d.namespace, d.name) },
  ])
}

/** Generic menu: open details + YAML for any kind. */
export function genericMenu(
  e: React.MouseEvent,
  kind: string,
  namespace: string | null,
  name: string,
  providers: DataProviders,
) {
  showContextMenu(e, [
    { label: 'View YAML', icon: 'file-code', onClick: () => openYaml(providers, kind, namespace, name) },
  ])
}
