import './charts.css'

interface UsageBarProps {
  label: string
  value: number
  total: number
  unit: string
  color: string
}

/** Horizontal allocation bar: "label — value / total unit (pct%)". */
export function UsageBar({ label, value, total, unit, color }: UsageBarProps) {
  const pct = total > 0 ? (value / total) * 100 : 0
  return (
    <div className="kb-usagebar">
      <div className="kb-usagebar__head">
        <span>{label}</span>
        <span className="kb-usagebar__value">
          {value} / {total} {unit} · {pct.toFixed(0)}%
        </span>
      </div>
      <div className="kb-usagebar__track">
        <div
          className="kb-usagebar__fill"
          style={{ width: `${Math.min(100, pct)}%`, background: color }}
        />
        {pct > 100 && <div className="kb-usagebar__overflow" title="Overcommitted" />}
      </div>
    </div>
  )
}
