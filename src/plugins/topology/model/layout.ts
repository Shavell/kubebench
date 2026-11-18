import { adjacency, type Adjacency } from './focus'
import type { TopologyGraph, TopologyKind, TopologyNode } from './types'

export const NODE_WIDTH = 200
export const NODE_HEIGHT = 36
const ROW_GAP = 10
/** Extra space between the Pods of different owners. */
const GROUP_GAP = 14
const COLUMN_GAP = 96
/** How far below its first neighbour a node may be centred, in rows. */
const MAX_ANCHOR_ROWS = 3

/**
 * Columns, left to right: what runs the Pods, the Pods, what the Pods are
 * reached by or use, and what that in turn leads to. Every edge joins
 * neighbouring columns.
 */
const COLUMN: Record<TopologyKind, number> = {
  Deployment: 0,
  ReplicaSet: 1,
  StatefulSet: 1,
  DaemonSet: 1,
  Pod: 2,
  Service: 3,
  PersistentVolumeClaim: 3,
  ConfigMap: 3,
  Secret: 3,
  Ingress: 4,
  Route: 4,
  PersistentVolume: 4,
}

/** Placement order: the Pods, then outward from them. */
const PLACEMENT_ORDER = [2, 1, 0, 3, 4]

const KIND_ORDER: TopologyKind[] = [
  'Deployment',
  'StatefulSet',
  'DaemonSet',
  'ReplicaSet',
  'Pod',
  'Service',
  'PersistentVolumeClaim',
  'ConfigMap',
  'Secret',
  'Ingress',
  'Route',
  'PersistentVolume',
]

export interface Point {
  x: number
  y: number
}

export interface Layout {
  /** Top-left corner of every node. */
  positions: Map<string, Point>
  width: number
  height: number
}

const byKindThenName = (a: TopologyNode, b: TopologyNode) =>
  KIND_ORDER.indexOf(a.kind) - KIND_ORDER.indexOf(b.kind) || a.name.localeCompare(b.name)

/** The owner chain of a Pod as a sort key: its root owner, its direct owner, its name. */
function podSortKey(adj: Adjacency, pod: TopologyNode): string[] {
  const ownerOf = (id: string) => adj.in.get(id)?.find((e) => e.kind === 'owns')?.from
  const owner = ownerOf(pod.id)
  const root = owner ? (ownerOf(owner) ?? owner) : undefined
  // '￿' sorts Pods without an owner after the owned ones.
  return [root ?? '￿', owner ?? '', pod.name]
}

function compareKeys(a: string[], b: string[]): number {
  for (let i = 0; i < a.length; i++) {
    const c = a[i].localeCompare(b[i])
    if (c !== 0) return c
  }
  return 0
}

function anchorY(neighbours: Point[]): number {
  const mean = neighbours.reduce((sum, p) => sum + p.y, 0) / neighbours.length
  const first = Math.min(...neighbours.map((p) => p.y))
  return Math.min(mean, first + MAX_ANCHOR_ROWS * (NODE_HEIGHT + ROW_GAP))
}

/**
 * A layered layout (design §7.9). Pods are stacked in owner groups; every
 * other node is placed level with the mean of its already placed
 * neighbours (at most a few rows below the first, so the owner of many Pods
 * stays at the top of its group), pushed down where it would overlap, and
 * nodes without placed neighbours go below in kind and name order.
 * Deterministic: the same
 * graph always gets the same positions, and a Pod coming or going only
 * shifts the nodes below it.
 */
export function layoutGraph(graph: TopologyGraph): Layout {
  const adj = adjacency(graph.edges)
  const columns = new Map<number, TopologyNode[]>()
  for (const node of graph.nodes) {
    const col = COLUMN[node.kind]
    const list = columns.get(col)
    if (list) list.push(node)
    else columns.set(col, [node])
  }
  // Empty columns take no space.
  const xOf = new Map([...columns.keys()].sort((a, b) => a - b).map((col, i) => [col, i * (NODE_WIDTH + COLUMN_GAP)]))

  const positions = new Map<string, Point>()
  let height = 0
  const place = (node: TopologyNode, col: number, y: number) => {
    positions.set(node.id, { x: xOf.get(col)!, y })
    height = Math.max(height, y + NODE_HEIGHT)
  }

  const pods = (columns.get(COLUMN.Pod) ?? [])
    .map((pod) => ({ pod, key: podSortKey(adj, pod) }))
    .sort((a, b) => compareKeys(a.key, b.key))
  let y = 0
  pods.forEach(({ pod, key }, i) => {
    if (i > 0 && (key[0] !== pods[i - 1].key[0] || key[1] !== pods[i - 1].key[1])) y += GROUP_GAP
    place(pod, COLUMN.Pod, y)
    y += NODE_HEIGHT + ROW_GAP
  })

  for (const col of PLACEMENT_ORDER.slice(1)) {
    const nodes = columns.get(col) ?? []
    const anchored: { node: TopologyNode; want: number }[] = []
    const free: TopologyNode[] = []
    for (const node of nodes) {
      const neighbours = [
        ...(adj.out.get(node.id) ?? []).map((e) => e.to),
        ...(adj.in.get(node.id) ?? []).map((e) => e.from),
      ]
        .map((id) => positions.get(id))
        .filter((p): p is Point => p !== undefined)
      if (neighbours.length === 0) free.push(node)
      else anchored.push({ node, want: anchorY(neighbours) })
    }
    anchored.sort((a, b) => a.want - b.want || a.node.id.localeCompare(b.node.id))
    let next = 0
    for (const { node, want } of anchored) {
      const top = Math.max(want, next)
      place(node, col, top)
      next = top + NODE_HEIGHT + ROW_GAP
    }
    if (anchored.length > 0 && free.length > 0) next += GROUP_GAP
    for (const node of free.sort(byKindThenName)) {
      place(node, col, next)
      next += NODE_HEIGHT + ROW_GAP
    }
  }

  const width = columns.size === 0 ? 0 : (columns.size - 1) * (NODE_WIDTH + COLUMN_GAP) + NODE_WIDTH
  return { positions, width, height }
}
