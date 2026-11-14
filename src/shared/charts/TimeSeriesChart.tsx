import { useId } from 'react'
import { useTranslation } from 'react-i18next'
import {
  Area,
  AreaChart,
  CartesianGrid,
  ResponsiveContainer,
  Tooltip,
  XAxis,
  YAxis,
} from 'recharts'
import { formatNumber, formatPercent } from '@/i18n/format'
import { CHART_GRID, CHART_TEXT } from './chartTheme'
import './charts.css'

export interface SeriesPoint {
  /** Minute offset label, e.g. "14:32". */
  time: string
  value: number
}

interface TimeSeriesChartProps {
  title: string
  data: SeriesPoint[]
  color: string
  /** Suffix appended to values, e.g. " MB/s"; '%' uses the locale's percent format. */
  unit?: string
  height?: number
  /** Fixed y-domain, e.g. [0, 100] for percentages. */
  domain?: [number, number]
}

/** A value with the chart unit; percentages follow the locale (e.g. "%12,5" in Turkish). */
function formatValue(value: number, unit: string, fractionDigits: number): string {
  if (unit === '%') return formatPercent(value / 100, fractionDigits)
  return `${formatNumber(value, { maximumFractionDigits: fractionDigits })}${unit}`
}

/**
 * Single-series area chart. Charts are deliberately single-series
 * (small multiples over legends) so the title alone names the series.
 */
export function TimeSeriesChart({
  title,
  data,
  color,
  unit = '%',
  height = 180,
  domain = [0, 100],
}: TimeSeriesChartProps) {
  // Re-render on language change so the locale-aware formatters below re-run.
  useTranslation()
  // Unique per chart instance; translated titles are not safe as SVG ids.
  const gradientId = `grad-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`
  return (
    <div className="kb-chart">
      <div className="kb-chart__title">{title}</div>
      <ResponsiveContainer width="100%" height={height}>
        <AreaChart data={data} margin={{ top: 6, right: 8, bottom: 0, left: 0 }}>
          <defs>
            <linearGradient id={gradientId} x1="0" y1="0" x2="0" y2="1">
              <stop offset="0%" stopColor={color} stopOpacity={0.28} />
              <stop offset="100%" stopColor={color} stopOpacity={0.02} />
            </linearGradient>
          </defs>
          <CartesianGrid stroke={CHART_GRID} vertical={false} />
          <XAxis
            dataKey="time"
            tick={{ fill: CHART_TEXT, fontSize: 10 }}
            tickLine={false}
            axisLine={{ stroke: CHART_GRID }}
            minTickGap={40}
          />
          {/* "auto" fits the widest tick ("%100", "60 MB/s"); a negative chart margin would clip it again. */}
          <YAxis
            width="auto"
            domain={domain}
            tick={{ fill: CHART_TEXT, fontSize: 10 }}
            tickLine={false}
            axisLine={false}
            tickFormatter={(v: number) => formatValue(v, unit, 0)}
          />
          <Tooltip
            cursor={{ stroke: CHART_TEXT, strokeDasharray: '3 3' }}
            contentStyle={{
              background: 'var(--vscode-editorWidget-background)',
              border: '1px solid var(--vscode-panel-border)',
              borderRadius: 3,
              fontSize: 12,
              color: 'var(--vscode-editor-foreground)',
            }}
            formatter={(value) => [formatValue(Number(value), unit, 1), title]}
          />
          <Area
            type="monotone"
            dataKey="value"
            stroke={color}
            strokeWidth={2}
            fill={`url(#${gradientId})`}
            isAnimationActive={false}
            dot={false}
            activeDot={{ r: 4, strokeWidth: 2, stroke: 'var(--vscode-editor-background)' }}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  )
}
