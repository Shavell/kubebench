import { useMemo } from 'react'
import { useProviderQuery, type QueryState } from '@/data/DataContext'
import { dataMode } from '@/data/mode'
import type { DataProviders } from '@/data/providers'
import type { KubeObject } from '@/data/types'
import type { TopologySnapshot } from '../model/types'

/**
 * Loads the lists a namespace's topology is built from, each on its own,
 * so the graph fills in as they arrive and one list that cannot be read
 * (e.g. Secrets without RBAC) leaves the others standing. Pods,
 * Deployments and Services come from the read-only Kubernetes passthrough,
 * which keeps owner references, selectors and volumes; the other kinds
 * from the generic resource lists. Re-runs on refreshData() and when live
 * updates report a change of its list's topic in the namespace.
 */

export type ListName = keyof TopologySnapshot

/** All lists, in the order their errors are reported. */
export const LIST_NAMES: ListName[] = [
  'pods',
  'deployments',
  'replicaSets',
  'statefulSets',
  'daemonSets',
  'services',
  'ingresses',
  'routes',
  'pvcs',
  'pvs',
  'configMaps',
  'secrets',
]

export interface ListError {
  list: ListName
  error: string
  code: string | null
}

export interface TopologyData {
  snapshot: TopologySnapshot
  /** Lists still loading. */
  pending: number
  errors: ListError[]
}

interface ItemList {
  items?: KubeObject[]
}

const servesRoutes = () => (dataMode.capabilities?.optionalResources ?? []).includes('routes')

const passthrough = (p: DataProviders, path: string) =>
  p.cluster.k8sGet<ItemList>(path).then((list) => list.items ?? [])

/** One list of `topic` in `namespace` (undefined for a cluster-scoped list). */
function useList(
  load: (p: DataProviders) => Promise<KubeObject[]>,
  namespace: string,
  topic: string,
  scoped = true,
): QueryState<KubeObject[]> {
  return useProviderQuery(load, [namespace], { topics: [topic], namespace: scoped ? namespace : undefined })
}

export function useTopologyData(namespace: string): TopologyData {
  const ns = encodeURIComponent(namespace)
  const queries: Record<ListName, QueryState<KubeObject[]>> = {
    pods: useList((p) => passthrough(p, `/api/v1/namespaces/${ns}/pods`), namespace, 'pods'),
    deployments: useList((p) => passthrough(p, `/apis/apps/v1/namespaces/${ns}/deployments`), namespace, 'deployments'),
    services: useList((p) => passthrough(p, `/api/v1/namespaces/${ns}/services`), namespace, 'services'),
    replicaSets: useList((p) => p.cluster.listResource('replicasets', namespace), namespace, 'replicasets'),
    statefulSets: useList((p) => p.cluster.listResource('statefulsets', namespace), namespace, 'statefulsets'),
    daemonSets: useList((p) => p.cluster.listResource('daemonsets', namespace), namespace, 'daemonsets'),
    ingresses: useList((p) => p.cluster.listResource('ingresses', namespace), namespace, 'ingresses'),
    // OpenShift Routes only where the cluster serves them (design §7.15).
    routes: useList(
      (p) => (servesRoutes() ? p.cluster.listResource('routes', namespace) : Promise.resolve([])),
      namespace,
      'routes.route.openshift.io',
    ),
    pvcs: useList((p) => p.cluster.listResource('persistentvolumeclaims', namespace), namespace, 'persistentvolumeclaims'),
    pvs: useList((p) => p.cluster.listResource('persistentvolumes'), namespace, 'persistentvolumes', false),
    configMaps: useList((p) => p.cluster.listResource('configmaps', namespace), namespace, 'configmaps'),
    secrets: useList((p) => p.cluster.listResource('secrets', namespace), namespace, 'secrets'),
  }

  // A refresh keeps each list's previous data, so the snapshot (and the
  // graph built from it) only changes when a list's content arrives.
  const lists = LIST_NAMES.map((name) => queries[name].data)
  const snapshot = useMemo(
    () => Object.fromEntries(LIST_NAMES.map((name, i) => [name, lists[i] ?? undefined])) as TopologySnapshot,
    // eslint-disable-next-line react-hooks/exhaustive-deps
    lists,
  )
  const errors = LIST_NAMES.flatMap((list) => {
    const q = queries[list]
    return q.error ? [{ list, error: q.error, code: q.errorCode }] : []
  })
  const pending = LIST_NAMES.filter((name) => queries[name].loading && !queries[name].data).length
  return { snapshot, pending, errors }
}
