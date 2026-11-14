import { ApiError } from '../apiClient'
import type { KubeObject } from '../types'
import { delay, delayReject } from './delay'
import { deployments, namespaces, nodes, pods, services } from './mockCluster'
import { genericMock } from './mockGeneric'

/**
 * Mock for the read-only Kubernetes passthrough (GET /api/k8s/<path>).
 * Answers the common core list paths with Kubernetes-shaped lists built
 * from the mock dataset; anything else is a 404, like an unknown path on
 * a real API server. Secrets are refused, as by the backend.
 */

interface ListShape {
  kind: string
  apiVersion: string
  items: () => KubeObject[]
  namespaced: boolean
}

const meta = (name: string, namespace?: string, labels: Record<string, string> = {}) => ({
  name,
  ...(namespace ? { namespace } : {}),
  labels,
})

const RESOURCES: Record<string, ListShape> = {
  'v1/namespaces': {
    kind: 'NamespaceList',
    apiVersion: 'v1',
    namespaced: false,
    items: () =>
      namespaces.map((n) => ({ kind: 'Namespace', metadata: meta(n.name), status: { phase: n.status }, age: n.age })),
  },
  'v1/nodes': {
    kind: 'NodeList',
    apiVersion: 'v1',
    namespaced: false,
    items: () =>
      nodes.map((n) => ({
        kind: 'Node',
        metadata: meta(n.name, undefined, n.labels),
        status: { nodeInfo: { kubeletVersion: n.version, osImage: n.os } },
        age: n.age,
      })),
  },
  'v1/pods': {
    kind: 'PodList',
    apiVersion: 'v1',
    namespaced: true,
    items: () =>
      pods.map((p) => ({
        kind: 'Pod',
        metadata: meta(p.name, p.namespace, p.labels),
        spec: { nodeName: p.node },
        status: { phase: p.status, podIP: p.podIP },
        age: p.age,
      })),
  },
  'v1/services': {
    kind: 'ServiceList',
    apiVersion: 'v1',
    namespaced: true,
    items: () =>
      services.map((s) => ({
        kind: 'Service',
        metadata: meta(s.name, s.namespace),
        spec: { type: s.type, clusterIP: s.clusterIP },
        age: s.age,
      })),
  },
  'v1/configmaps': {
    kind: 'ConfigMapList',
    apiVersion: 'v1',
    namespaced: true,
    items: () => genericMock.configmaps ?? [],
  },
  'apps/v1/deployments': {
    kind: 'DeploymentList',
    apiVersion: 'apps/v1',
    namespaced: true,
    items: () =>
      deployments.map((d) => ({
        kind: 'Deployment',
        metadata: meta(d.name, d.namespace, d.labels),
        spec: { replicas: d.replicas },
        status: { readyReplicas: d.readyReplicas, availableReplicas: d.available },
        age: d.age,
      })),
  },
}

// /api/v1/<res>, /api/v1/namespaces/<ns>/<res>, /apis/<group>/<version>/[namespaces/<ns>/]<res>
const PATH = /^\/(?:api\/(v1)|apis\/([^/]+\/[^/]+))(?:\/namespaces\/([^/]+))?\/([^/]+)$/

const notFound = (path: string) =>
  new ApiError(404, 'not_found', `the server could not find the requested resource (${path})`)

export function mockK8sGet<T>(path: string): Promise<T> {
  if (path === '/version') {
    return delay({ major: '1', minor: '30', gitVersion: 'v1.30.2' } as T)
  }
  const match = PATH.exec(path.split('?')[0])
  if (!match) return delayReject(notFound(path))
  const [, coreVersion, groupVersion, namespace, resource] = match
  if (resource === 'secrets') {
    return delayReject(new ApiError(403, 'forbidden', 'secrets are not readable through /api/k8s'))
  }
  const shape = RESOURCES[`${coreVersion ?? groupVersion}/${resource}`]
  if (!shape || (namespace && !shape.namespaced)) return delayReject(notFound(path))
  const items = shape.items().filter((o) => !namespace || o.metadata?.namespace === namespace)
  return delay({ kind: shape.kind, apiVersion: shape.apiVersion, metadata: {}, items } as T)
}
