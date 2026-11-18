/**
 * The interface scale (Settings → Appearance) is CSS `zoom` on <body>.
 * Under it, the browser reports pointer coordinates and
 * `getBoundingClientRect()` in viewport pixels, while `style.left`,
 * `offsetWidth`, `scrollTop` and friends are in the page's own CSS pixels,
 * which are `zoom` viewport pixels each. Code that mixes the two converts
 * with these helpers. (Monaco divides by the zoom of the elements between
 * it and the root, which is why it is set on <body>, not <html>.)
 */

/** The interface scale in effect (1 = 100%). */
export function uiZoom(): number {
  const zoom = Number.parseFloat(getComputedStyle(document.body).zoom)
  return Number.isFinite(zoom) && zoom > 0 ? zoom : 1
}

export interface CssRect {
  left: number
  top: number
  right: number
  bottom: number
  width: number
  height: number
}

/** `element.getBoundingClientRect()` in the page's CSS pixels. */
export function cssRect(element: Element): CssRect {
  const zoom = uiZoom()
  const r = element.getBoundingClientRect()
  return {
    left: r.left / zoom,
    top: r.top / zoom,
    right: r.right / zoom,
    bottom: r.bottom / zoom,
    width: r.width / zoom,
    height: r.height / zoom,
  }
}

/** The window's size in the page's CSS pixels. */
export function cssViewport(): { width: number; height: number } {
  const zoom = uiZoom()
  return { width: window.innerWidth / zoom, height: window.innerHeight / zoom }
}
