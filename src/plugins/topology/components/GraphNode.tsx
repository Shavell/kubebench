import clsx from 'clsx'
import { memo, type MouseEvent } from 'react'
import { NODE_HEIGHT, NODE_WIDTH } from '../model/layout'
import type { TopologyNode } from '../model/types'

/** Characters that fit on a line of the node at 12px. */
const NAME_CHARS = 28
const STATUS_CHARS = 22

/** Shortens in the middle: generated names differ at the end, owners at the start. */
function ellipsize(text: string, max: number): string {
  if (text.length <= max) return text
  const head = Math.ceil((max - 1) / 2)
  return `${text.slice(0, head)}…${text.slice(text.length - (max - 1 - head))}`
}

export interface NodeHandlers {
  onHover: (id: string | null) => void
  onOpen: (node: TopologyNode) => void
  onMenu: (e: MouseEvent, node: TopologyNode) => void
}

interface GraphNodeProps {
  node: TopologyNode
  x: number
  y: number
  /** On the hovered object's chain while something is hovered. */
  active: boolean
  focused: boolean
  /** kubectl short name of the kind, e.g. "deploy". */
  kindLabel: string
  statusText: string
  label: string
  handlers: NodeHandlers
}

/** One object: kind and status on top, name below, coloured by its status tone. */
export const GraphNode = memo(function GraphNode({
  node,
  x,
  y,
  active,
  focused,
  kindLabel,
  statusText,
  label,
  handlers,
}: GraphNodeProps) {
  const open = () => {
    if (!node.missing) handlers.onOpen(node)
  }
  return (
    <g
      className={clsx(
        'topo-node',
        `topo-node--${node.status.tone}`,
        active && 'is-active',
        focused && 'is-focused',
        node.missing && 'is-missing',
      )}
      transform={`translate(${x} ${y})`}
      data-id={node.id}
      role={node.missing ? 'img' : 'button'}
      tabIndex={node.missing ? -1 : 0}
      aria-label={label}
      onPointerEnter={() => handlers.onHover(node.id)}
      onPointerLeave={() => handlers.onHover(null)}
      onFocus={() => handlers.onHover(node.id)}
      onBlur={() => handlers.onHover(null)}
      onClick={open}
      onKeyDown={(e) => {
        if (e.key === 'Enter' || e.key === ' ') {
          e.preventDefault()
          open()
        }
      }}
      onContextMenu={(e) => handlers.onMenu(e, node)}
    >
      <title>{label}</title>
      <rect className="topo-node__box" width={NODE_WIDTH} height={NODE_HEIGHT} rx={3} />
      <rect className="topo-node__bar" width={4} height={NODE_HEIGHT} rx={1} />
      <text className="topo-node__kind" x={12} y={14}>
        {kindLabel}
      </text>
      <text className="topo-node__status" x={NODE_WIDTH - 8} y={14} textAnchor="end">
        {ellipsize(statusText, STATUS_CHARS)}
      </text>
      <text className="topo-node__name" x={12} y={29}>
        {ellipsize(node.name, NAME_CHARS)}
      </text>
    </g>
  )
})
