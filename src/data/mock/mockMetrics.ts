import type { ClusterStats, MetricKind, MetricPoint } from '../types'
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

interface MetricShape {
  baseline: () => number
  amplitude: () => number
  noise: number
  driftTotal: number
  min: number
  max: number
}

/** Value shapes per metric: cpu/memory/fs in %, network in MB/s. */
const SHAPES: Record<MetricKind, (rand: () => number) => MetricShape> = {
  cpu: (r) => ({ baseline: () => 32 + r() * 18, amplitude: () => 12 + r() * 8, noise: 7, driftTotal: 0, min: 3, max: 97 }),
  memory: (r) => ({ baseline: () => 45 + r() * 15, amplitude: () => 4 + r() * 3, noise: 2, driftTotal: 8, min: 3, max: 97 }),
  network: (r) => ({ baseline: () => 8 + r() * 12, amplitude: () => 5 + r() * 5, noise: 9, driftTotal: 0, min: 0.2, max: 58 }),
  fs: (r) => ({ baseline: () => 42 + r() * 18, amplitude: () => 1 + r() * 1.5, noise: 0.8, driftTotal: 3, min: 5, max: 96 }),
}

export function generateSeries(key: string, metric: MetricKind, minutes: number): MetricPoint[] {
  const rand = mulberry32(seedFor(`${key}:${metric}`))
  const shape = SHAPES[metric](rand)
  const baseline = shape.baseline()
  const amplitude = shape.amplitude()
  const phase = rand() * Math.PI * 2
  const drift = shape.driftTotal / minutes

  const points: MetricPoint[] = []
  for (let i = minutes - 1; i >= 0; i--) {
    const t = minutes - 1 - i
    const wave = Math.sin(phase + (t / minutes) * Math.PI * 3) * amplitude
    const noise = (rand() - 0.5) * shape.noise
    const value = Math.min(shape.max, Math.max(shape.min, baseline + wave + noise + drift * t))

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
