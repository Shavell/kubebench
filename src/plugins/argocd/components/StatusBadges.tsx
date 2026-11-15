import { useTranslation } from 'react-i18next'
import { Badge, type BadgeTone } from '@/shared/Badge'
import { NS } from '../i18n'

/** Badge tones of Argo CD's sync, health and operation states. */
const TONES: Record<string, BadgeTone> = {
  Synced: 'success',
  OutOfSync: 'warning',
  Healthy: 'success',
  Progressing: 'info',
  Degraded: 'error',
  Suspended: 'neutral',
  Missing: 'warning',
  Running: 'info',
  Terminating: 'warning',
  Succeeded: 'success',
  Failed: 'error',
  Error: 'error',
  SyncFailed: 'error',
  Pruned: 'neutral',
  PruneSkipped: 'neutral',
  Deleting: 'warning',
}

/**
 * A translated Argo CD state (`states.<value>`); values Argo CD adds later
 * show as they are.
 */
export function StateBadge({ value }: { value: string }) {
  const { t } = useTranslation(NS)
  if (!value) return null
  return <Badge tone={TONES[value] ?? 'neutral'}>{t(`states.${value}`, { defaultValue: value })}</Badge>
}
