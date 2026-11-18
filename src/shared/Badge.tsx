import { toneForStatus, type BadgeTone } from './badgeTone'
import './Badge.css'

export type { BadgeTone }

interface BadgeProps {
  children: string
  tone?: BadgeTone
}

export function Badge({ children, tone }: BadgeProps) {
  return (
    <span className={`kb-badge kb-badge--${tone ?? toneForStatus(children)}`}>
      {children}
    </span>
  )
}
