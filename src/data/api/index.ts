import type { DataProviders } from '../providers'
import type { LiveCapabilities } from '../types'
import {
  createApiAudit,
  createApiInventory,
  createApiPlugins,
  createApiSession,
  createApiUsers,
} from './admin'
import { createApiCluster } from './cluster'
import { createApiCrd } from './crd'
import { createApiHelm } from './helm'
import { createApiMetrics } from './metrics'
import { createApiWorkloadOps } from './workloadOps'

/**
 * Live providers backed by the Go backend (`/api`). Capabilities come from
 * /api/info for the active cluster; a missing capability makes its
 * provider reject with `capability_unavailable` — never mock data.
 */
export function createApiProviders(capabilities: LiveCapabilities): DataProviders {
  return {
    cluster: createApiCluster(capabilities),
    crd: createApiCrd(capabilities.crds),
    helm: createApiHelm(capabilities.helm),
    metrics: createApiMetrics(),
    inventory: createApiInventory(),
    users: createApiUsers(),
    session: createApiSession(),
    audit: createApiAudit(),
    plugins: createApiPlugins(),
    workloadOps: createApiWorkloadOps(),
  }
}
