import type { TopologyEdge, TopologyGraph } from './types'

/** Outgoing and incoming edges of every node. */
export interface Adjacency {
  out: Map<string, TopologyEdge[]>
  in: Map<string, TopologyEdge[]>
}

export function adjacency(edges: TopologyEdge[]): Adjacency {
  const out = new Map<string, TopologyEdge[]>()
  const inc = new Map<string, TopologyEdge[]>()
  const push = (map: Map<string, TopologyEdge[]>, key: string, e: TopologyEdge) => {
    const list = map.get(key)
    if (list) list.push(e)
    else map.set(key, [e])
  }
  for (const e of edges) {
    push(out, e.from, e)
    push(inc, e.to, e)
  }
  return { out, in: inc }
}

function walk(start: Iterable<string>, next: (id: string) => string[]): Set<string> {
  const seen = new Set<string>()
  const queue = [...start]
  while (queue.length > 0) {
    const id = queue.pop()!
    for (const n of next(id)) {
      if (!seen.has(n)) {
        seen.add(n)
        queue.push(n)
      }
    }
  }
  return seen
}

/**
 * The objects related to `id`: everything it leads to (a Deployment's
 * ReplicaSets, Pods and their volumes and config), and everything leading
 * to it or to its Pods (the Services selecting them, the Ingresses routing
 * there, their owners). Parents of shared leaves such as a ConfigMap many
 * workloads read are not followed, so a focus stays on its own workload.
 */
export function relatedIds(adj: Adjacency, id: string): Set<string> {
  const down = walk([id], (n) => (adj.out.get(n) ?? []).map((e) => e.to))
  const anchors = [id, ...[...down].filter((n) => n.startsWith('Pod/'))]
  const up = walk(anchors, (n) => (adj.in.get(n) ?? []).map((e) => e.from))
  return new Set([id, ...down, ...up])
}

/** The part of the graph made of `ids` and the edges between them. */
export function subgraph(graph: TopologyGraph, ids: Set<string>): TopologyGraph {
  return {
    nodes: graph.nodes.filter((n) => ids.has(n.id)),
    edges: graph.edges.filter((e) => ids.has(e.from) && ids.has(e.to)),
  }
}
