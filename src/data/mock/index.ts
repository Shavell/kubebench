import type { DataProviders } from '../providers'
import { deployments, namespaces, nodes, pods, services } from './mockCluster'
import { crds, customResources } from './mockCrds'
import { releases, repositories, valuesYaml } from './mockHelm'
import { clusterStats, generateSeries } from './mockMetrics'
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
    },
    metrics: {
      clusterOverview: () => delay(clusterStats()),
      nodeSeries: (node, metric, minutes) => delay(generateSeries(node, metric, minutes)),
      clusterSeries: (metric, minutes) => delay(generateSeries('cluster', metric, minutes)),
    },
  }
}
