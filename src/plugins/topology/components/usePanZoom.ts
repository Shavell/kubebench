import { useCallback, useEffect, useMemo, useRef, type RefObject } from 'react'
import { cssRect, uiZoom } from '@/shared/zoom'

export interface Viewport {
  x: number
  y: number
  k: number
}

export interface Bounds {
  width: number
  height: number
}

const MIN_ZOOM = 0.1
const MAX_ZOOM = 2.5
/** Fitting never shrinks below this: tall graphs start readable, top-aligned. */
const MIN_FIT_ZOOM = 0.4
const PADDING = 24
/** Pointer travel (px) after which a press is a pan, not a click. */
const DRAG_THRESHOLD = 4

const clamp = (k: number) => Math.min(MAX_ZOOM, Math.max(MIN_ZOOM, k))

/**
 * Pan and zoom of an SVG's content group, applied to the DOM directly so a
 * gesture never re-renders the graph: dragging pans, the wheel pans
 * (Ctrl/⌘ + wheel or a pinch zooms at the pointer). `wasDrag()` tells a
 * click handler that the press it ends was a pan.
 */
export function usePanZoom(svgRef: RefObject<SVGSVGElement | null>, viewRef: RefObject<SVGGElement | null>) {
  const viewport = useRef<Viewport>({ x: PADDING, y: PADDING, k: 1 })
  const dragged = useRef(false)
  const interacted = useRef(false)

  const apply = useCallback(() => {
    const { x, y, k } = viewport.current
    viewRef.current?.setAttribute('transform', `translate(${x} ${y}) scale(${k})`)
  }, [viewRef])

  /** Zooms by `factor` keeping the point (px, py) of the SVG where it is. */
  const zoomAt = useCallback(
    (factor: number, px: number, py: number) => {
      const v = viewport.current
      const k = clamp(v.k * factor)
      viewport.current = { k, x: px - ((px - v.x) * k) / v.k, y: py - ((py - v.y) * k) / v.k }
      interacted.current = true
      apply()
    },
    [apply],
  )

  const zoomBy = useCallback(
    (factor: number) => {
      const rect = svgRef.current ? cssRect(svgRef.current) : undefined
      zoomAt(factor, (rect?.width ?? 0) / 2, (rect?.height ?? 0) / 2)
    },
    [svgRef, zoomAt],
  )

  /**
   * Shows the whole graph (centred, at most 1:1); a tall one stays readable
   * and starts at the top. Does nothing while the SVG has no size.
   */
  const fit = useCallback(
    (bounds: Bounds) => {
      const rect = svgRef.current ? cssRect(svgRef.current) : undefined
      if (!rect || rect.width === 0 || bounds.width === 0) return
      const fitK = Math.min(
        1,
        (rect.width - 2 * PADDING) / bounds.width,
        (rect.height - 2 * PADDING) / Math.max(bounds.height, 1),
      )
      const k = Math.max(MIN_FIT_ZOOM, fitK)
      const x = Math.max(PADDING, (rect.width - bounds.width * k) / 2)
      const y = Math.max(PADDING, (rect.height - bounds.height * k) / 2)
      viewport.current = { x, y, k }
      interacted.current = false
      apply()
    },
    [svgRef, apply],
  )

  useEffect(apply, [apply])

  useEffect(() => {
    const svg = svgRef.current
    if (!svg) return
    let start: { x: number; y: number; vx: number; vy: number; id: number } | null = null

    const onWheel = (e: WheelEvent) => {
      e.preventDefault()
      if (e.ctrlKey || e.metaKey) {
        const rect = cssRect(svg)
        const zoom = uiZoom()
        zoomAt(Math.exp(-e.deltaY * 0.002), e.clientX / zoom - rect.left, e.clientY / zoom - rect.top)
        return
      }
      const v = viewport.current
      viewport.current = { ...v, x: v.x - e.deltaX, y: v.y - e.deltaY }
      interacted.current = true
      apply()
    }
    const onDown = (e: PointerEvent) => {
      if (e.button !== 0) return
      dragged.current = false
      start = { x: e.clientX, y: e.clientY, vx: viewport.current.x, vy: viewport.current.y, id: e.pointerId }
    }
    const onMove = (e: PointerEvent) => {
      if (!start || e.pointerId !== start.id) return
      // Pointer moves are in viewport pixels; the graph is laid out in CSS pixels.
      const zoom = uiZoom()
      const dx = (e.clientX - start.x) / zoom
      const dy = (e.clientY - start.y) / zoom
      if (!dragged.current) {
        if (Math.hypot(dx, dy) < DRAG_THRESHOLD) return
        // Captured only once it is a pan, so a plain click still reaches the node under it.
        dragged.current = true
        svg.setPointerCapture(e.pointerId)
        svg.classList.add('is-panning')
      }
      viewport.current = { ...viewport.current, x: start.vx + dx, y: start.vy + dy }
      interacted.current = true
      apply()
    }
    const onUp = (e: PointerEvent) => {
      if (!start || e.pointerId !== start.id) return
      start = null
      svg.classList.remove('is-panning')
      if (svg.hasPointerCapture(e.pointerId)) svg.releasePointerCapture(e.pointerId)
    }

    svg.addEventListener('wheel', onWheel, { passive: false })
    svg.addEventListener('pointerdown', onDown)
    svg.addEventListener('pointermove', onMove)
    svg.addEventListener('pointerup', onUp)
    svg.addEventListener('pointercancel', onUp)
    return () => {
      svg.removeEventListener('wheel', onWheel)
      svg.removeEventListener('pointerdown', onDown)
      svg.removeEventListener('pointermove', onMove)
      svg.removeEventListener('pointerup', onUp)
      svg.removeEventListener('pointercancel', onUp)
    }
  }, [svgRef, apply, zoomAt])

  return useMemo(
    () => ({
      zoomBy,
      fit,
      /** The user moved or zoomed the view since the last fit. */
      hasInteracted: () => interacted.current,
      /** The press that ends with this click was a pan. */
      wasDrag: () => dragged.current,
    }),
    [zoomBy, fit],
  )
}
