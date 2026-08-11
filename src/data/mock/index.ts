import type { DataProviders } from '../providers'
import { deployments, namespaces, nodes, pods, services, syncDeploymentPods } from './mockCluster'
import { crds, customResources } from './mockCrds'
import { eventsFor } from './mockEvents'
import { podLogs } from './mockLogs'
import { releases, repositories, valuesYaml } from './mockHelm'
import { clusterAllocation, clusterStats, generateSeries } from './mockMetrics'
import {
  customResourceYaml,
  deploymentYaml,
  nodeYaml,
  podYaml,
  serviceYaml,
} from './mockYaml'
import { mulberry32 } from './rng'

/** Simulated network latency so loading states are exercised for real. */
const latencyRand = mulberry32(0x6b85)

function delay<T>(value: T): Promise<T> {
  const ms = 150 + latencyRand() * 150
  return new Promise((resolve) => setTimeout(() => resolve(value), ms))
}

const byNamespace = <T extends { namespace: string }>(items: T[], namespace?: string) =>
  namespace ? items.filter((i) => i.namespace === namespace) : items

/** Mock inventory persists to localStorage; activation is visual-only. */
const INVENTORY_KEY = 'kubebench.mock.clusters'

interface StoredCluster {
  name: string
  kubeconfigPath?: string
  address?: string
  active: boolean
}

function loadStoredClusters(): StoredCluster[] {
  try {
    const raw = localStorage.getItem(INVENTORY_KEY)
    if (raw) return JSON.parse(raw) as StoredCluster[]
  } catch {
    /* fall through */
  }
  return [{ name: 'kind-dev-cluster', kubeconfigPath: '~/.kube/config (mock)', active: true }]
}

function saveStoredClusters(clusters: StoredCluster[]): void {
  localStorage.setItem(INVENTORY_KEY, JSON.stringify(clusters))
}

export function createMockProviders(): DataProviders {
  return {
    cluster: {
      listNodes: () => delay(nodes),
      listNamespaces: () => delay(namespaces),
      listPods: (ns) => delay(byNamespace(pods, ns)),
      listDeployments: (ns) => delay(byNamespace(deployments, ns)),
      listServices: (ns) => delay(byNamespace(services, ns)),
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
      getPodLogs: (_ns, pod, container) => delay(podLogs(pod, container)),
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
    },
    crd: {
      listCrds: () => delay(crds),
      listCustomResources: (crdName) =>
        delay(customResources.filter((cr) => cr.crdName === crdName)),
    },
    helm: {
      listReleases: () => delay(releases),
      listRepositories: () => delay(repositories),
      getReleaseValuesYaml: (name) => delay(valuesYaml[name] ?? `# no values for ${name}`),
      rollbackRelease: (name, revision) => {
        const release = releases.find((r) => r.name === name)
        if (release) {
          const newRevision = release.revision + 1
          for (const h of release.history) {
            if (h.status === 'deployed' || h.status === 'failed') h.status = 'superseded'
          }
          release.history.unshift({
            revision: newRevision,
            status: 'deployed',
            updated: '2026-08-11 09:15',
            description: `Rollback to ${revision}`,
          })
          release.revision = newRevision
          release.status = 'deployed'
          release.updated = '2026-08-11 09:15'
        }
        return delay(undefined)
      },
      uninstallRelease: (name) => {
        const idx = releases.findIndex((r) => r.name === name)
        if (idx >= 0) releases.splice(idx, 1)
        return delay(undefined)
      },
    },
    inventory: {
      listClusters: () =>
        delay(loadStoredClusters().map((c) => ({ ...c, context: c.name, server: c.address }))),
      addCluster: (entry) => {
        const clusters = loadStoredClusters()
        if (clusters.some((c) => c.name === entry.name)) {
          return Promise.reject(new Error(`cluster "${entry.name}" already exists`))
        }
        clusters.push({ ...entry, active: false })
        saveStoredClusters(clusters)
        return delay(undefined)
      },
      removeCluster: (name) => {
        saveStoredClusters(loadStoredClusters().filter((c) => c.name !== name))
        return delay(undefined)
      },
      activateCluster: (name) => {
        saveStoredClusters(loadStoredClusters().map((c) => ({ ...c, active: c.name === name })))
        return delay(undefined)
      },
    },
    metrics: {
      clusterOverview: () => delay(clusterStats()),
      clusterAllocation: () => delay(clusterAllocation()),
      nodeSeries: (node, metric, minutes) => delay(generateSeries(node, metric, minutes)),
      clusterSeries: (metric, minutes) => delay(generateSeries('cluster', metric, minutes)),
      podSeries: (ns, pod, metric, minutes) =>
        delay(generateSeries(`pod:${ns}/${pod}`, metric, minutes)),
    },
  }
}
