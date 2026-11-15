import clsx from 'clsx'
import { useTranslation } from 'react-i18next'
import { PLATFORM_NS } from '@/core/locales'
import type { ClusterPlatform } from '@/data/types'
import { platformLabel, platformTooltip } from './platformText'
import './cloud.css'

interface ProviderBadgeProps {
  platform: ClusterPlatform | null | undefined
  /** Adds the version after the label (tables). */
  showVersion?: boolean
  /** Status bar variant: inherits the bar's colours. */
  compact?: boolean
}

/**
 * A text badge naming where a cluster runs (EKS, AKS, GKE, k3s, … — no
 * brand logos; design §7.12), with the version and its support window in
 * the tooltip. An unsupported version gets the warning or error tone.
 * Renders nothing for plain Kubernetes.
 */
export function ProviderBadge({ platform, showVersion, compact }: ProviderBadgeProps) {
  const { t } = useTranslation(PLATFORM_NS)
  if (!platform) return null
  const label = platformLabel(t, platform)
  if (!label) return null
  const status = platform.support.status
  return (
    <span
      className={clsx(
        'kb-provider',
        compact && 'kb-provider--compact',
        status === 'ending-soon' && 'kb-provider--warning',
        status === 'end-of-life' && 'kb-provider--error',
      )}
      title={platformTooltip(t, platform)}
      data-provider={platform.provider}
    >
      {label}
      {showVersion && platform.version && <span className="kb-provider__version">{platform.version}</span>}
    </span>
  )
}
