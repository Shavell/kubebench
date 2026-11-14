/**
 * Chart colors as references to the theme's `--vscode-*` tokens
 * (src/core/theme/theme.css). SVG presentation attributes and inline
 * styles resolve `var()` themselves, so charts follow a dark/light theme
 * switch without re-rendering. Charts are single-series by design (small
 * multiples instead of multi-series lines), so no categorical palette is
 * needed.
 */
export type ChartToken = 'blue' | 'purple' | 'green' | 'yellow' | 'red'

export function chartColor(token: ChartToken): string {
  return `var(--vscode-charts-${token})`
}

export const CHART_TEXT = 'var(--vscode-descriptionForeground)'
export const CHART_GRID = 'var(--vscode-charts-gridLines)'
