import { useState } from 'react'
import { useTranslation } from 'react-i18next'
import { Badge } from '@/shared/Badge'
import { ErrorNotice } from '@/shared/ErrorNotice'
import { accessData } from '../data'
import type { AccessRequest, ReviewResult, Subject } from '../data/types'
import { NS } from '../i18n'
import { requestTarget } from '../model/format'
import { EMPTY_DRAFT, requestOf, type RequestDraft } from '../model/request'
import { AccessRequestForm } from './AccessRequestForm'
import { PathCell } from './parts'

interface Answer {
  request: AccessRequest
  result: ReviewResult
}

/**
 * Quick "can-i" for one subject: the cluster answers through a
 * SubjectAccessReview, shown next to the bindings that grant it.
 */
export function CanIBox({ subject }: { subject: Subject }) {
  const { t } = useTranslation(NS)
  const [draft, setDraft] = useState<RequestDraft>(() => ({ ...EMPTY_DRAFT, namespace: subject.namespace ?? '' }))
  const [busy, setBusy] = useState(false)
  const [error, setError] = useState<unknown>(null)
  const [answer, setAnswer] = useState<Answer | null>(null)

  const ask = async () => {
    const request = requestOf(draft)
    setBusy(true)
    setError(null)
    try {
      setAnswer({ request, result: await accessData().review({ ...request, subject }) })
    } catch (err) {
      setAnswer(null)
      setError(err)
    } finally {
      setBusy(false)
    }
  }

  const result = answer?.result
  const rbacAllows = Boolean(result?.grants.length)
  return (
    <div className="access-cani">
      <AccessRequestForm draft={draft} onChange={setDraft} onSubmit={ask} submitLabel={t('canI.ask')} busy={busy} />
      <ErrorNotice error={error} />
      {answer && result && (
        <div className="access-cani__answer" role="status">
          <div className="access-cani__verdict">
            <Badge tone={result.allowed ? 'success' : 'error'}>{result.allowed ? t('canI.yes') : t('canI.no')}</Badge>
            <span>
              {t('canI.question', { verb: answer.request.verb, target: requestTarget(answer.request) })}
              {answer.request.namespace && ` · ${t('paths.inNamespace', { namespace: answer.request.namespace })}`}
            </span>
          </div>
          {result.reason && <div className="access-muted">{t('canI.reason', { reason: result.reason })}</div>}
          {result.evaluationError && <div className="access-missing">{t('canI.evaluationError', { error: result.evaluationError })}</div>}
          {result.allowed !== rbacAllows && <div className="access-note">{t(result.allowed ? 'canI.otherAuthorizer' : 'canI.deniedDespiteRbac')}</div>}
          {rbacAllows && (
            <div className="access-stack">
              <span className="access-muted">{t('canI.grantedBy')}</span>
              {result.grants.map((g) => (
                <PathCell key={`${g.binding.kind}/${g.binding.namespace ?? ''}/${g.binding.name}`} path={g} />
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  )
}
