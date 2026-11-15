import type { ReactNode } from 'react'
import type { DrainResult } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { Dialog } from '@/shared/Dialog'
import { useWorkloadsTranslation } from '../i18n'
import './ops.css'

type Tone = 'error' | 'warning' | 'success' | 'neutral'

function ResultSection({ title, tone, hint, children }: { title: string; tone: Tone; hint?: string; children: ReactNode }) {
  return (
    <section className="ops-result">
      <h3 className={`ops-result__title ops-result__title--${tone}`}>{title}</h3>
      {hint && <p className="ops-result__hint">{hint}</p>}
      <ul className="ops-result__list">{children}</ul>
    </section>
  )
}

/** What a drain did, Pod by Pod: blocked, failed, evicted and skipped. */
export function DrainResultDialog({ result, onClose }: { result: DrainResult; onClose: () => void }) {
  const { t } = useWorkloadsTranslation()
  const blocked = new Set(result.blockedByPDB)
  const pdbFailures = result.failed.filter((f) => blocked.has(f.pod))
  const otherFailures = result.failed.filter((f) => !blocked.has(f.pod))
  const nothingToDo =
    result.errors.length + result.failed.length + result.evicted.length + result.skipped.length === 0

  return (
    <Dialog
      open
      size="medium"
      icon={result.completed ? 'pass' : 'warning'}
      title={t('workloadOps.drain.resultTitle', { name: result.node })}
      onClose={onClose}
      closeOnBackdropClick
      footer={<Button onClick={onClose}>{t('common.close')}</Button>}
    >
      <div className="ops-result__summary">
        <Badge tone={result.completed ? 'success' : 'warning'}>
          {result.completed ? t('workloadOps.drain.completed') : t('workloadOps.drain.incomplete')}
        </Badge>
        {result.cordoned && <span>{t('workloadOps.drain.cordonedNote')}</span>}
      </div>
      {nothingToDo && <p className="ops-muted">{t('workloadOps.drain.nothing')}</p>}
      {result.errors.length > 0 && (
        <ResultSection title={t('workloadOps.drain.errors')} tone="error" hint={t('workloadOps.drain.errorsHint')}>
          {result.errors.map((e) => (
            <li key={e} className="ops-result__detail">
              {e}
            </li>
          ))}
        </ResultSection>
      )}
      {pdbFailures.length > 0 && (
        <ResultSection
          title={t('workloadOps.drain.blocked', { count: pdbFailures.length })}
          tone="warning"
          hint={t('workloadOps.drain.blockedHint')}
        >
          {pdbFailures.map((f) => (
            <li key={f.pod}>
              <code className="ops-result__pod">{f.pod}</code>
              <span className="ops-result__detail">{f.error}</span>
            </li>
          ))}
        </ResultSection>
      )}
      {otherFailures.length > 0 && (
        <ResultSection title={t('workloadOps.drain.failed', { count: otherFailures.length })} tone="error">
          {otherFailures.map((f) => (
            <li key={f.pod}>
              <code className="ops-result__pod">{f.pod}</code>
              <span className="ops-result__detail">{f.error}</span>
            </li>
          ))}
        </ResultSection>
      )}
      {result.evicted.length > 0 && (
        <ResultSection title={t('workloadOps.drain.evicted', { count: result.evicted.length })} tone="success">
          {result.evicted.map((pod) => (
            <li key={pod}>
              <code className="ops-result__pod">{pod}</code>
            </li>
          ))}
        </ResultSection>
      )}
      {result.skipped.length > 0 && (
        <ResultSection title={t('workloadOps.drain.skipped', { count: result.skipped.length })} tone="neutral">
          {result.skipped.map((s) => (
            <li key={s.pod}>
              <code className="ops-result__pod">{s.pod}</code>
              <span className="ops-result__detail">
                {s.reason === 'mirror' ? t('workloadOps.drain.reasonMirror') : t('workloadOps.drain.reasonDaemonSet')}
              </span>
            </li>
          ))}
        </ResultSection>
      )}
    </Dialog>
  )
}
