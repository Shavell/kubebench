import { createServer } from 'node:http'
import { URL } from 'node:url'
import * as yaml from 'js-yaml'
import { createKubeClient } from './kube.mjs'
import { startMetricsSampler } from './metricsSampler.mjs'
import {
  mapDeployment,
  mapEvent,
  mapNamespace,
  mapNode,
  mapPod,
  mapService,
  parseCpu as parseCpuQty,
  parseMemoryMi as parseMemQty,
} from './mappers.mjs'

const round1 = (n) => Math.round(n * 10) / 10

import { readFileSync } from 'node:fs'
import { inventoryPath, loadInventory, saveInventory } from './inventory.mjs'
import { discoverPrometheus } from './prometheus.mjs'

const PORT = Number(process.env.PORT ?? 9876)

/** Active-cluster state; rebuilt when the inventory activates another entry. */
let active = null // { name, kube, metrics }
const inventory = loadInventory()

function describeKubeconfig(path) {
  try {
    const config = yaml.load(readFileSync(path, 'utf8'))
    const cluster = config.clusters?.[0]?.cluster
    return { context: config['current-context'] ?? config.contexts?.[0]?.name, server: cluster?.server }
  } catch (err) {
    return { error: err.code === 'ENOENT' ? 'kubeconfig file not found' : err.message }
  }
}

function activateCluster(name) {
  const entry = inventory.clusters.find((c) => c.name === name)
  if (!entry) throw Object.assign(new Error(`unknown cluster "${name}"`), { status: 404 })
  if (!entry.kubeconfigPath) {
    throw Object.assign(
      new Error(`cluster "${name}" has no kubeconfig — address-only entries cannot be activated`),
      { status: 400 },
    )
  }
  const kube = createKubeClient(entry.kubeconfigPath)
  active?.metrics?.stop()
  active = { name, kube, metrics: startMetricsSampler(kube), prom: null }
  inventory.active = name
  saveInventory(inventory)
  console.log(`[kube] active=${name} context=${kube.contextName} server=${kube.server}`)
  const current = active
  discoverPrometheus(kube).then((prom) => {
    if (active === current) active.prom = prom
    if (!prom) console.log('[prometheus] not found or not reachable — using metrics-server only')
  })
}

// Seed the inventory from KUBECONFIG on first run.
if (inventory.clusters.length === 0 && process.env.KUBECONFIG) {
  try {
    const probe = createKubeClient(process.env.KUBECONFIG)
    inventory.clusters.push({ name: probe.contextName, kubeconfigPath: process.env.KUBECONFIG })
    inventory.active = probe.contextName
    saveInventory(inventory)
  } catch (err) {
    console.error(`[inventory] could not seed from KUBECONFIG: ${err.message}`)
  }
}
if (inventory.active) {
  try {
    activateCluster(inventory.active)
  } catch (err) {
    console.error(`[kube] could not activate "${inventory.active}": ${err.message}`)
  }
}
console.log(`[inventory] ${inventory.clusters.length} cluster(s) in ${inventoryPath()}`)

function requireActive() {
  if (!active) throw Object.assign(new Error('no active cluster'), { status: 503 })
  return active
}

const kube = new Proxy({}, { get: (_t, prop) => requireActive().kube[prop] })
const metrics = new Proxy({}, { get: (_t, prop) => requireActive().metrics[prop] })

/**
 * Picks the metrics backend for a request. source=prometheus |
 * metrics-server | auto (default: prometheus when discovered).
 * Returns {source} for prometheus, {sampler} for metrics-server.
 * metrics-server has no network/fs data, so those 501 for client fallback.
 */
function resolveMetricsSource(q) {
  const wanted = q.get('source') ?? 'auto'
  const { prom, metrics: sampler } = requireActive()
  const metric = q.get('metric')

  if (wanted === 'prometheus') {
    if (!prom) throw Object.assign(new Error('prometheus source not available'), { status: 503 })
    return { source: prom, sampler }
  }
  const useProm = wanted === 'auto' && prom
  if (!useProm && (metric === 'network' || metric === 'fs')) {
    throw Object.assign(new Error(`metrics-server has no "${metric}" data`), { status: 501 })
  }
  return { source: useProm ? prom : null, sampler }
}

const KIND_PATHS = {
  Pod: (ns, name) => `/api/v1/namespaces/${ns}/pods/${name}`,
  Deployment: (ns, name) => `/apis/apps/v1/namespaces/${ns}/deployments/${name}`,
  Service: (ns, name) => `/api/v1/namespaces/${ns}/services/${name}`,
  Node: (_ns, name) => `/api/v1/nodes/${name}`,
  Namespace: (_ns, name) => `/api/v1/namespaces/${name}`,
}

