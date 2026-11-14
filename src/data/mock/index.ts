import type { DataProviders } from '../providers'
import { deployments, namespaces, nodes, pods, services, syncDeploymentPods } from './mockCluster'
import { crds, customResources } from './mockCrds'
import { delay } from './delay'
import { eventsFor } from './mockEvents'
import { genericMock } from './mockGeneric'
import { podLogs } from './mockLogs'
import { releases, repositories, valuesYaml } from './mockHelm'
import { clusterAllocation, clusterStats, generateSeries } from './mockMetrics'
import { createMockAudit } from './mockAudit'
import { createMockInventory } from './mockInventory'
import { mockK8sGet } from './mockK8s'
import { createMockPlugins } from './mockPlugins'
import { createMockSession, createMockUsers } from './mockUsers'
import {
  customResourceYaml,
  deploymentYaml,
  nodeYaml,
  podYaml,
  serviceYaml,
} from './mockYaml'

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
      listEvents: (kind, ns, name) => delay(eventsFor(kind, ns, name)),
      getPodLogs: (_ns, pod, container) => delay(podLogs(pod, container)),
      listResource: (key, ns) => {
        const items = genericMock[key] ?? []
        return delay(ns ? items.filter((o) => o.metadata?.namespace === ns) : items)
      },
      getResource: (key, ns, name) => {
        const found = (genericMock[key] ?? []).find(
          (o) => o.metadata?.name === name && (!ns || o.metadata?.namespace === ns),
        )
        return delay(found ?? { kind: key, metadata: { name } })
      },
      execCommand: (ns, podName, container, command) => {
        const pod = pods.find((p) => p.namespace === ns && p.name === podName)
        const env = pod?.containers[0]?.env ?? []
        const [bin, ...args] = command.trim().split(/\s+/)
        const out = (() => {
          switch (bin) {
            case '':
              return []
            case 'help':
              return ['Mock shell. Try: ls, pwd, whoami, hostname, env, ps, uname']
            case 'ls':
              return ['app  bin  dev  etc  home  lib  proc  root  sys  tmp  usr  var']
            case 'pwd':
              return ['/']
            case 'whoami':
              return ['root']
            case 'hostname':
              return [podName]
            case 'uname':
              return [`Linux ${podName} 6.6.16 #1 SMP x86_64 GNU/Linux`]
            case 'env':
              return [
                `HOSTNAME=${podName}`,
                'PATH=/usr/local/sbin:/usr/local/bin:/usr/sbin:/usr/bin:/sbin:/bin',
                'KUBERNETES_SERVICE_HOST=10.96.0.1',
                ...env.map((e) => `${e.name}=${e.value}`),
              ]
            case 'ps':
              return ['PID   USER     COMMAND', `1     root     /app/${container}`, '27    root     sh']
            case 'cat':
              return args.length ? [`cat: ${args[0]}: Permission denied (mock)`] : ['usage: cat FILE']
            default:
              return [`/bin/sh: ${bin}: command not found`]
          }
        })()
        return delay(out)
      },
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
    },
    crd: {
      listCrds: () => delay(crds),
      listCustomResources: (crdName) =>
        delay(customResources.filter((cr) => cr.crdName === crdName)),
    },
    helm: {
      listReleases: () => delay(releases),
      listRepositories: () => delay(repositories),
      getReleaseValuesYaml: (name, _ns) => delay(valuesYaml[name] ?? `# no values for ${name}`),
      rollbackRelease: (name, _ns, revision) => {
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
      uninstallRelease: (name, _ns) => {
        const idx = releases.findIndex((r) => r.name === name)
        if (idx >= 0) releases.splice(idx, 1)
        return delay(undefined)
      },
    },
    inventory: createMockInventory(),
    users: createMockUsers(),
    session: createMockSession(),
    audit: createMockAudit(),
    plugins: createMockPlugins(),
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
