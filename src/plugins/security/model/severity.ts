import type { BadgeTone } from '@/shared/Badge'
import { chartColor } from '@/shared/charts/chartTheme'
import type { Severity } from '../data/types'

/** Badge tone of a severity. */
export const SEVERITY_TONES: Record<Severity, BadgeTone> = {
  critical: 'error',
  high: 'error',
  medium: 'warning',
  low: 'info',
}

/** Accent of a severity's tile and marker, through the theme's chart tokens. */
export const SEVERITY_COLORS: Record<Severity, string> = {
  critical: chartColor('red'),
  high: 'var(--vscode-charts-orange)',
  medium: chartColor('yellow'),
  low: chartColor('blue'),
}

/** Color of a 0–100 score: green from 80, yellow from 50, red below. */
export function scoreColor(score: number | null): string {
  if (score === null) return 'var(--vscode-descriptionForeground)'
  if (score >= 80) return chartColor('green')
  if (score >= 50) return chartColor('yellow')
  return chartColor('red')
}

/** "CIS 5.2.2" → "CIS"; the Pod Security Standards and the rest keep their name. */
export function frameworkFamily(framework: string): string {
  return framework.startsWith('CIS ') ? 'CIS' : framework
}