function nsPath(resource, ns) {
  const apiRoot = resource === 'deployments' ? '/apis/apps/v1' : '/api/v1'
  return ns ? `${apiRoot}/namespaces/${ns}/${resource}` : `${apiRoot}/${resource}`
}

const routes = {
  'GET /api/info': async () => ({
    context: kube.contextName,
    server: kube.server,
    namespace: kube.namespace ?? null,
  }),

  'GET /api/clusters': async () =>
    inventory.clusters.map((c) => ({
      name: c.name,
      kubeconfigPath: c.kubeconfigPath,
      address: c.address,
      active: c.name === inventory.active,
      ...(c.kubeconfigPath ? describeKubeconfig(c.kubeconfigPath) : { server: c.address }),
    })),
  'POST /api/clusters': async (q) => {
    const name = q.get('name')?.trim()
    const path = q.get('path')?.trim()
    const address = q.get('address')?.trim()
    if (!name) throw Object.assign(new Error('name is required'), { status: 400 })
    if (!path && !address)
      throw Object.assign(new Error('kubeconfig path or API address is required'), { status: 400 })
    if (inventory.clusters.some((c) => c.name === name))
      throw Object.assign(new Error(`cluster "${name}" already exists`), { status: 409 })
    if (path) createKubeClient(path) // validate before saving
    inventory.clusters.push({ name, kubeconfigPath: path || undefined, address: address || undefined })
    saveInventory(inventory)
    return { ok: true }
  },
  'DELETE /api/clusters': async (q) => {
    const name = q.get('name')
    inventory.clusters = inventory.clusters.filter((c) => c.name !== name)
    if (inventory.active === name) {
      inventory.active = null
      active?.metrics?.stop()
      active = null
    }
    saveInventory(inventory)
    return { ok: true }
  },
  'POST /api/clusters/activate': async (q) => {
    activateCluster(q.get('name'))
    return { ok: true }
  },

  'GET /api/nodes': async () => (await kube.getJSON('/api/v1/nodes')).items.map(mapNode),
  'GET /api/namespaces': async () => (await kube.getJSON('/api/v1/namespaces')).items.map(mapNamespace),
  'GET /api/pods': async (q) => (await kube.getJSON(nsPath('pods', q.get('ns')))).items.map(mapPod),
  'GET /api/deployments': async (q) =>
    (await kube.getJSON(nsPath('deployments', q.get('ns')))).items.map(mapDeployment),
  'GET /api/services': async (q) =>
    (await kube.getJSON(nsPath('services', q.get('ns')))).items.map(mapService),

  'GET /api/yaml': async (q) => {
    const kind = q.get('kind')
    const pathFor = KIND_PATHS[kind]
    if (!pathFor) return { yaml: `# unsupported kind ${kind}` }
    const obj = await kube.getJSON(pathFor(q.get('ns'), q.get('name')))
    delete obj.metadata?.managedFields
    return { yaml: yaml.dump(obj, { noRefs: true, lineWidth: 140 }) }
  },

  'GET /api/events': async (q) => {
    const ns = q.get('ns')
    const name = q.get('name')
    const base = ns ? `/api/v1/namespaces/${ns}/events` : '/api/v1/events'
    const list = await kube.getJSON(`${base}?fieldSelector=${encodeURIComponent(`involvedObject.name=${name}`)}`)
    return list.items
      .map(mapEvent)
      .sort((a, b) => (a.age < b.age ? -1 : 1))
  },

  'GET /api/logs': async (q) => {
    const ns = q.get('ns')
    const pod = q.get('pod')
    const container = q.get('container')
    const raw = await kube.getRaw(
      `/api/v1/namespaces/${ns}/pods/${pod}/log?tailLines=300&timestamps=true${container ? `&container=${container}` : ''}`,
    )
    return raw
      .split('\n')
      .filter(Boolean)
      .map((line) => {
        const space = line.indexOf(' ')
        const ts = line.slice(0, space)
        const time = ts.includes('T') ? ts.split('T')[1].replace(/Z.*/, '').slice(0, 12) : ts
        return `${time} ${line.slice(space + 1)}`
      })
  },

  'GET /api/metrics/sources': async () => ({
    metricsServer: { available: true },
    prometheus: {
      available: Boolean(requireActive().prom),
      location: requireActive().prom?.location ?? null,
    },
  }),

  'GET /api/metrics/cluster': async (q) => {
    const { source, sampler } = resolveMetricsSource(q)
    return source
      ? source.clusterSeries(q.get('metric'), Number(q.get('minutes') ?? 60))
      : sampler.clusterSeries(q.get('metric'), Number(q.get('minutes') ?? 60))
  },
  'GET /api/metrics/node': async (q) => {
    const { source, sampler } = resolveMetricsSource(q)
    return source
      ? source.nodeSeries(q.get('node'), q.get('metric'), Number(q.get('minutes') ?? 60))
      : sampler.nodeSeries(q.get('node'), q.get('metric'), Number(q.get('minutes') ?? 60))
  },
  'GET /api/metrics/pod': async (q) => {
    const { source, sampler } = resolveMetricsSource(q)
    return source
      ? source.podSeries(q.get('ns'), q.get('pod'), q.get('metric'), Number(q.get('minutes') ?? 60))
      : sampler.podSeries(q.get('ns'), q.get('pod'), q.get('metric'), Number(q.get('minutes') ?? 60))
  },
  'GET /api/metrics/allocation': async () => {
    const [nodes, pods] = await Promise.all([
      kube.getJSON('/api/v1/nodes'),
      kube.getJSON('/api/v1/pods'),
    ])
    let allocatableCpu = 0
    let allocatableMemMi = 0
    for (const n of nodes.items) {
      allocatableCpu += parseCpuQty(n.status.allocatable?.cpu)
      allocatableMemMi += parseMemQty(n.status.allocatable?.memory)
    }
    let reqCpu = 0
    let limCpu = 0
    let reqMemMi = 0
    let limMemMi = 0
    for (const p of pods.items) {
      if (p.status?.phase === 'Succeeded' || p.status?.phase === 'Failed') continue
      for (const c of p.spec?.containers ?? []) {
        reqCpu += parseCpuQty(c.resources?.requests?.cpu)
        limCpu += parseCpuQty(c.resources?.limits?.cpu)
        reqMemMi += parseMemQty(c.resources?.requests?.memory)
        limMemMi += parseMemQty(c.resources?.limits?.memory)
      }
    }
    const usage = metrics.latestClusterUsage()
    return {
      cpu: {
        unit: 'cores',
        allocatable: round1(allocatableCpu),
        requests: round1(reqCpu),
        limits: round1(limCpu),
        usage: round1((usage.cpuUsagePercent / 100) * usage.cpuCapacityCores),
      },
      memory: {
        unit: 'Gi',
        allocatable: round1(allocatableMemMi / 1024),
        requests: round1(reqMemMi / 1024),
        limits: round1(limMemMi / 1024),
        usage: round1((usage.memoryUsagePercent / 100) * usage.memoryCapacityGi),
      },
    }
  },

  'GET /api/metrics/overview': async () => {
    const [nodes, pods, namespaces] = await Promise.all([
      kube.getJSON('/api/v1/nodes'),
      kube.getJSON('/api/v1/pods'),
      kube.getJSON('/api/v1/namespaces'),
    ])
    return {
      nodeCount: nodes.items.length,
      readyNodes: nodes.items.map(mapNode).filter((n) => n.status === 'Ready').length,
      podCount: pods.items.length,
      runningPods: pods.items.filter((p) => p.status?.phase === 'Running').length,
      namespaceCount: namespaces.items.length,
      ...metrics.latestClusterUsage(),
    }
  },

  'DELETE /api/pods': async (q) => {
    await kube.delete(`/api/v1/namespaces/${q.get('ns')}/pods/${q.get('name')}`)
    return { ok: true }
  },
  'POST /api/deployments/scale': async (q) => {
    await kube.mergePatch(
      `/apis/apps/v1/namespaces/${q.get('ns')}/deployments/${q.get('name')}/scale`,
      { spec: { replicas: Number(q.get('replicas')) } },
    )
    return { ok: true }
  },
  'POST /api/deployments/restart': async (q) => {
    await kube.strategicPatch(`/apis/apps/v1/namespaces/${q.get('ns')}/deployments/${q.get('name')}`, {
      spec: {
        template: {
          metadata: {
            annotations: { 'kubectl.kubernetes.io/restartedAt': new Date().toISOString() },
          },
        },
      },
    })
    return { ok: true }
  },
}

const server = createServer(async (req, res) => {
  const url = new URL(req.url, 'http://localhost')
  const handler = routes[`${req.method} ${url.pathname}`]
  res.setHeader('Access-Control-Allow-Origin', '*')
  res.setHeader('Access-Control-Allow-Methods', 'GET,POST,DELETE,OPTIONS')
  if (req.method === 'OPTIONS') {
    res.writeHead(204).end()
    return
  }
  if (!handler) {
    res.writeHead(404, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: `no route for ${req.method} ${url.pathname}` }))
    return
  }
  try {
    const result = await handler(url.searchParams)
    res.writeHead(200, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify(result))
  } catch (err) {
    const status = err.status && Number.isInteger(err.status) ? err.status : 500
    res.writeHead(status, { 'Content-Type': 'application/json' })
    res.end(JSON.stringify({ error: err.message }))
    console.error(`[api] ${req.method} ${url.pathname} -> ${status}: ${err.message}`)
  }
})

server.listen(PORT, () => console.log(`[api] listening on http://localhost:${PORT}`))
