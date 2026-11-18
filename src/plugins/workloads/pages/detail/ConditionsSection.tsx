import type { KubeObject } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { useWorkloadsTranslation } from '../../i18n'
import { DetailRow, DetailSection } from './DetailBits'

interface Condition {
  type?: unknown
  status?: unknown
  reason?: unknown
  message?: unknown
}

const text = (v: unknown) => (typeof v === 'string' ? v : '')

/**
 * The object's status.conditions (a DeploymentConfig's Available and
 * Progressing, a Build's, …): type, status, reason and message. Renders
 * nothing for objects without conditions.
 */
export function ConditionsSection({ obj }: { obj: KubeObject }) {
  const { t } = useWorkloadsTranslation()
  const raw = (obj as { status?: { conditions?: unknown } }).status?.conditions
  const conditions = Array.isArray(raw) ? (raw as Condition[]).filter((c) => text(c.type)) : []
  if (conditions.length === 0) return null
  return (
    <>
      <DetailSection title={t('openshift.conditions')} />
      <div className="detail__grid detail__grid--wide">
        {conditions.map((c) => {
          const status = text(c.status)
          const why = [text(c.reason), text(c.message)].filter(Boolean).join(': ')
          return (
            <DetailRow
              key={text(c.type)}
              label={text(c.type)}
              value={
                <>
                  <Badge tone={status === 'True' ? 'success' : status === 'False' ? 'error' : 'neutral'}>{status || '—'}</Badge>
                  {why && <span> {why}</span>}
                </>
              }
            />
          )
        })}
      </div>
    </>
  )
}
