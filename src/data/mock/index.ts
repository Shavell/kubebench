import { ApiError } from '../apiClient'
import type { DataProviders } from '../providers'
import type { ListOptions, ListResult } from '../types'
import { deployments, namespaces, nodes, pods, services, syncDeploymentPods } from './mockCluster'
import { crds, customResources } from './mockCrds'
import { delay, delayReject } from './delay'
import { eventsFor } from './mockEvents'
import { mockExecOutput } from './mockExec'
import { genericMock } from './mockGeneric'
import { mockOptionalResources, OPENSHIFT_RESOURCES } from './mockOpenShift'
import { podLogs } from './mockLogs'
import { streamMockLogs } from './mockLogStream'
import { openMockTerminal } from './mockTerminal'
import { clusterAllocation, clusterStats, generateSeries } from './mockMetrics'
import { createMockAudit } from './mockAudit'
import { activeMockCluster, createMockInventory } from './mockInventory'
import { mockK8sGet } from './mockK8s'
import { createMockPlugins } from './mockPlugins'
import { createMockSession } from './mockSession'
import { createMockUsers } from './mockUsers'
import { createMockIdentity } from './mockIdentity'
import { mockEditing } from './mockEditing'
import { createMockWorkloadOps } from './mockWorkloadOps'
import { createMockUpdates } from './mockUpdates'
import { createMockLive } from './mockWatch'
import {
  customResourceYaml,
  deploymentYaml,
  nodeYaml,
  podYaml,
  serviceYaml,
} from './mockYaml'

/** Whether the active mock cluster serves a catalog type: OpenShift's only on the OpenShift cluster. */
const servedByMock = (key: string) =>
  !OPENSHIFT_RESOURCES.includes(key) || mockOptionalResources(activeMockCluster().platform).includes(key)

const byNamespace = <T extends { namespace: string }>(items: T[], namespace?: string) =>
  namespace ? items.filter((i) => i.namespace === namespace) : items

/** A list cut at `limit` as the server cuts it (design §4.4, "Lists"). */
function limited<T>(items: readonly T[], options?: ListOptions): ListResult<T> {
  const limit = options?.limit
  if (!limit || items.length <= limit) return [...items]
  const out: ListResult<T> = items.slice(0, limit)
  out.truncated = { remaining: items.length - limit }
  return out
}

/** How many mock objects a live-update topic's resource has (GET /api/count). */
function mockCount(topic: string, namespace?: string): number {
  const resource = topic.split('.')[0]
  switch (resource) {
    case 'pods':
      return byNamespace(pods, namespace).length
    case 'deployments':
      return byNamespace(deployments, namespace).length
    case 'services':
      return byNamespace(services, namespace).length
    case 'nodes':
      return nodes.length
    case 'namespaces':
      return namespaces.length
  }
  const items = servedByMock(resource) ? (genericMock[resource] ?? []) : []
  return namespace ? items.filter((o) => !o.metadata?.namespace || o.metadata.namespace === namespace).length : items.length
}

