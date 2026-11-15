import type { BindingGraph, GraphEdge, GraphNode } from '../data/types'

/**
 * Layout of the three-column binding graph: subjects → bindings → roles.
 * Roles keep the server's order, bindings are grouped by their role, and
 * subjects sit at the mean position of their bindings, which keeps most
 * edges from crossing without a general graph layout.
 */

export const NODE_WIDTH = 250
export const NODE_HEIGHT = 24
const ROW_GAP = 6
const COLUMN_GAP = 110
const PADDING = 12
export const HEADER_HEIGHT = 24

export type Column = 0 | 1 | 2

export interface PlacedNode extends GraphNode {
  column: Column
  x: number
  y: number
}

export interface PlacedEdge extends GraphEdge {
  /** SVG path from the right edge of `from` to the left edge of `to`. */
  d: string
}

export interface GraphLayout {
  nodes: PlacedNode[]
  edges: PlacedEdge[]
  width: number
  height: number
  columnX: [number, number, number]
}

const mean = (values: number[]) => (values.length ? values.reduce((a, b) => a + b, 0) / values.length : Infinity)

function sortBy<T>(items: T[], key: (item: T) => number): T[] {
  return items
    .map((item, index) => ({ item, index, k: key(item) }))
    .sort((a, b) => a.k - b.k || a.index - b.index)
    .map((e) => e.item)
}

export function layoutGraph(graph: BindingGraph): GraphLayout {
  const roleIndex = new Map(graph.roles.map((r, i) => [r.id, i]))
  const roleOfBinding = new Map<string, string>()
  const bindingsOfSubject = new Map<string, string[]>()
  for (const e of graph.edges) {
    if (e.from.startsWith('b:')) roleOfBinding.set(e.from, e.to)
    else bindingsOfSubject.set(e.from, [...(bindingsOfSubject.get(e.from) ?? []), e.to])
  }

  const bindings = sortBy(graph.bindings, (b) => roleIndex.get(roleOfBinding.get(b.id) ?? '') ?? Infinity)
  const bindingIndex = new Map(bindings.map((b, i) => [b.id, i]))
  const subjects = sortBy(graph.subjects, (s) =>
    mean((bindingsOfSubject.get(s.id) ?? []).map((id) => bindingIndex.get(id) ?? 0)),
  )

  const columnX: [number, number, number] = [
    PADDING,
    PADDING + NODE_WIDTH + COLUMN_GAP,
    PADDING + 2 * (NODE_WIDTH + COLUMN_GAP),
  ]
  const nodes: PlacedNode[] = []
  const place = (list: GraphNode[], column: Column) =>
    list.forEach((n, i) =>
      nodes.push({ ...n, column, x: columnX[column], y: PADDING + HEADER_HEIGHT + i * (NODE_HEIGHT + ROW_GAP) }),
    )
  place(subjects, 0)
  place(bindings, 1)
  place(graph.roles, 2)

  const byId = new Map(nodes.map((n) => [n.id, n]))
  const edges: PlacedEdge[] = []
  for (const e of graph.edges) {
    const from = byId.get(e.from)
    const to = byId.get(e.to)
    if (!from || !to) continue
    const x1 = from.x + NODE_WIDTH
    const y1 = from.y + NODE_HEIGHT / 2
    const x2 = to.x
    const y2 = to.y + NODE_HEIGHT / 2
    const mid = (x1 + x2) / 2
    edges.push({ ...e, d: `M${x1},${y1} C${mid},${y1} ${mid},${y2} ${x2},${y2}` })
  }

  const rows = Math.max(subjects.length, bindings.length, graph.roles.length, 1)
  return {
    nodes,
    edges,
    columnX,
    width: columnX[2] + NODE_WIDTH + PADDING,
    height: PADDING * 2 + HEADER_HEIGHT + rows * (NODE_HEIGHT + ROW_GAP),
  }
}

export const edgeKey = (e: GraphEdge) => `${e.from}>${e.to}`

/**
 * The nodes and edges (by edgeKey) connected to `id` along subject → binding → role
 * chains: a subject lights up its bindings and their roles, a role its
 * bindings and their subjects, a binding both sides.
 */
export function connected(graph: BindingGraph, id: string): { nodes: Set<string>; edges: Set<string> } {
  const nodes = new Set([id])
  const edges = new Set<string>()
  const walk = (start: string, forward: boolean) => {
    const queue = [start]
    while (queue.length) {
      const current = queue.shift() as string
      for (const e of graph.edges) {
        const [here, there] = forward ? [e.from, e.to] : [e.to, e.from]
        if (here !== current) continue
        edges.add(edgeKey(e))
        if (!nodes.has(there)) {
          nodes.add(there)
          queue.push(there)
        }
      }
    }
  }
  walk(id, true)
  walk(id, false)
  return { nodes, edges }
}
