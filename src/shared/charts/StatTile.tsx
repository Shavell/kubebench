import { Icon } from '../Icon'
import './charts.css'

interface StatTileProps {
  icon: string
  label: string
  value: string
  /** Small line under the value, e.g. "2 of 3 ready". */
  detail?: string
  accent?: string
}

/** Big-number KPI tile for overview pages. */
export function StatTile({ icon, label, value, detail, accent }: StatTileProps) {
  return (
    <div className="kb-stat">
      <div className="kb-stat__head">
        <Icon name={icon} size={16} style={accent ? { color: accent } : undefined} />
        <span>{label}</span>
      </div>
      <div className="kb-stat__value">{value}</div>
      {detail && <div className="kb-stat__detail">{detail}</div>}
    </div>
  )
}
