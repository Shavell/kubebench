import { useCallback, useRef } from 'react'
import { uiZoom } from './zoom'
import './ResizeHandle.css'

interface ResizeHandleProps {
  orientation: 'vertical' | 'horizontal'
  /** Called with the pointer delta since the last call, in CSS px. */
  onResize: (delta: number) => void
}

/**
 * Invisible drag strip on a container edge. `vertical` = a vertical strip
 * resizing width (sidebar), `horizontal` = a horizontal strip resizing
 * height (bottom panel).
 */
export function ResizeHandle({ orientation, onResize }: ResizeHandleProps) {
  const startPos = useRef(0)

  const onPointerDown = useCallback(
    (e: React.PointerEvent<HTMLDivElement>) => {
      startPos.current = orientation === 'vertical' ? e.clientX : e.clientY
      const target = e.currentTarget
      target.setPointerCapture(e.pointerId)

      const onMove = (ev: PointerEvent) => {
        const pos = orientation === 'vertical' ? ev.clientX : ev.clientY
        onResize((pos - startPos.current) / uiZoom())
        startPos.current = pos
      }
      const onUp = () => {
        target.removeEventListener('pointermove', onMove)
        target.removeEventListener('pointerup', onUp)
      }
      target.addEventListener('pointermove', onMove)
      target.addEventListener('pointerup', onUp)
    },
    [orientation, onResize],
  )

  return (
    <div
      className={`resize-handle resize-handle--${orientation}`}
      onPointerDown={onPointerDown}
    />
  )
}
