import type { FitAddon } from '@xterm/addon-fit'
import type { ITheme, Terminal } from '@xterm/xterm'

export interface XtermModules {
  Terminal: typeof Terminal
  FitAddon: typeof FitAddon
}

let loading: Promise<XtermModules> | null = null

/**
 * Loads xterm.js, its fit addon and its stylesheet on first use, as a
 * separate chunk, so the main bundle stays small. After a failed load
 * (server unreachable, stale deployment) the next call tries again.
 */
export function loadXterm(): Promise<XtermModules> {
  loading ??= Promise.all([
    import('@xterm/xterm'),
    import('@xterm/addon-fit'),
    import('@xterm/xterm/css/xterm.css'),
  ]).then(
    ([xterm, fit]) => ({ Terminal: xterm.Terminal, FitAddon: fit.FitAddon }),
    (error: unknown) => {
      loading = null
      throw error
    },
  )
  return loading
}

/** The colour tokens (theme.css) behind each colour of the terminal. */
const THEME_TOKENS = {
  background: '--vscode-terminal-background',
  foreground: '--vscode-terminal-foreground',
  cursor: '--vscode-terminalCursor-foreground',
  cursorAccent: '--vscode-terminal-background',
  selectionBackground: '--vscode-terminal-selectionBackground',
  scrollbarSliderBackground: '--vscode-scrollbarSlider-background',
  scrollbarSliderHoverBackground: '--vscode-scrollbarSlider-hoverBackground',
  scrollbarSliderActiveBackground: '--vscode-scrollbarSlider-activeBackground',
  black: '--vscode-terminal-ansiBlack',
  red: '--vscode-terminal-ansiRed',
  green: '--vscode-terminal-ansiGreen',
  yellow: '--vscode-terminal-ansiYellow',
  blue: '--vscode-terminal-ansiBlue',
  magenta: '--vscode-terminal-ansiMagenta',
  cyan: '--vscode-terminal-ansiCyan',
  white: '--vscode-terminal-ansiWhite',
  brightBlack: '--vscode-terminal-ansiBrightBlack',
  brightRed: '--vscode-terminal-ansiBrightRed',
  brightGreen: '--vscode-terminal-ansiBrightGreen',
  brightYellow: '--vscode-terminal-ansiBrightYellow',
  brightBlue: '--vscode-terminal-ansiBrightBlue',
  brightMagenta: '--vscode-terminal-ansiBrightMagenta',
  brightCyan: '--vscode-terminal-ansiBrightCyan',
  brightWhite: '--vscode-terminal-ansiBrightWhite',
} satisfies Partial<Record<keyof ITheme, string>>

/**
 * The terminal colours of the palette in effect, read from the tokens.
 * xterm.js paints with concrete colours, so call it again after a theme
 * change.
 */
export function xtermTheme(): ITheme {
  const style = getComputedStyle(document.documentElement)
  const theme: ITheme = {}
  for (const [key, token] of Object.entries(THEME_TOKENS)) {
    theme[key as keyof typeof THEME_TOKENS] = style.getPropertyValue(token).trim()
  }
  return theme
}

/** The UI's monospace font stack (`--font-mono`). */
export function monospaceFont(): string {
  return getComputedStyle(document.documentElement).getPropertyValue('--font-mono').trim()
}
