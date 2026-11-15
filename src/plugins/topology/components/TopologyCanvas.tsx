import clsx from 'clsx'
import { useEffect, useMemo, useRef, useState, type MouseEvent } from 'react'
import { useTranslation } from 'react-i18next'
import { Button } from '@/shared/Button'
import { showContextMenu } from '@/shared/contextMenu'
import { adjacency, relatedIds } from '../model/focus'
import type { Layout } from '../model/layout'
import type { TopologyGraph, TopologyNode } from '../model/types'
import { NS } from '../i18n'
import { FOCUSABLE_KINDS, openObject, openObjectYaml, openTopology } from '../tabs'
import { ARROW_ACTIVE_ID, ARROW_ID, GraphEdge } from './GraphEdge'
import { GraphNode, type NodeHandlers } from './GraphNode'
import { usePanZoom } from './usePanZoom'

const ZOOM_STEP = 1.25

interface TopologyCanvasProps {
  namespace: string
  graph: TopologyGraph
  layout: Layout
  /** The node the view is focused on, outlined. */
  focusId: string | null
}

/**
 * The graph as SVG with pan and zoom. Hovering (or focusing) a node
 * highlights its chain of related objects; clicking opens its object, the
 * context menu offers the YAML and a topology focused on it.
 */
export function TopologyCanvas({ namespace, graph, layout, focusId }: TopologyCanvasProps) {
  const { t } = useTranslation(NS)
  const svgRef = useRef<SVGSVGElement>(null)
  const viewRef = useRef<SVGGElement>(null)
  const panZoom = usePanZoom(svgRef, viewRef)
  const [hovered, setHovered] = useState<string | null>(null)

  // The view fits the graph as it fills in, and when the tab is shown again
  // or resized, until the user pans or zooms it.
  useEffect(() => {
    const svg = svgRef.current
    if (!svg) return
    const refit = () => {
      if (!panZoom.hasInteracted()) panZoom.fit(layout)
    }
    refit()
    const observer = new ResizeObserver(refit)
    observer.observe(svg)
    return () => observer.disconnect()
  }, [layout, panZoom])

  const adj = useMemo(() => adjacency(graph.edges), [graph])
  // A hovered node that left the graph (refresh, option change) highlights nothing.
  const hoveredId = hovered && layout.positions.has(hovered) ? hovered : null
  const active = useMemo(() => (hoveredId ? relatedIds(adj, hoveredId) : null), [adj, hoveredId])

  const labels = useMemo(
    () =>
      new Map(
        graph.nodes.map((n) => {
          const status = t(n.status.key, n.status.values ?? {})
          return [n.id, { status, kind: t(`kinds.short.${n.kind}`), label: t('node.label', { kind: n.kind, name: n.name, status }) }]
        }),
      ),
    [graph, t],
  )

  const handlers = useMemo<NodeHandlers>(
    () => ({
      onHover: setHovered,
      onOpen: (node) => {
        if (!panZoom.wasDrag()) openObject(node)
      },
      onMenu: (e: MouseEvent, node: TopologyNode) => {
        if (node.missing) return
        showContextMenu(e, [
          { label: t('menu.open'), icon: 'go-to-file', onClick: () => openObject(node) },
          { label: t('menu.yaml'), icon: 'file-code', onClick: () => openObjectYaml(node) },
          ...(FOCUSABLE_KINDS.includes(node.kind) && node.id !== focusId
            ? [{ label: t('menu.focus'), icon: 'type-hierarchy', onClick: () => openTopology(namespace, { kind: node.kind, name: node.name }) }]
            : []),
        ])
      },
    }),
    [t, panZoom, namespace, focusId],
  )

  return (
    <div className="topo-canvas">
      <svg
        ref={svgRef}
        className={clsx('topo-canvas__svg', active && 'is-hovering')}
        role="group"
        aria-label={t('canvas.label', { namespace })}
      >
        <defs>
          {[ARROW_ID, ARROW_ACTIVE_ID].map((id) => (
            <marker
              key={id}
              id={id}
              className={clsx('topo-arrow', id === ARROW_ACTIVE_ID && 'is-active')}
              viewBox="0 0 10 10"
              refX={9}
              refY={5}
              markerWidth={7}
              markerHeight={7}
              orient="auto-start-reverse"
            >
              <path d="M0 0 L10 5 L0 10 z" />
            </marker>
          ))}
        </defs>
        <g ref={viewRef}>
          <g className="topo-edges">
            {graph.edges.map((e) => {
              const from = layout.positions.get(e.from)
              const to = layout.positions.get(e.to)
              if (!from || !to) return null
              return <GraphEdge key={e.id} id={e.id} from={from} to={to} kind={e.kind} active={!!active?.has(e.from) && active.has(e.to)} />
            })}
          </g>
          <g className="topo-nodes">
            {graph.nodes.map((n) => {
              const p = layout.positions.get(n.id)
              const text = labels.get(n.id)
              if (!p || !text) return null
              return (
                <GraphNode
                  key={n.id}
                  node={n}
                  x={p.x}
                  y={p.y}
                  active={!!active?.has(n.id)}
                  focused={n.id === focusId}
                  kindLabel={text.kind}
                  statusText={text.status}
                  label={text.label}
                  handlers={handlers}
                />
              )
            })}
          </g>
        </g>
      </svg>
      <div className="topo-canvas__controls">
        <Button variant="secondary" icon="zoom-in" title={t('canvas.zoomIn')} aria-label={t('canvas.zoomIn')} onClick={() => panZoom.zoomBy(ZOOM_STEP)} />
        <Button variant="secondary" icon="zoom-out" title={t('canvas.zoomOut')} aria-label={t('canvas.zoomOut')} onClick={() => panZoom.zoomBy(1 / ZOOM_STEP)} />
        <Button variant="secondary" icon="screen-full" title={t('canvas.fit')} aria-label={t('canvas.fit')} onClick={() => panZoom.fit(layout)} />
      </div>
    </div>
  )
}
