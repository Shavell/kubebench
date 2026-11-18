import { openYamlTab } from '@/core/plugin/editorTabs'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { TopologyKind, TopologyNode } from './model/types'
import { NS, translate } from './i18n'

/**
 * The topology tab, and the tab kinds of other plugins it opens objects in
 * (design §7 tab kind contract), so it never imports them: the workloads
 * plugin's rich detail page (`workloads-detail`) for Pods, Deployments and
 * Services, its generic detail page (`generic-detail`) for catalog kinds,
 * and the resource editor (`yaml`).
 */

export const TAB_KIND = 'topology'
export const ICON = 'type-hierarchy'

/** The object a topology tab is focused on. */
export interface TopologyFocus {
  kind: TopologyKind
  name: string
}

/** One tab per namespace, and one per focused object. */
export function topologyTabId(namespace: string, focus?: TopologyFocus): string {
  return focus ? `topology:${namespace}:${focus.kind}/${focus.name}` : `topology:${namespace}`
}

export function openTopology(namespace: string, focus?: TopologyFocus): void {
  const titleKey = focus ? 'tab.focused' : 'tab.namespace'
  const titleValues: Record<string, string> = focus ? { name: focus.name } : { namespace }
  useLayoutStore.getState().openTab({
    id: topologyTabId(namespace, focus),
    title: translate(titleKey, titleValues),
    titleKey,
    titleNs: NS,
    titleValues,
    icon: ICON,
    kind: TAB_KIND,
    props: { namespace, focus: focus ?? null },
  })
}

const API_VERSIONS: Record<TopologyKind, string> = {
  Ingress: 'networking.k8s.io/v1',
  Route: 'route.openshift.io/v1',
  Service: 'v1',
  Deployment: 'apps/v1',
  ReplicaSet: 'apps/v1',
  StatefulSet: 'apps/v1',
  DaemonSet: 'apps/v1',
  Pod: 'v1',
  PersistentVolumeClaim: 'v1',
  PersistentVolume: 'v1',
  ConfigMap: 'v1',
  Secret: 'v1',
}

/** Kinds with a rich detail page; the others are generic catalog keys. */
const RICH: Partial<Record<TopologyKind, string>> = { Pod: 'package', Deployment: 'rocket', Service: 'plug' }

const CATALOG_KEYS: Partial<Record<TopologyKind, string>> = {
  Ingress: 'ingresses',
  Route: 'routes',
  ReplicaSet: 'replicasets',
  StatefulSet: 'statefulsets',
  DaemonSet: 'daemonsets',
  PersistentVolumeClaim: 'persistentvolumeclaims',
  PersistentVolume: 'persistentvolumes',
  ConfigMap: 'configmaps',
  Secret: 'secrets',
}

/** Opens a node's object in its detail page (same tab ids as the workloads views use). */
export function openObject(node: TopologyNode): void {
  const namespace = node.namespace || null
  const openTab = useLayoutStore.getState().openTab
  const richIcon = RICH[node.kind]
  if (richIcon) {
    openTab({
      id: `${node.kind.toLowerCase()}:${namespace ?? '-'}/${node.name}`,
      title: node.name,
      icon: richIcon,
      kind: 'workloads-detail',
      props: { kind: node.kind, namespace, name: node.name },
    })
    return
  }
  const key = CATALOG_KEYS[node.kind]!
  openTab({
    id: `res:${key}:${namespace ?? '-'}/${node.name}`,
    title: node.name,
    icon: 'symbol-field',
    kind: 'generic-detail',
    props: { resourceKey: key, namespace, name: node.name },
  })
}

export function openObjectYaml(node: TopologyNode): void {
  openYamlTab({
    apiVersion: API_VERSIONS[node.kind],
    kind: node.kind,
    namespace: node.namespace || undefined,
    name: node.name,
  })
}

/** Kinds the "Show topology" action focuses (design §7.9). */
export const FOCUSABLE_KINDS: TopologyKind[] = ['Deployment', 'StatefulSet', 'DaemonSet', 'Service', 'Ingress', 'Route']
