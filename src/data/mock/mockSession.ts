import { ApiError } from '../apiClient'
import { reasonError } from '../reasonError'
import type { SessionProvider } from '../providers'
import { MIN_PASSWORD_LENGTH, passwordLength } from '../rules'
import type { ServerInfo } from '../types'
import { delay, delayReject } from './delay'
import { activeMockCluster } from './mockInventory'
import { mockOptionalResources } from './mockOpenShift'
import { MOCK_USERNAME } from './mockUsers'

/**
 * The mock session: an administrator on the active mock cluster. The
 * cluster's environment and read-only flag come from the mock inventory,
 * so switching to the production entry behaves like production (red status
 * bar, type-to-confirm, no writes on a read-only cluster).
 */
export function mockServerInfo(): ServerInfo {
  const cluster = activeMockCluster()
  const readOnly = cluster.readOnly ?? false
  return {
    version: __APP_VERSION__,
    commit: '',
    user: MOCK_USERNAME,
    role: 'admin',
    mustChangePassword: false,
    authSource: 'local',
    context: cluster.name,
    server: cluster.server ?? '',
    cluster: {
      id: cluster.id ?? 0,
      name: cluster.name,
      environment: cluster.environment ?? '',
      readOnly,
      platform: cluster.platform,
    },
    permissions: {
      role: 'admin',
      canRead: true,
      canWrite: !readOnly,
      canExec: !readOnly,
      isAdmin: true,
      readOnly,
    },
    // The OpenShift resource types are served by the OpenShift mock cluster only.
    capabilities: {
      crds: true,
      helm: true,
      exec: true,
      optionalResources: mockOptionalResources(cluster.platform),
      // The mock's credential may do everything cluster-wide (design §7.16).
      listNamespaces: true,
      namespaces: cluster.allowedNamespaces ?? [],
      defaultNamespace: '',
      execNamespaces: [],
      helmNamespaces: [],
    },
    // The mock has no server side for runtime-loaded plugins.
    features: { pathKubeconfig: true, externalPlugins: false, ambientCloudIdentity: true },
    clusterError: '',
  }
}

export function createMockSession(): SessionProvider {
  return {
    info: () => delay(mockServerInfo()),
    changePassword: (current, next) => {
      if (passwordLength(next) < MIN_PASSWORD_LENGTH) {
        return delayReject(reasonError(400, 'bad_request', 'password.too_short', { min: MIN_PASSWORD_LENGTH }))
      }
      if (next === current) {
        return delayReject(reasonError(400, 'bad_request', 'auth.same_password'))
      }
      return delay(undefined)
    },
    logout: () =>
      delayReject(new ApiError(400, 'bad_request', 'demo mode has no sign-in, so there is nothing to sign out of')),
  }
}