export function createMockProviders(): DataProviders {
  return {
    cluster: {
      listNodes: (options) => delay(limited(nodes, options)),
      listNamespaces: (options) => delay(limited(namespaces, options)),
      listPods: (ns, options) => delay(limited(byNamespace(pods, ns), options)),
      listDeployments: (ns, options) => delay(limited(byNamespace(deployments, ns), options)),
      listServices: (ns, options) => delay(limited(byNamespace(services, ns), options)),
      countResource: (topic, ns) => delay(mockCount(topic, ns)),
      getResourceYaml: (kind, ns, name) => {
        switch (kind) {
          case 'Pod': {
            const pod = pods.find((p) => p.namespace === ns && p.name === name)
            return delay(pod ? podYaml(pod) : `# ${kind} ${name} not found`)
          }
          case 'Deployment': {
            const d = deployments.find((x) => x.namespace === ns && x.name === name)
            return delay(d ? deploymentYaml(d) : `# ${kind} ${name} not found`)
          }
          case 'Service': {
            const s = services.find((x) => x.namespace === ns && x.name === name)
            return delay(s ? serviceYaml(s) : `# ${kind} ${name} not found`)
          }
          case 'Node': {
            const n = nodes.find((x) => x.name === name)
            return delay(n ? nodeYaml(n) : `# ${kind} ${name} not found`)
          }
          default: {
            const cr = customResources.find((x) => x.name === name && x.namespace === ns)
            if (cr) {
              const crd = crds.find((c) => c.name === cr.crdName)
              return delay(
                customResourceYaml(cr, `${crd?.group}/${crd?.versions[0]}`, crd?.kind ?? kind),
              )
            }
            return delay(`# ${kind} ${name} not found`)
          }
        }
      },
      listEvents: (kind, ns, name) => delay(eventsFor(kind, ns, name)),
      getPodLogs: (ns, podName, container, options = {}) => {
        const pod = pods.find((p) => p.namespace === ns && p.name === podName)
        if (options.previous && !pod?.restarts) {
          // What the Kubernetes API answers for a container that never restarted.
          return delayReject(
            new ApiError(
              400,
              'kubernetes',
              `previous terminated container "${container}" in pod "${podName}" not found`,
            ),
          )
        }
        return delay(podLogs(podName, container, options))
      },
      listResource: (key, ns, options) => {
        // Like a live cluster, one that does not serve a type lists none.
        const items = servedByMock(key) ? (genericMock[key] ?? []) : []
        return delay(limited(ns ? items.filter((o) => o.metadata?.namespace === ns) : items, options))
      },
      getResource: (key, ns, name) => {
        const found = (genericMock[key] ?? []).find(
          (o) => o.metadata?.name === name && (!ns || o.metadata?.namespace === ns),
        )
        return delay(found ?? { kind: key, metadata: { name } })
      },
      execCommand: (ns, podName, container, command) => delay(mockExecOutput(ns, podName, container, command)),
      streamLogs: (request, handlers) => streamMockLogs(request, handlers),
      openTerminal: (target, handlers) =>
        openMockTerminal(target, handlers, (command) =>
          delay(mockExecOutput(target.namespace, target.pod, target.container, command)),
        ),
      k8sGet: <T>(path: string) => mockK8sGet<T>(path),
      deletePod: (ns, name) => {
        const idx = pods.findIndex((p) => p.namespace === ns && p.name === name)
        if (idx >= 0) pods.splice(idx, 1)
        return delay(undefined)
      },
      scaleDeployment: (ns, name, replicas) => {
        const d = deployments.find((x) => x.namespace === ns && x.name === name)
        if (d) {
          syncDeploymentPods(d, replicas)
          d.replicas = replicas
          d.upToDate = replicas
          d.readyReplicas = Math.min(replicas, d.readyReplicas + (replicas - d.available))
          d.readyReplicas = Math.max(0, Math.min(replicas, d.readyReplicas))
          d.available = d.readyReplicas
        }
        return delay(undefined)
      },
      restartDeployment: (ns, name) => {
        const d = deployments.find((x) => x.namespace === ns && x.name === name)
        if (d) {
          const app = d.labels.app ?? d.labels['k8s-app'] ?? d.name
          for (const p of pods) {
            if (p.namespace === ns && p.labels.app === app) {
              p.age = '5s'
              p.restarts = 0
              p.status = 'Running'
              p.readyContainers = p.totalContainers
            }
          }
          d.readyReplicas = d.replicas
          d.available = d.replicas
        }
        return delay(undefined)
      },
      ...mockEditing,
    },
    crd: {
      listCrds: () => delay(crds),
      listCustomResources: (crdName, options) =>
        delay(limited(customResources.filter((cr) => cr.crdName === crdName), options)),
    },
    inventory: createMockInventory(),
    users: createMockUsers(),
    session: createMockSession(),
    audit: createMockAudit(),
    plugins: createMockPlugins(),
    workloadOps: createMockWorkloadOps(),
    updates: createMockUpdates(),
    live: createMockLive(),
    identity: createMockIdentity(),
    metrics: {
      listSources: () =>
        delay([
          { id: 'metrics-server', name: 'metrics-server', available: true, detail: 'mock' },
          { id: 'prometheus', name: 'Prometheus', available: true, detail: 'monitoring/prometheus-server (mock)' },
        ]),
      clusterOverview: () => delay(clusterStats()),
      clusterAllocation: () => delay(clusterAllocation()),
      nodeSeries: (node, metric, minutes) => delay(generateSeries(node, metric, minutes)),
      clusterSeries: (metric, minutes) => delay(generateSeries('cluster', metric, minutes)),
      podSeries: (ns, pod, metric, minutes) =>
        delay(generateSeries(`pod:${ns}/${pod}`, metric, minutes)),
    },
  }
}
