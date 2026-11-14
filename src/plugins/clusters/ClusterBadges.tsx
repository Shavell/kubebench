import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import type { ClusterEnvironment, ClusterRole } from '@/data/types'
import { Badge } from '@/shared/Badge'
import { Icon } from '@/shared/Icon'
import { CLUSTERS_NS } from './namespace'
import './clusters.css'

/** The environment tag in the status bar's colours (production red, staging amber, development green). */
export function EnvironmentBadge({ environment }: { environment: ClusterEnvironment | undefined }) {
  const { t } = useTranslation(CLUSTERS_NS)
  if (!environment) return <span className="clusters__muted">{t('environment.none')}</span>
  return <span className={clsx('clusters-env', `clusters-env--${environment}`)}>{t(`environment.${environment}`)}</span>
}

/** A small dot in the environment colour, for compact lists. */
export function EnvironmentDot({ environment }: { environment: ClusterEnvironment | undefined }) {
  const { t } = useTranslation(CLUSTERS_NS)
  if (!environment) return null
  return (
    <span
      className={clsx('clusters-env-dot', `clusters-env--${environment}`)}
      role="img"
      aria-label={t(`environment.${environment}`)}
      title={t(`environment.${environment}`)}
    />
  )
}

/** The caller's effective role on a cluster. */
export function RoleBadge({ role }: { role: ClusterRole | undefined }) {
  const { t } = useTranslation(CLUSTERS_NS)
  if (!role) return <span className="clusters__muted">{t('roles.none')}</span>
  return <Badge tone={role === 'viewer' ? 'neutral' : 'info'}>{t(`roles.${role}`)}</Badge>
}

export function ReadOnlyBadge() {
  const { t } = useTranslation(CLUSTERS_NS)
  return (
    <span className="clusters-readonly" title={t('readOnly.hint')}>
      <Icon name="lock" size={12} />
      {t('readOnly.label')}
    </span>
  )
}
