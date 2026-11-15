import clsx from 'clsx'
import { useMemo, useState, type KeyboardEvent } from 'react'
import { useTranslation } from 'react-i18next'
import type { BindingGraph, Subject } from '../data/types'
import { NS } from '../i18n'
import { connected, edgeKey, HEADER_HEIGHT, layoutGraph, NODE_HEIGHT, NODE_WIDTH, type PlacedNode } from '../model/graphLayout'
import { openResource, openSubject } from '../tabs'

/** Longest label drawn in a node; the full text is in its tooltip. */
const MAX_LABEL = 31

const clip = (text: string) => (text.length > MAX_LABEL ? `${text.slice(0, MAX_LABEL - 1)}…` : text)

const KIND_SHORT: Record<string, string> = {
  User: 'U',
  Group: 'G',
  ServiceAccount: 'SA',
  RoleBinding: 'RB',
  ClusterRoleBinding: 'CRB',
  Role: 'R',
  ClusterRole: 'CR',
}

function open(node: PlacedNode) {
  if (node.column === 0) openSubject({ kind: node.kind as Subject['kind'], name: node.name, namespace: node.namespace })
  else if (!node.missing) openResource(node.kind, node.namespace, node.name)
}

/**
 * The binding graph as SVG in three columns. Hovering or focusing a node
 * highlights its chain; clicking a subject opens its permissions, a
 * binding or role its resource view.
 */
export function BindingGraphView({ graph }: { graph: BindingGraph }) {
  const { t } = useTranslation(NS)
  const layout = useMemo(() => layoutGraph(graph), [graph])
  const [active, setActive] = useState<string | null>(null)
  const lit = useMemo(() => (active ? connected(graph, active) : null), [graph, active])

  const onKey = (node: PlacedNode) => (event: KeyboardEvent) => {
    if (event.key === 'Enter' || event.key === ' ') {
      event.preventDefault()
      open(node)
    }
  }

  const headers = [t('graph.columns.subjects'), t('graph.columns.bindings'), t('graph.columns.roles')]
  return (
    <svg
      className="access-graph"
      width={layout.width}
      height={layout.height}
      viewBox={`0 0 ${layout.width} ${layout.height}`}
      role="img"
      aria-label={t('graph.aria', { subjects: graph.subjects.length, bindings: graph.bindings.length, roles: graph.roles.length })}
    >
      {headers.map((label, i) => (
        <text key={label} className="access-graph__header" x={layout.columnX[i]} y={HEADER_HEIGHT - 8}>
          {label}
        </text>
      ))}
      <g>
        {layout.edges.map((e) => (
          <path
            key={edgeKey(e)}
            d={e.d}
            className={clsx('access-graph__edge', lit && (lit.edges.has(edgeKey(e)) ? 'is-lit' : 'is-dim'))}
          />
        ))}
      </g>
      {layout.nodes.map((node) => {
        const label = node.namespace ? `${node.namespace}/${node.name}` : node.name
        const title = `${node.kind} ${label}${node.missing ? ` — ${t(node.column === 0 ? 'graph.missingSubject' : 'graph.missingRole')}` : ''}`
        return (
          <g
            key={node.id}
            transform={`translate(${node.x},${node.y})`}
            className={clsx(
              'access-graph__node',
              `access-graph__node--c${node.column}`,
              node.missing && 'is-missing',
              lit && (lit.nodes.has(node.id) ? 'is-lit' : 'is-dim'),
            )}
            role="button"
            tabIndex={0}
            aria-label={title}
            onMouseEnter={() => setActive(node.id)}
            onMouseLeave={() => setActive(null)}
            onFocus={() => setActive(node.id)}
            onBlur={() => setActive(null)}
            onClick={() => open(node)}
            onKeyDown={onKey(node)}
          >
            <title>{title}</title>
            <rect width={NODE_WIDTH} height={NODE_HEIGHT} rx={4} />
            <text className="access-graph__kind" x={8} y={NODE_HEIGHT / 2 + 4}>
              {KIND_SHORT[node.kind] ?? node.kind}
            </text>
            <text className="access-graph__label" x={40} y={NODE_HEIGHT / 2 + 4}>
              {clip(label)}
            </text>
          </g>
        )
      })}
    </svg>
  )
}
