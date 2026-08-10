import type { ClusterStats, MetricPoint } from '../types'
import { mulberry32 } from './rng'
import { nodes, pods, namespaces } from './mockCluster'

/**
 * Deterministic time series: baseline + sine wave + seeded noise per
 * (node, metric) pair. A fixed "now" keeps hot reloads stable.
 */
const BASE_HOUR = 14
const BASE_MINUTE = 30

function seedFor(key: string): number {
  let hash = 2166136261
  for (let i = 0; i < key.length; i++) {
    hash ^= key.charCodeAt(i)
    hash = Math.imul(hash, 16777619)
  }
  return hash >>> 0
}

export function generateSeries(key: string, metric: 'cpu' | 'memory', minutes: number): MetricPoint[] {
  const rand = mulberry32(seedFor(`${key}:${metric}`))
  const baseline = metric === 'cpu' ? 32 + rand() * 18 : 45 + rand() * 15
  const amplitude = metric === 'cpu' ? 12 + rand() * 8 : 4 + rand() * 3
  const phase = rand() * Math.PI * 2
  const drift = metric === 'memory' ? 8 / minutes : 0

  const points: MetricPoint[] = []
  for (let i = minutes - 1; i >= 0; i--) {
    const t = minutes - 1 - i
    const wave = Math.sin(phase + (t / minutes) * Math.PI * 3) * amplitude
    const noise = (rand() - 0.5) * (metric === 'cpu' ? 7 : 2)
    const value = Math.min(97, Math.max(3, baseline + wave + noise + drift * t))

    const totalMinutes = BASE_HOUR * 60 + BASE_MINUTE - i
    const hh = String(Math.floor(totalMinutes / 60)).padStart(2, '0')
    const mm = String(totalMinutes % 60).padStart(2, '0')
    points.push({ time: `${hh}:${mm}`, value: Math.round(value * 10) / 10 })
  }
  return points
}

export function clusterStats(): ClusterStats {
  const cpuSeries = generateSeries('cluster', 'cpu', 5)
  const memSeries = generateSeries('cluster', 'memory', 5)
  return {
    nodeCount: nodes.length,
    readyNodes: nodes.filter((n) => n.status === 'Ready').length,
    podCount: pods.length,
    runningPods: pods.filter((p) => p.status === 'Running').length,
    namespaceCount: namespaces.length,
    cpuUsagePercent: cpuSeries[cpuSeries.length - 1].value,
    memoryUsagePercent: memSeries[memSeries.length - 1].value,
    cpuCapacityCores: nodes.reduce((sum, n) => sum + n.cpuCapacity, 0),
    memoryCapacityGi: nodes.reduce((sum, n) => sum + n.memoryCapacityGi, 0),
  }
}
