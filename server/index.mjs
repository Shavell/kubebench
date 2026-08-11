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
} from './mappers.mjs'

const PORT = Number(process.env.PORT ?? 9876)

const kube = createKubeClient()
const metrics = startMetricsSampler(kube)
console.log(`[kube] context=${kube.contextName} server=${kube.server}`)

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

  'GET /api/metrics/cluster': async (q) =>
    metrics.clusterSeries(q.get('metric'), Number(q.get('minutes') ?? 60)),
  'GET /api/metrics/node': async (q) =>
    metrics.nodeSeries(q.get('node'), q.get('metric'), Number(q.get('minutes') ?? 60)),
  'GET /api/metrics/pod': async (q) =>
    metrics.podSeries(q.get('ns'), q.get('pod'), q.get('metric'), Number(q.get('minutes') ?? 60)),
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
