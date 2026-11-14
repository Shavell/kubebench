import { useTranslation } from 'react-i18next'
import { formatNumber, formatPercent } from '@/i18n/format'
import './charts.css'

interface UsageBarProps {
  label: string
  value: number
  total: number
  /** Already translated unit label, e.g. "cores" or "GiB". */
  unit: string
  color: string
}

/** Horizontal allocation bar: "label — value / total unit · pct". */
export function UsageBar({ label, value, total, unit, color }: UsageBarProps) {
  const { t } = useTranslation()
  const fraction = total > 0 ? value / total : 0
  return (
    <div className="kb-usagebar">
      <div className="kb-usagebar__head">
        <span>{label}</span>
        <span className="kb-usagebar__value">
          {t('charts.usage', {
            value: formatNumber(value),
            total: formatNumber(total),
            unit,
            percent: formatPercent(fraction),
          })}
        </span>
      </div>
      <div className="kb-usagebar__track">
        <div
          className="kb-usagebar__fill"
          style={{ width: `${Math.min(100, fraction * 100)}%`, background: color }}
        />
        {fraction > 1 && <div className="kb-usagebar__overflow" title={t('charts.overcommitted')} />}
      </div>
    </div>
  )
}
