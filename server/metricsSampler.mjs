import { parseCpu, parseMemoryMi } from './mappers.mjs'

const SAMPLE_INTERVAL_MS = 15_000
const MAX_SAMPLES = 240 // 1h at 15s

/**
 * metrics.k8s.io only exposes instant usage, so we poll it and keep an
 * in-memory ring buffer to serve "last N minutes" series while the
 * server runs. History starts accumulating at server start.
 */
export function startMetricsSampler(kube) {
  const samples = [] // { time: "HH:MM", nodes: Map, pods: Map, clusterCpu, clusterMem }
  let capacities = { cpuCores: 0, memoryMi: 0, perNode: new Map() }

  async function refreshCapacities() {
    const nodeList = await kube.getJSON('/api/v1/nodes')
    const perNode = new Map()
    let cpuCores = 0
    let memoryMi = 0
    for (const n of nodeList.items) {
      const cpu = parseCpu(n.status.capacity?.cpu)
      const mem = parseMemoryMi(n.status.capacity?.memory)
      perNode.set(n.metadata.name, { cpu, mem })
      cpuCores += cpu
      memoryMi += mem
    }
    capacities = { cpuCores, memoryMi, perNode }
  }

  async function sample() {
    const now = new Date()
    const time = `${String(now.getHours()).padStart(2, '0')}:${String(now.getMinutes()).padStart(2, '0')}`

    const [nodeMetrics, podMetrics] = await Promise.all([
      kube.getJSON('/apis/metrics.k8s.io/v1beta1/nodes'),
      kube.getJSON('/apis/metrics.k8s.io/v1beta1/pods'),
    ])

    const nodes = new Map()
    let clusterCpu = 0
    let clusterMem = 0
    for (const m of nodeMetrics.items) {
      const cpu = parseCpu(m.usage?.cpu)
      const mem = parseMemoryMi(m.usage?.memory)
      nodes.set(m.metadata.name, { cpu, mem })
      clusterCpu += cpu
      clusterMem += mem
    }

    const pods = new Map()
    for (const m of podMetrics.items) {
      const cpu = (m.containers ?? []).reduce((s, c) => s + parseCpu(c.usage?.cpu), 0)
      const mem = (m.containers ?? []).reduce((s, c) => s + parseMemoryMi(c.usage?.memory), 0)
      pods.set(`${m.metadata.namespace}/${m.metadata.name}`, { cpu, mem })
    }

    samples.push({ time, nodes, pods, clusterCpu, clusterMem })
    if (samples.length > MAX_SAMPLES) samples.shift()
  }

  async function tick() {
    try {
      if (capacities.perNode.size === 0) await refreshCapacities()
      await sample()
    } catch (err) {
      console.error(`[metrics] sample failed: ${err.message}`)
    }
  }
  tick()
  const timer = setInterval(tick, SAMPLE_INTERVAL_MS)
  timer.unref?.()

  const pct = (value, total) => (total > 0 ? Math.round((value / total) * 1000) / 10 : 0)

  function series(minutes, pick) {
    const count = Math.max(1, Math.ceil((minutes * 60_000) / SAMPLE_INTERVAL_MS))
    return samples.slice(-count).map((s) => ({ time: s.time, value: pick(s) }))
  }

  return {
    stop: () => clearInterval(timer),
    clusterSeries: (metric, minutes) =>
      series(minutes, (s) =>
        metric === 'cpu' ? pct(s.clusterCpu, capacities.cpuCores) : pct(s.clusterMem, capacities.memoryMi),
      ),
    nodeSeries: (node, metric, minutes) =>
      series(minutes, (s) => {
        const usage = s.nodes.get(node)
        const cap = capacities.perNode.get(node)
        if (!usage || !cap) return 0
        return metric === 'cpu' ? pct(usage.cpu, cap.cpu) : pct(usage.mem, cap.mem)
      }),
    podSeries: (namespace, pod, metric, minutes) =>
      series(minutes, (s) => {
        const usage = s.pods.get(`${namespace}/${pod}`)
        if (!usage) return 0
        // Pod % is relative to a whole node's capacity (like Lens).
        const cap = capacities.perNode.values().next().value
        if (!cap) return 0
        return metric === 'cpu' ? pct(usage.cpu, cap.cpu) : pct(usage.mem, cap.mem)
      }),
    latestClusterUsage: () => {
      const last = samples[samples.length - 1]
      return {
        cpuUsagePercent: last ? pct(last.clusterCpu, capacities.cpuCores) : 0,
        memoryUsagePercent: last ? pct(last.clusterMem, capacities.memoryMi) : 0,
        cpuCapacityCores: Math.round(capacities.cpuCores),
        memoryCapacityGi: Math.round(capacities.memoryMi / 1024),
      }
    },
  }
}
