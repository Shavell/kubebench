import { usePermission } from '@/core/permissions'
import { reasonText, warningTexts } from '@/core/reasons'
import type { ApplyResult } from '@/data/types'
import { Badge, type BadgeTone } from '@/shared/Badge'
import { Button } from '@/shared/Button'
import { Icon } from '@/shared/Icon'
import { outcomeChangedOnServer, outcomeHasConflict, summarize, type ApplyOutcome } from './applyRunner'
import { useEditorTranslation } from './i18n'
import './editor.css'

const TONES: Record<string, BadgeTone> = {
  created: 'success',
  configured: 'info',
  unchanged: 'neutral',
  failed: 'error',
}

const objectName = (r: ApplyResult) => (r.namespace ? `${r.namespace}/${r.name}` : r.name)

/**
 * The outcome of a dry run or apply, one row per object. A server-side
 * apply conflict (another field manager owns a field the manifest changes)
 * gets an explanation and the Force apply button; an edit of an object
 * that changed on the server since editing started gets the ways to go on
 * (`onChanged`).
 */
export function ApplyResults({
  outcome,
  onForce,
  forcing,
  onChanged,
  onDismiss,
}: {
  outcome: ApplyOutcome
  /** Applies again with force; omitted where forcing makes no sense. */
  onForce?: () => void
  forcing?: boolean
  /** For an edit: compare it with the object's current version, or discard it and reload. */
  onChanged?: { review: () => void; reload: () => void; busy: boolean }
  onDismiss: () => void
}) {
  const { t } = useEditorTranslation()
  const write = usePermission('write')
  const { dryRun, results, refusal } = outcome
  const title = dryRun ? t('apply.dryRunTitle') : t('apply.appliedTitle')

  return (
    <section className="kb-apply" aria-label={title}>
      <header className="kb-apply__header">
        <Icon name={dryRun ? 'beaker' : 'cloud-upload'} size={14} />
        <span className="kb-apply__title">{title}</span>
        {results.length > 0 && <span className="kb-apply__summary">{summarize(results, dryRun)}</span>}
        <button
          type="button"
          className="kb-apply__dismiss"
          title={t('apply.dismiss')}
          aria-label={t('apply.dismiss')}
          onClick={onDismiss}
        >
          <Icon name="close" size={14} />
        </button>
      </header>

      {refusal && (
        <div className="kb-apply__refusal" role="alert">
          <Icon name="error" size={14} />
          <div>
            <div>{refusal.message}</div>
            {refusal.detail && <div className="kb-apply__detail">{refusal.detail}</div>}
          </div>
        </div>
      )}

      {results.length > 0 && (
        <ul className="kb-apply__rows">
          {results.map((r, i) => {
            const state = r.error ? 'failed' : (r.action ?? 'unchanged')
            return (
              // Objects may repeat in a manifest; the position is the identity.
              <li key={i} className="kb-apply__row">
                <Badge tone={TONES[state]}>{t(`apply.${dryRun ? 'dryState' : 'state'}.${state}`)}</Badge>
                <span className="kb-apply__object">
                  <span className="kb-apply__kind">{r.kind}</span> {objectName(r)}
                </span>
                {r.error && <div className="kb-apply__message">{reasonText(r.reason, r.params, r.error)}</div>}
                {warningTexts(r.warnings, r.warningReasons).map((w) => (
                  <div key={w} className="kb-apply__warning">
                    <Icon name="warning" size={12} /> {w}
                  </div>
                ))}
              </li>
            )
          })}
        </ul>
      )}

      {onChanged && outcomeChangedOnServer(outcome) && (
        <div className="kb-apply__conflict">
          <Icon name="warning" size={14} />
          <div className="kb-apply__conflict-text">
            <strong>{t('apply.changedTitle')}</strong>
            <span>{t('apply.changedHint')}</span>
          </div>
          <Button variant="secondary" icon="discard" disabled={onChanged.busy} onClick={onChanged.reload}>
            {t('apply.changedReload')}
          </Button>
          <Button icon="diff" busy={onChanged.busy} onClick={onChanged.review}>
            {t('apply.changedReview')}
          </Button>
        </div>
      )}

      {onForce && outcomeHasConflict(outcome) && (
        <div className="kb-apply__conflict">
          <Icon name="warning" size={14} />
          <div className="kb-apply__conflict-text">
            <strong>{t('apply.conflictTitle')}</strong>
            <span>{t('apply.conflictHint')}</span>
          </div>
          <Button
            variant="danger"
            icon="flame"
            busy={forcing}
            disabled={!write.allowed}
            title={write.reason ?? undefined}
            onClick={onForce}
          >
            {t('apply.force')}
          </Button>
        </div>
      )}
    </section>
  )
}
