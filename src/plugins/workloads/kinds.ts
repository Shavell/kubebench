import type { ResourceKindContribution, ResourceKindObject, TabDescriptor } from '@/core/plugin/types'
import { useLayoutStore } from '@/core/store/layoutStore'
import type { DataProviders } from '@/data/providers'
import { WORKLOADS_NS } from './i18n'
import { objName, objNamespace, RESOURCE_CATALOG, resourceLabel, richLabel } from './resourceCatalog'

/** kubectl's short names (`kubectl api-resources`) of the kinds this plugin lists. */
const SHORT_NAMES: Record<string, string[]> = {
  pods: ['po'],
  deployments: ['deploy'],
  services: ['svc'],
  nodes: ['no'],
  namespaces: ['ns'],
  configmaps: ['cm'],
  resourcequotas: ['quota'],
  limitranges: ['limits'],
  horizontalpodautoscalers: ['hpa'],
  poddisruptionbudgets: ['pdb'],
  priorityclasses: ['pc'],
  statefulsets: ['sts'],
  daemonsets: ['ds'],
  replicasets: ['rs'],
  replicationcontrollers: ['rc'],
  cronjobs: ['cj'],
  ingresses: ['ing'],
  endpoints: ['ep'],
  networkpolicies: ['netpol'],
  persistentvolumeclaims: ['pvc'],
  persistentvolumes: ['pv'],
  storageclasses: ['sc'],
  serviceaccounts: ['sa'],
  events: ['ev'],
}

/** plural, singular (the kind in lowercase), short names. */
function namesOf(plural: string, kind: string): string[] {
  return [...new Set([plural, kind.toLowerCase(), ...(SHORT_NAMES[plural] ?? [])])]
}

const openTab = (tab: TabDescriptor) => useLayoutStore.getState().openTab(tab)

type Lister = (providers: DataProviders, namespace?: string) => Promise<{ name: string; namespace?: string }[]>

const RICH: { resource: string; kind: string; icon: string; namespaced: boolean; list: Lister }[] = [
  { resource: 'Pods', kind: 'Pod', icon: 'package', namespaced: true, list: (p, ns) => p.cluster.listPods(ns) },
  { resource: 'Deployments', kind: 'Deployment', icon: 'rocket', namespaced: true, list: (p, ns) => p.cluster.listDeployments(ns) },
  { resource: 'Services', kind: 'Service', icon: 'plug', namespaced: true, list: (p, ns) => p.cluster.listServices(ns) },
  { resource: 'Nodes', kind: 'Node', icon: 'server', namespaced: false, list: (p) => p.cluster.listNodes() },
  { resource: 'Namespaces', kind: 'Namespace', icon: 'folder', namespaced: false, list: (p) => p.cluster.listNamespaces() },
]

const objects = (items: { name: string; namespace?: string }[]): ResourceKindObject[] =>
  items.map((o) => ({ name: o.name, namespace: o.namespace || undefined }))

/** The rich lists (Pods, Deployments, …) of the `workloads-list` / `workloads-detail` tab kinds. */
const richKinds: ResourceKindContribution[] = RICH.map((r) => ({
  kind: r.kind,
  names: namesOf(r.resource.toLowerCase(), r.kind),
  namespaced: r.namespaced,
  icon: r.icon,
  openList: (namespace) =>
    openTab({
      id: namespace ? `list:${r.resource}:${namespace}` : `list:${r.resource}`,
      title: namespace ? `${richLabel(r.resource)} · ${namespace}` : richLabel(r.resource),
      ...(namespace ? {} : { titleKey: `resources.${r.resource.toLowerCase()}`, titleNs: WORKLOADS_NS }),
      icon: r.icon,
      kind: 'workloads-list',
      props: { resource: r.resource, namespace },
    }),
  openDetail: ({ namespace, name }) =>
    openTab({
      id: `${r.kind.toLowerCase()}:${namespace ?? '-'}/${name}`,
      title: name,
      icon: r.icon,
      kind: 'workloads-detail',
      props: { kind: r.kind, namespace: r.namespaced ? (namespace ?? null) : null, name },
    }),
  listObjects: async (providers, namespace) => objects(await r.list(providers, namespace)),
}))

/** Every other catalog resource (`generic-list` / `generic-detail`). */
const catalogKinds: ResourceKindContribution[] = RESOURCE_CATALOG.map((d) => ({
  kind: d.kind,
  names: namesOf(d.key, d.kind),
  namespaced: d.namespaced,
  icon: d.icon,
  openList: (namespace) =>
    openTab({
      id: namespace ? `reslist:${d.key}:${namespace}` : `reslist:${d.key}`,
      title: namespace ? `${resourceLabel(d.key)} · ${namespace}` : resourceLabel(d.key),
      ...(namespace ? {} : { titleKey: `resources.${d.key}`, titleNs: WORKLOADS_NS }),
      icon: d.icon,
      kind: 'generic-list',
      props: { resourceKey: d.key, namespace },
    }),
  openDetail: ({ namespace, name }) =>
    openTab({
      id: `res:${d.key}:${namespace || '-'}/${name}`,
      title: name,
      icon: d.icon,
      kind: 'generic-detail',
      props: { resourceKey: d.key, namespace: d.namespaced ? (namespace ?? null) : null, name },
    }),
  listObjects: async (providers, namespace) =>
    (await providers.cluster.listResource(d.key, namespace)).map((o) => ({
      name: objName(o),
      namespace: objNamespace(o) || undefined,
    })),
}))

/** Kinds for the palette's kubectl-style `get` / `describe` commands. */
export const workloadKinds: ResourceKindContribution[] = [...richKinds, ...catalogKinds]
