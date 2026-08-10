/**
 * Chart colors resolved from the VSCode Dark+ CSS variables so charts and
 * chrome always agree. Charts are single-series by design (small multiples
 * instead of multi-series lines), so no categorical palette is needed.
 */
export function chartColor(token: 'blue' | 'purple' | 'green' | 'yellow' | 'red'): string {
  const value = getComputedStyle(document.documentElement)
    .getPropertyValue(`--vscode-charts-${token}`)
    .trim()
  return value || '#3794ff'
}

export const CHART_TEXT = '#9d9d9d'
export const CHART_GRID = 'rgba(128, 128, 128, 0.15)'
