import { can } from '@/core/permissions'
import { showInPanel } from '@/core/plugin/panelRequests'
import { resourceActionItems } from '@/core/plugin/resourceActions'
import type { ResourceRef } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import { refreshData } from '@/data/DataContext'
import type { DataProviders } from '@/data/providers'
import type { Deployment, Pod } from '@/data/types'
import { showContextMenu, type ContextMenuItem } from '@/shared/contextMenu'
import { notify } from '@/shared/notify'
import { editingMenuItems, yamlMenuItem } from './editing/menuItems'
import { tw } from './i18n'

const ICONS: Record<string, string> = {
  Pod: 'package',
  Deployment: 'rocket',
  Service: 'plug',
  Node: 'server',
}

const SEPARATOR: ContextMenuItem = { type: 'separator' }

/**
 * Menu items with the ones the user may not run left out (`false`), and
 * without the leading, trailing or doubled separators that leaves behind.
 */
function menu(items: (ContextMenuItem | false)[]): ContextMenuItem[] {
  const out: ContextMenuItem[] = []
  for (const item of items) {
    if (!item) continue
    const previous = out[out.length - 1]
    if (item.type === 'separator' && (!previous || previous.type === 'separator')) continue
    out.push(item)
  }
  if (out[out.length - 1]?.type === 'separator') out.pop()
  return out
}

/** Actions other plugins contribute for the row's kind (design §6.4), behind a separator. */
function contributed(ref: ResourceRef): ContextMenuItem[] {
  return [SEPARATOR, ...resourceActionItems(ref)]
}

/** Runs a row mutation; a failure becomes an error toast rather than vanishing. */
async function mutate(action: () => Promise<void>) {
  try {
    await action()
  } catch (err) {
    notify.error(err)
  }
  refreshData()
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

/**
 * Context menu for a pod row: details, logs & shell (bottom panel), YAML,
 * labels, delete (asks first, through `onDelete`) and contributed actions.
 */
export function podMenu(e: React.MouseEvent, pod: Pod, onDelete: (pod: Pod) => void) {
  const container = pod.containers[0]?.name ?? pod.name
  const target = { namespace: pod.namespace, pod: pod.name, container }
  const ref = { kind: 'Pod', apiVersion: 'v1', namespace: pod.namespace, name: pod.name }
  showContextMenu(
    e,
    menu([
      { label: tw('actions.openDetails'), icon: 'go-to-file', onClick: () => openDetail('Pod', pod.namespace, pod.name) },
      SEPARATOR,
      // The terminal plugin shows these kinds in its bottom panel tabs.
      { label: tw('common.logs'), icon: 'output', onClick: () => showInPanel('pod-logs', target) },
      can('exec') && { label: tw('actions.shell'), icon: 'terminal', onClick: () => showInPanel('pod-exec', target) },
      SEPARATOR,
      ...yamlMenuItem(ref),
      ...editingMenuItems(ref, { withDelete: false }),
      can('write') && { label: tw('common.delete'), icon: 'trash', danger: true, onClick: () => onDelete(pod) },
      ...contributed(ref),
    ]),
  )
}

/** Context menu for a deployment row: details, scale/restart, YAML, labels, delete, contributed actions. */
export function deploymentMenu(e: React.MouseEvent, d: Deployment, providers: DataProviders) {
  const canWrite = can('write')
  const ref = { kind: 'Deployment', apiVersion: 'apps/v1', namespace: d.namespace, name: d.name }
  showContextMenu(
    e,
    menu([
      { label: tw('actions.openDetails'), icon: 'go-to-file', onClick: () => openDetail('Deployment', d.namespace, d.name) },
      SEPARATOR,
      canWrite && {
        label: tw('common.restart'),
        icon: 'sync',
        onClick: () => mutate(() => providers.cluster.restartDeployment(d.namespace, d.name)),
      },
      canWrite && {
        label: tw('actions.scaleUpTo', { from: d.replicas, to: d.replicas + 1 }),
        icon: 'add',
        onClick: () => mutate(() => providers.cluster.scaleDeployment(d.namespace, d.name, d.replicas + 1)),
      },
      SEPARATOR,
      ...yamlMenuItem(ref),
      ...editingMenuItems(ref),
      ...contributed(ref),
    ]),
  )
}

/** Menu for a row of any other kind: YAML, labels, delete (Secrets: reveal) plus contributed actions. */
export function genericMenu(e: React.MouseEvent, ref: ResourceRef) {
  showContextMenu(e, menu([...yamlMenuItem(ref), ...editingMenuItems(ref), ...contributed(ref)]))
}
