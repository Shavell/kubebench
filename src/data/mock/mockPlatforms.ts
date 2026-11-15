import type { ClusterPlatform } from '../types'

/**
 * Fixed platforms of the default mock clusters (design §7.12): one of each
 * support status, so the badges and the overview notice can be seen in
 * demo mode. Statuses are part of the data rather than computed from a
 * clock, which keeps the mock deterministic.
 */
const MOCK_PLATFORMS: Record<number, ClusterPlatform> = {
  // kind-dev-cluster
  1: {
    provider: 'k3s',
    cloud: '',
    version: 'v1.35.5+k3s1',
    support: { status: 'supported', endDate: '2027-02-28', basis: 'kubernetes' },
    detected: true,
  },
  // staging
  2: {
    provider: 'aks',
    cloud: 'azure',
    version: 'v1.34.4',
    support: { status: 'ending-soon', endDate: '2026-11-30', basis: 'aks' },
    detected: true,
  },
  // prod-eu-west-1
  3: {
    provider: 'eks',
    cloud: 'aws',
    version: 'v1.33.5-eks-3025e55',
    support: { status: 'end-of-life', endDate: '2026-07-29', basis: 'eks' },
    detected: true,
  },
}

/** A cluster added in demo mode is never connected: nothing is known about it. */
const UNDETECTED: ClusterPlatform = {
  provider: 'kubernetes',
  cloud: '',
  version: '',
  support: { status: 'unknown', endDate: '', basis: '' },
  detected: false,
}

/** The platform of the mock cluster with this id (ids are kept across renames). */
export function mockPlatform(id: number): ClusterPlatform {
  return MOCK_PLATFORMS[id] ?? UNDETECTED
}
