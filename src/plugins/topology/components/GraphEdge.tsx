import clsx from 'clsx'
import { memo } from 'react'
import { NODE_HEIGHT, NODE_WIDTH, type Point } from '../model/layout'
import type { EdgeKind } from '../model/types'

/** Id of the arrowhead marker (normal and highlighted). */
export const ARROW_ID = 'topo-arrow'
export const ARROW_ACTIVE_ID = 'topo-arrow-active'

interface GraphEdgeProps {
  id: string
  from: Point
  to: Point
  kind: EdgeKind
  active: boolean
}

/**
 * A curve between the facing sides of two nodes in neighbouring columns,
 * with the arrowhead at the referenced object.
 */
export const GraphEdge = memo(function GraphEdge({ id, from, to, kind, active }: GraphEdgeProps) {
  const forward = from.x <= to.x
  const sx = forward ? from.x + NODE_WIDTH : from.x
  const tx = forward ? to.x : to.x + NODE_WIDTH
  const sy = from.y + NODE_HEIGHT / 2
  const ty = to.y + NODE_HEIGHT / 2
  const bend = (tx - sx) / 2
  return (
    <path
      className={clsx('topo-edge', `topo-edge--${kind}`, active && 'is-active')}
      data-id={id}
      d={`M${sx} ${sy} C${sx + bend} ${sy} ${tx - bend} ${ty} ${tx} ${ty}`}
      markerEnd={`url(#${active ? ARROW_ACTIVE_ID : ARROW_ID})`}
    />
  )
})
