import { Icon } from './Icon'
import './EmptyState.css'

interface EmptyStateProps {
  icon: string
  title: string
  hint?: string
}

export function EmptyState({ icon, title, hint }: EmptyStateProps) {
  return (
    <div className="kb-empty">
      <Icon name={icon} size={48} className="kb-empty__icon" />
      <div className="kb-empty__title">{title}</div>
      {hint && <div className="kb-empty__hint">{hint}</div>}
    </div>
  )
}
