import { PLATFORM_NS } from '@/core/locales'
import { i18n } from '@/i18n'
import { ApiError, apiGet } from '../apiClient'
import type {
  ActiveCluster,
  ClusterEnvironment,
  ClusterRole,
  GlobalRole,
  ServerInfo,
} from '../types'
import { parsePlatform } from './platform'

/**
 * The backend can take up to 3 s to answer /api/info while it waits for a
 * fresh cluster runtime's capability probe (design §4.1).
 */
const INFO_TIMEOUT_MS = 10_000

type Json = Record<string, unknown>

const isObject = (value: unknown): value is Json =>
  typeof value === 'object' && value !== null && !Array.isArray(value)
const str = (value: unknown): string => (typeof value === 'string' ? value : '')
const bool = (value: unknown): boolean => value === true

const ENVIRONMENTS: readonly ClusterEnvironment[] = ['', 'development', 'staging', 'production']
const CLUSTER_ROLES: readonly ClusterRole[] = ['admin', 'operator', 'viewer']

function environment(value: unknown): ClusterEnvironment {
  return ENVIRONMENTS.find((env) => env === value) ?? ''
}

function cluster(value: unknown): ActiveCluster | null {
  if (!isObject(value) || typeof value.id !== 'number') return null
  return {
    id: value.id,
    name: str(value.name),
    environment: environment(value.environment),
    readOnly: bool(value.readOnly),
    platform: parsePlatform(value.platform),
  }
}

/**
 * Normalises the /api/info body. Anything that is not a KubeBench session
 * (an HTML page from a proxy or static host answering for the API, an
 * older server) is rejected, so boot never mistakes it for a live session.
 * Missing permission flags count as not granted.
 */
function parseServerInfo(body: unknown): ServerInfo {
  if (!isObject(body) || typeof body.user !== 'string' || !isObject(body.permissions)) {
    throw new ApiError(502, 'http_error', i18n.t('boot.notKubeBench', { ns: PLATFORM_NS }))
  }
  const perms = body.permissions
  const capabilities = isObject(body.capabilities) ? body.capabilities : {}
  const features = isObject(body.features) ? body.features : {}
  const role: GlobalRole = body.role === 'admin' ? 'admin' : 'user'
  const clusterRole = CLUSTER_ROLES.find((r) => r === perms.role) ?? 'viewer'
  return {
    version: str(body.version),
    commit: str(body.commit),
    user: body.user,
    role,
    mustChangePassword: bool(body.mustChangePassword),
    context: str(body.context),
    server: str(body.server),
    cluster: cluster(body.cluster),
    permissions: {
      role: clusterRole,
      canRead: bool(perms.canRead),
      canWrite: bool(perms.canWrite),
      canExec: bool(perms.canExec),
      isAdmin: bool(perms.isAdmin),
      readOnly: bool(perms.readOnly),
    },
    capabilities: {
      crds: bool(capabilities.crds),
      helm: bool(capabilities.helm),
      exec: bool(capabilities.exec),
    },
    features: {
      pathKubeconfig: bool(features.pathKubeconfig),
      externalPlugins: bool(features.externalPlugins),
      ambientCloudIdentity: bool(features.ambientCloudIdentity),
    },
    clusterError: str(body.clusterError),
    clusterErrorReason: str(body.clusterErrorReason) || undefined,
    clusterErrorParams: isObject(body.clusterErrorParams) ? body.clusterErrorParams : undefined,
  }
}

/** GET /api/info. Rejects with ApiError (401 when signed out, status 0 when unreachable). */
export async function fetchServerInfo(): Promise<ServerInfo> {
  const body = await apiGet<unknown>('/info', undefined, { signal: AbortSignal.timeout(INFO_TIMEOUT_MS) })
  return parseServerInfo(body)
}
