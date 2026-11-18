import type { ITheme } from '@xterm/xterm'

/**
 * The CSS zoom of the element xterm.js opens into, cancelling the
 * interface scale (CSS zoom of <body>, `scale` = 1.3 at 130%): zoomed,
 * xterm.js places selections and mouse clicks on the wrong cells, since it
 * compares viewport-pixel pointer positions with cell sizes in unzoomed
 * pixels. xtermOptions puts the scale into the font size instead.
 */
export function screenZoom(scale: number): string {
  return scale === 1 ? '' : String(1 / scale)
}

/** The xterm.js options of a terminal appearance at interface scale `scale`. */
export function xtermOptions({ fontSize, fontFamily, theme, scale }: {
  fontSize: number
  fontFamily: string
  theme: ITheme
  scale: number
}): { fontSize: number; fontFamily: string; theme: ITheme } {
  return { fontSize: fontSize * scale, fontFamily, theme }
}
