import { ApiError } from '../apiClient'
import { reasonError } from '../reasonError'
import type { KubeObject } from '../types'
import { delay, delayReject } from './delay'
import { deployments, namespaces, nodes, pods, services } from './mockCluster'
import { genericMock } from './mockGeneric'
import { deploymentObject, podObject, serviceObject } from './mockK8sObjects'

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
    items: () => pods.map(podObject),
  },
  'v1/services': {
    kind: 'ServiceList',
    apiVersion: 'v1',
    namespaced: true,
    items: () => services.map(serviceObject),
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
    items: () => deployments.map((d) => deploymentObject(d, pods)),
  },
}

// /api/v1/<res>, /api/v1/namespaces/<ns>/<res>, /apis/<group>/<version>/[namespaces/<ns>/]<res>
const PATH = /^\/(?:api\/(v1)|apis\/([^/]+\/[^/]+))(?:\/namespaces\/([^/]+))?\/([^/]+)$/

const notFound = (path: string) =>
  new ApiError(404, 'not_found', `the server could not find the requested resource (${path})`)

export function mockK8sGet<T>(rawPath: string): Promise<T> {
  // Same normalisation as the live client: optional leading slash, query ignored.
  const path = (rawPath.startsWith('/') ? rawPath : `/${rawPath}`).split('?')[0]
  if (path === '/version') {
    return delay({ major: '1', minor: '30', gitVersion: 'v1.30.2' } as T)
  }
  const match = PATH.exec(path)
  if (!match) return delayReject(notFound(path))
  const [, coreVersion, groupVersion, namespace, resource] = match
  if (resource === 'secrets') {
    return delayReject(reasonError(403, 'forbidden', 'passthrough.no_secrets'))
  }
  const shape = RESOURCES[`${coreVersion ?? groupVersion}/${resource}`]
  if (!shape || (namespace && !shape.namespaced)) return delayReject(notFound(path))
  const items = shape.items().filter((o) => !namespace || o.metadata?.namespace === namespace)
  return delay({ kind: shape.kind, apiVersion: shape.apiVersion, metadata: {}, items } as T)
}
