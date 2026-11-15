import type { ReactNode } from 'react'
import { useTranslation } from 'react-i18next'
import { Badge, type BadgeTone } from '@/shared/Badge'
import { Icon } from '@/shared/Icon'
import type { GrantPath, RbacRef, Severity, Subject } from '../data/types'
import { NS } from '../i18n'
import { refText, subjectName } from '../model/format'
import { openResource, openSubject, subjectIcon } from '../tabs'

/** A link-styled button that does not open the surrounding table row. */
function LinkButton({ onClick, title, children }: { onClick: () => void; title?: string; children: ReactNode }) {
  return (
    <button
      type="button"
      className="access-link"
      title={title}
      onClick={(e) => {
        e.stopPropagation()
        onClick()
      }}
    >
      {children}
    </button>
  )
}

/** Icon, kind and name of a subject; opens its effective permissions. */
export function SubjectCell({ subject, missing }: { subject: Subject; missing?: boolean }) {
  const { t } = useTranslation(NS)
  return (
    <span className="access-subject-cell">
      <Icon name={subjectIcon(subject)} size={14} />
      <LinkButton onClick={() => openSubject(subject)} title={t('subjects.open')}>
        <span className="access-mono">{subjectName(subject)}</span>
      </LinkButton>
      {missing && <Badge tone="warning">{t('subjects.missing')}</Badge>}
    </span>
  )
}

/** An RBAC object reference that opens the object. */
export function RefLink({ refValue, missing }: { refValue: RbacRef; missing?: boolean }) {
  const { t } = useTranslation(NS)
  if (missing) {
    return (
      <span className="access-missing" title={t('paths.roleMissingHint')}>
        <Icon name="warning" size={12} /> {refValue.kind} <span className="access-mono">{refText(refValue)}</span>
      </span>
    )
  }
  return (
    <LinkButton onClick={() => openResource(refValue.kind, refValue.namespace, refValue.name)} title={t('paths.open')}>
      <span className="access-muted">{refValue.kind}</span> <span className="access-mono">{refText(refValue)}</span>
    </LinkButton>
  )
}

/** binding → role, with the scope it grants in and the group it reaches the subject through (both left out when `compact`). */
export function PathCell({ path, compact = false }: { path: GrantPath; compact?: boolean }) {
  const { t } = useTranslation(NS)
  return (
    <span className="access-path">
      <RefLink refValue={path.binding} />
      <Icon name="arrow-right" size={12} className="access-muted" />
      <RefLink refValue={path.role} missing={path.roleMissing} />
      {!compact && (
        <Badge tone={path.scope ? 'neutral' : 'info'}>
          {path.scope ? t('paths.inNamespace', { namespace: path.scope }) : t('paths.clusterWide')}
        </Badge>
      )}
      {!compact && path.via && <span className="access-muted">{t('paths.via', { group: path.via })}</span>}
    </span>
  )
}

const SEVERITY_TONES: Record<Severity, BadgeTone> = { critical: 'error', high: 'warning', medium: 'info' }

export function SeverityBadge({ severity }: { severity: Severity }) {
  const { t } = useTranslation(NS)
  return <Badge tone={SEVERITY_TONES[severity]}>{t(`severity.${severity}`)}</Badge>
}

/** Comma-separated monospace values, or a dash. */
export function Mono({ values }: { values: string[] }) {
  return <span className="access-mono">{values.length ? values.join(', ') : '—'}</span>
}
