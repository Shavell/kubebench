import { ApiError } from '../apiClient'
import { reasonError } from '../reasonError'
import { MAX_ALLOWED_NAMESPACES, normalizeAllowedNamespaces } from '../rules'
import type { InventoryProvider } from '../providers'
import type { CloudCredentialProvider, ClusterEnvironment, ClusterEntry } from '../types'
import { delay, delayReject } from './delay'
import { mockPlatform } from './mockPlatforms'

/**
 * Mock cluster inventory, persisted to localStorage so add/remove/activate
 * survive the reload that follows a cluster switch. Activating a cluster
 * only changes which entry is marked active (and thereby the status bar
 * environment); the mock dataset itself is shared by all entries.
 */
const INVENTORY_KEY = 'kubebench.mock.clusters'

interface StoredCluster {
  id: number
  name: string
  kubeconfigPath?: string
  address?: string
  active: boolean
  environment: ClusterEnvironment
  readOnly: boolean
  /** The cloud whose identity signs in (design §7.12) and the stored per-cluster credentials. */
  cloudAuth?: CloudCredentialProvider
  cloudCredentials?: CloudCredentialProvider
  /** Namespaces offered when the credential cannot list them (design §7.16). */
  allowedNamespaces?: string[]
}

const DEFAULT_CLUSTERS: StoredCluster[] = [
  {
    id: 1,
    name: 'kind-dev-cluster',
    kubeconfigPath: '~/.kube/config (mock)',
    active: true,
    environment: 'development',
    readOnly: false,
  },
  {
    id: 2,
    name: 'staging',
    kubeconfigPath: '(stored kubeconfig)',
    active: false,
    environment: 'staging',
    readOnly: false,
    cloudAuth: 'azure',
  },
  {
    id: 3,
    name: 'prod-eu-west-1',
    kubeconfigPath: '(stored kubeconfig)',
    active: false,
    environment: 'production',
    readOnly: true,
    cloudAuth: 'aws',
    cloudCredentials: 'aws',
  },
  {
    // OpenShift (design §7.15): Routes, Projects, SCCs, builds and image streams.
    id: 4,
    name: 'ocp-lab',
    kubeconfigPath: '(stored kubeconfig)',
    active: false,
    environment: 'staging',
    readOnly: false,
  },
]

function isStoredCluster(value: unknown): value is StoredCluster {
  const c = value as StoredCluster
  return typeof c?.id === 'number' && typeof c.name === 'string' && typeof c.active === 'boolean'
}

function load(): StoredCluster[] {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(INVENTORY_KEY) ?? 'null')
    // Entries written by older versions lack ids/flags: start over from the defaults.
    if (Array.isArray(parsed) && parsed.every(isStoredCluster)) return parsed
  } catch {
    /* unreadable storage → defaults */
  }
  return DEFAULT_CLUSTERS.map((c) => ({ ...c }))
}

function save(clusters: StoredCluster[]): void {
  try {
    localStorage.setItem(INVENTORY_KEY, JSON.stringify(clusters))
  } catch {
    /* storage unavailable (private mode): changes last for this page only */
  }
}

function toEntry(c: StoredCluster): ClusterEntry {
  return {
    id: c.id,
    name: c.name,
    source: c.address && !c.kubeconfigPath ? 'address' : 'kubeconfig',
    kubeconfigPath: c.kubeconfigPath,
    address: c.address,
    active: c.active,
    context: c.name,
    server: c.address ?? `https://${c.name}.mock:6443`,
    environment: c.environment,
    readOnly: c.readOnly,
    role: 'admin',
    platform: mockPlatform(c.id),
    cloudAuth: c.cloudAuth,
    cloudCredentials: c.cloudCredentials,
    allowedNamespaces: c.allowedNamespaces ?? [],
  }
}

/** The active mock cluster, used at boot for the session facts. */
export function activeMockCluster(): ClusterEntry {
  const clusters = load()
  return toEntry(clusters.find((c) => c.active) ?? clusters[0] ?? DEFAULT_CLUSTERS[0])
}

/** Id → name of every mock cluster (for user assignments and audit rows). */
export function mockClusterNames(): Map<number, string> {
  return new Map(load().map((c) => [c.id, c.name]))
}

export function createMockInventory(): InventoryProvider {
  return {
    listClusters: () => delay(load().map(toEntry)),
    addCluster: (entry) => {
      const clusters = load()
      if (clusters.some((c) => c.name === entry.name)) {
        return delayReject(reasonError(409, 'conflict', 'clusters.exists', { name: entry.name }))
      }
      clusters.push({
        id: Math.max(0, ...clusters.map((c) => c.id)) + 1,
        name: entry.name,
        kubeconfigPath: entry.kubeconfigPath ?? (entry.content ? '(stored kubeconfig)' : undefined),
        address: entry.address,
        active: false,
        environment: '',
        readOnly: false,
        // The mock reads no kubeconfig: credentials imply a matching stanza.
        cloudAuth: entry.credentials?.provider,
        cloudCredentials: entry.credentials?.provider,
      })
      save(clusters)
      return delay(undefined)
    },
    removeCluster: (name) => {
      save(load().filter((c) => c.name !== name))
      return delay(undefined)
    },
    activateCluster: (name) => {
      save(load().map((c) => ({ ...c, active: c.name === name })))
      return delay(undefined)
    },
    updateCluster: (id, update) => {
      const clusters = load()
      const target = clusters.find((c) => c.id === id)
      if (!target) return delayReject(reasonError(404, 'not_found', 'clusters.id_not_found', { id }))
      if (update.name && clusters.some((c) => c.id !== id && c.name === update.name)) {
        return delayReject(reasonError(409, 'conflict', 'clusters.exists', { name: update.name }))
      }
      if (update.credentials && target.cloudAuth !== update.credentials.provider) {
        return delayReject(
          new ApiError(400, 'bad_request', `the kubeconfig does not sign in to ${update.credentials.provider}`),
        )
      }
      Object.assign(target, {
        name: update.name ?? target.name,
        environment: update.environment ?? target.environment,
        readOnly: update.readOnly ?? target.readOnly,
      })
      if (update.credentials !== undefined) target.cloudCredentials = update.credentials?.provider
      if (update.allowedNamespaces !== undefined) {
        const { names, invalid } = normalizeAllowedNamespaces(update.allowedNamespaces)
        if (invalid !== undefined) return delayReject(reasonError(400, 'bad_request', 'field.not_namespace', { value: invalid }))
        if (names.length > MAX_ALLOWED_NAMESPACES) {
          return delayReject(reasonError(400, 'bad_request', 'clusters.too_many_namespaces', { max: MAX_ALLOWED_NAMESPACES }))
        }
        target.allowedNamespaces = names
      }
      save(clusters)
      return delay(undefined)
    },
  }
}
