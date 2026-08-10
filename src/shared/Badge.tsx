import './Badge.css'

export type BadgeTone = 'success' | 'warning' | 'error' | 'info' | 'neutral'

const STATUS_TONES: Record<string, BadgeTone> = {
  Running: 'success',
  Succeeded: 'success',
  Ready: 'success',
  Active: 'success',
  Deployed: 'success',
  deployed: 'success',
  Bound: 'success',
  Pending: 'warning',
  ContainerCreating: 'warning',
  Terminating: 'warning',
  'pending-upgrade': 'warning',
  superseded: 'neutral',
  Failed: 'error',
  failed: 'error',
  CrashLoopBackOff: 'error',
  Error: 'error',
  ImagePullBackOff: 'error',
  NotReady: 'error',
  Unknown: 'neutral',
}

/** Map a Kubernetes/Helm status string to a badge tone. */
export function toneForStatus(status: string): BadgeTone {
  return STATUS_TONES[status] ?? 'neutral'
}

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
