import type { ClusterPlatform, CloudProvider, InfraCloud, VersionSupportStatus } from '../types'

const PROVIDERS: readonly CloudProvider[] = ['eks', 'aks', 'gke', 'doks', 'lke', 'ovh', 'openshift', 'rke2', 'k3s', 'kubernetes']
const CLOUDS: readonly InfraCloud[] = ['', 'aws', 'azure', 'gcp', 'digitalocean', 'linode', 'ovh']
const STATUSES: readonly VersionSupportStatus[] = ['supported', 'ending-soon', 'end-of-life', 'unknown']

type Json = Record<string, unknown>
const isObject = (value: unknown): value is Json => typeof value === 'object' && value !== null && !Array.isArray(value)
const str = (value: unknown): string => (typeof value === 'string' ? value : '')

/**
 * Normalises a `platform` object of /api/info (design §7.12). Values a
 * newer server may add (another provider) fall back to the generic ones
 * rather than reaching the UI unchecked.
 */
export function parsePlatform(value: unknown): ClusterPlatform | null {
  if (!isObject(value)) return null
  const support = isObject(value.support) ? value.support : {}
  return {
    provider: PROVIDERS.find((p) => p === value.provider) ?? 'kubernetes',
    cloud: CLOUDS.find((c) => c === value.cloud) ?? '',
    version: str(value.version),
    support: {
      status: STATUSES.find((s) => s === support.status) ?? 'unknown',
      endDate: str(support.endDate),
      basis: str(support.basis),
    },
    detected: value.detected === true,
  }
}
