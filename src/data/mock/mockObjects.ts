import { load } from 'js-yaml'
import type { ContainerSpec, DeletePropagation, KubeObject } from '../types'
import { deployments, namespaces, nodes, pods, services, syncDeploymentPods } from './mockCluster'
import { crds, customResources } from './mockCrds'
import { genericMock } from './mockGeneric'
import type { MockKind } from './mockKinds'
import { customResourceYaml, deploymentYaml, nodeYaml, podYaml, serviceYaml } from './mockYaml'
import { isObject, isRedacted, metadataOf, stringMap, valueAt, type Manifest } from './manifest'
import { secretValue } from './mockSecrets'

/**
 * The mock cluster's objects as full manifests, for the resource editing
 * provider (design §7.2). The lists the rest of the mock serves stay the
 * source of truth for what they model: an applied Deployment updates
 * `deployments` (and its Pods), an applied ConfigMap replaces its entry in
 * `genericMock`, so created and edited objects show up in every list. What
 * the typed lists do not model (a Deployment's containers, say) is kept
 * from the applied manifest.
 */

/** Manifests applied to typed kinds and custom resources, by objectKey. */
const applied = new Map<string, Manifest>()
/** Secret values by "namespace/name" → key → text; seeded Secrets fall back to secretValue(). */
const secretVault = new Map<string, Record<string, string>>()

export function objectKey(kind: MockKind, namespace: string, name: string): string {
  return `${kind.apiVersion}/${kind.kind}/${namespace}/${name}`
}

const sameObject = (namespace: string, name: string) => (o: { namespace?: string; name: string }) =>
  o.name === name && (o.namespace ?? '') === namespace

// --- reading ----------------------------------------------------------------

/** The object's current manifest, or undefined when the mock cluster has none. */
export function readObject(kind: MockKind, namespace: string, name: string): Manifest | undefined {
  return kind.key ? readGeneric(kind, namespace, name) : readTyped(kind, namespace, name)
}

function readGeneric(kind: MockKind, namespace: string, name: string): Manifest | undefined {
  const found = (genericMock[kind.key!] ?? []).find(
    (o) => o.metadata?.name === name && (o.metadata?.namespace ?? '') === namespace,
  )
  if (!found) return undefined
  const manifest: KubeObject = { apiVersion: kind.apiVersion, kind: kind.kind, ...structuredClone(found) }
  delete manifest.age // synthetic, not part of the object
  return manifest
}

/** An applied manifest, else the one the hand-written mock YAML describes. */
function baseManifest(key: string, yamlText: () => string): Manifest {
  const stored = applied.get(key)
  return stored ? structuredClone(stored) : (load(yamlText()) as Manifest)
}

function readTyped(kind: MockKind, namespace: string, name: string): Manifest | undefined {
  const key = objectKey(kind, namespace, name)
  switch (kind.kind) {
    case 'Pod': {
      const pod = pods.find(sameObject(namespace, name))
      if (!pod) return undefined
      const manifest = baseManifest(key, () => podYaml(pod))
      metadataOf(manifest).labels = { ...pod.labels }
      return manifest
    }
    case 'Deployment': {
      const d = deployments.find(sameObject(namespace, name))
      if (!d) return undefined
      const manifest = baseManifest(key, () => deploymentYaml(d))
      metadataOf(manifest).labels = { ...d.labels }
      manifest.spec = { ...(isObject(manifest.spec) ? manifest.spec : {}), replicas: d.replicas }
      manifest.status = {
        replicas: d.replicas,
        readyReplicas: d.readyReplicas,
        updatedReplicas: d.upToDate,
        availableReplicas: d.available,
      }
      return manifest
    }
    case 'Service': {
      const s = services.find(sameObject(namespace, name))
      if (!s) return undefined
      const manifest = baseManifest(key, () => serviceYaml(s))
      manifest.spec = { ...(isObject(manifest.spec) ? manifest.spec : {}), type: s.type, clusterIP: s.clusterIP }
      return manifest
    }
    case 'Node': {
      const n = nodes.find((x) => x.name === name)
      if (!n) return undefined
      const manifest = baseManifest(key, () => nodeYaml(n))
      Object.assign(metadataOf(manifest), { labels: { ...n.labels }, annotations: { ...n.annotations } })
      return manifest
    }
    case 'Namespace': {
      const ns = namespaces.find((x) => x.name === name)
      if (!ns) return undefined
      const manifest = applied.get(key)
        ? structuredClone(applied.get(key)!)
        : { apiVersion: 'v1', kind: 'Namespace', metadata: { name }, spec: { finalizers: ['kubernetes'] } }
      manifest.status = { phase: ns.status }
      return manifest
    }
    default: {
      const cr = customResources.find((x) => x.name === name && (x.namespace ?? '') === namespace && crdOf(kind)?.name === x.crdName)
      if (!cr) return undefined
      return baseManifest(key, () => customResourceYaml(cr, kind.apiVersion, kind.kind))
    }
  }
}

const crdOf = (kind: MockKind) => crds.find((c) => c.kind === kind.kind && kind.apiVersion.startsWith(`${c.group}/`))

// --- writing ----------------------------------------------------------------

/** Makes manifest the object's new state, creating it when it does not exist. */
export function writeObject(kind: MockKind, namespace: string, name: string, manifest: Manifest): void {
  if (kind.key) {
    writeGeneric(kind, namespace, name, manifest)
    return
  }
  const stored = structuredClone(manifest)
  delete stored.status // the typed lists own the status
  applied.set(objectKey(kind, namespace, name), stored)
  const labels = stringMap(metadataOf(manifest).labels)
  switch (kind.kind) {
    case 'Pod':
      return writePod(namespace, name, labels, manifest)
    case 'Deployment':
      return writeDeployment(namespace, name, labels, manifest)
    case 'Service':
      return writeService(namespace, name, manifest)
    case 'Node':
      return writeNode(name, labels, stringMap(metadataOf(manifest).annotations))
    case 'Namespace':
      if (!namespaces.some((x) => x.name === name)) namespaces.push({ name, status: 'Active', age: '5s' })
      return
    default: {
      const crd = crdOf(kind)
      if (crd && !customResources.some((x) => x.name === name && (x.namespace ?? '') === namespace && x.crdName === crd.name)) {
        customResources.push({ name, namespace: namespace || undefined, crdName: crd.name, status: 'Active', age: '5s', extra: {} })
      }
    }
  }
}

function writePod(namespace: string, name: string, labels: Record<string, string>, manifest: Manifest) {
  const existing = pods.find(sameObject(namespace, name))
  if (existing) {
    existing.labels = labels
    return
  }
  const specs = valueAt(manifest, 'spec.containers')
  const containers: ContainerSpec[] = (Array.isArray(specs) ? specs : []).filter(isObject).map((c) => ({
    name: String(c.name ?? 'main'),
    image: String(c.image ?? ''),
    env: [],
    requests: { cpu: '—', memory: '—' },
    limits: { cpu: '—', memory: '—' },
  }))
  pods.push({
    name,
    namespace,
    status: 'Running',
    readyContainers: containers.length,
    totalContainers: containers.length,
    restarts: 0,
    age: '5s',
    node: 'kb-worker-1',
    podIP: '10.244.1.98',
    labels,
    containers,
  })
}

function writeDeployment(namespace: string, name: string, labels: Record<string, string>, manifest: Manifest) {
  const wanted = valueAt(manifest, 'spec.replicas')
  const replicas = typeof wanted === 'number' && wanted >= 0 ? wanted : 1
  let d = deployments.find(sameObject(namespace, name))
  if (!d) {
    d = { name, namespace, readyReplicas: 0, replicas: 0, upToDate: 0, available: 0, age: '5s', labels }
    deployments.push(d)
  }
  d.labels = labels
  if (replicas !== d.replicas) {
    syncDeploymentPods(d, replicas)
    // New Pods start ready in the mock; removed ones take readiness with them.
    d.readyReplicas = Math.min(replicas, d.readyReplicas + Math.max(0, replicas - d.replicas))
    d.available = d.readyReplicas
    d.replicas = replicas
    d.upToDate = replicas
  }
  const app = deploymentApp(d)
  for (const pod of pods) {
    if (pod.namespace === namespace && pod.labels.app === app) pod.controlledBy ??= `Deployment/${name}`
  }
}

/** The app label a mock Deployment's Pods carry (see syncDeploymentPods). */
const deploymentApp = (d: { name: string; labels: Record<string, string> }) =>
  d.labels.app ?? d.labels['k8s-app'] ?? d.name

let serviceIPs = 0

function writeService(namespace: string, name: string, manifest: Manifest) {
  const type = valueAt(manifest, 'spec.type')
  const ports = valueAt(manifest, 'spec.ports')
  const portText = (Array.isArray(ports) ? ports : [])
    .filter(isObject)
    .map((p) => `${p.port}/${p.protocol ?? 'TCP'}`)
    .join(',')
  const existing = services.find(sameObject(namespace, name))
  const serviceType = type === 'NodePort' || type === 'LoadBalancer' || type === 'ExternalName' ? type : 'ClusterIP'
  if (existing) {
    existing.type = serviceType
    if (portText) existing.ports = portText
    return
  }
  serviceIPs += 1
  services.push({
    name,
    namespace,
    type: serviceType,
    clusterIP: `10.96.${200 + serviceIPs}.${10 + serviceIPs}`,
    ports: portText || '—',
    age: '5s',
  })
}

function writeNode(name: string, labels: Record<string, string>, annotations: Record<string, string>) {
  const existing = nodes.find((x) => x.name === name)
  if (existing) {
    existing.labels = labels
    existing.annotations = annotations
    return
  }
  nodes.push({
    name,
    roles: [],
    status: 'NotReady', // no kubelet behind an object created by hand
    version: '',
    os: '',
    kernelVersion: '',
    containerRuntime: '',
    cpuCapacity: 0,
    memoryCapacityGi: 0,
    age: '5s',
    internalIP: '',
    labels,
    annotations,
  })
}

function writeGeneric(kind: MockKind, namespace: string, name: string, manifest: Manifest) {
  const list = (genericMock[kind.key!] ??= [])
  const index = list.findIndex((o) => o.metadata?.name === name && (o.metadata?.namespace ?? '') === namespace)
  const stored: KubeObject = { ...structuredClone(manifest), apiVersion: kind.apiVersion, kind: kind.kind }
  stored.age = index >= 0 ? list[index].age : '5s'
  if (kind.kind === 'Secret') vaultSecret(namespace, name, stored)
  if (index >= 0) list[index] = stored
  else list.push(stored)
}

const byteLength = (text: string) => new TextEncoder().encode(text).length

function decodeBase64(value: string): string {
  try {
    const binary = atob(value)
    return new TextDecoder().decode(Uint8Array.from(binary, (c) => c.charCodeAt(0)))
  } catch {
    return value // not base64: keep what was given
  }
}

/**
 * Moves a Secret's values into the vault and leaves the redaction
 * placeholders the generic lists show (as the live API's reads do). A
 * placeholder in `data` (a key the manifest did not name, merged from the
 * old object) keeps that key's value.
 */
function vaultSecret(namespace: string, name: string, secret: KubeObject) {
  const id = `${namespace}/${name}`
  const values: Record<string, string> = {}
  for (const [key, value] of Object.entries(stringMap(secret.data))) {
    values[key] = isRedacted(value) ? revealedValue(namespace, name, key) : decodeBase64(value)
  }
  Object.assign(values, stringMap(secret.stringData)) // write-only, merged into data like the API server does
  delete secret.stringData
  secretVault.set(id, values)
  secret.data = Object.fromEntries(Object.entries(values).map(([key, value]) => [key, `<${byteLength(value)} bytes, redacted>`]))
}

/** The real value of a Secret key: what was applied, else the seeded demo value. */
export function revealedValue(namespace: string, name: string, key: string): string {
  return secretVault.get(`${namespace}/${name}`)?.[key] ?? secretValue(namespace, name, key)
}

// --- deleting ---------------------------------------------------------------

/**
 * Deletes the object; false when there was none. Background and
 * Foreground delete the Pods it controls, Orphan leaves them running
 * without an owner.
 */
export function removeObject(kind: MockKind, namespace: string, name: string, propagation: DeletePropagation): boolean {
  applied.delete(objectKey(kind, namespace, name))
  const cascade = propagation !== 'Orphan'
  if (kind.key) {
    const list = genericMock[kind.key] ?? []
    const index = list.findIndex((o) => o.metadata?.name === name && (o.metadata?.namespace ?? '') === namespace)
    if (index < 0) return false
    list.splice(index, 1)
    secretVault.delete(`${namespace}/${name}`)
    releasePods(namespace, (p) => p.controlledBy === `${kind.kind}/${name}`, cascade)
    return true
  }
  switch (kind.kind) {
    case 'Pod':
      return splice(pods, sameObject(namespace, name))
    case 'Deployment': {
      const d = deployments.find(sameObject(namespace, name))
      if (!d) return false
      const app = deploymentApp(d)
      splice(deployments, (x) => x === d)
      releasePods(namespace, (p) => p.labels.app === app, cascade)
      return true
    }
    case 'Service':
      return splice(services, sameObject(namespace, name))
    case 'Node':
      return splice(nodes, (x) => x.name === name)
    case 'Namespace':
      return splice(namespaces, (x) => x.name === name)
    default: {
      const crd = crdOf(kind)
      return splice(customResources, (x) => x.name === name && (x.namespace ?? '') === namespace && x.crdName === crd?.name)
    }
  }
}

function splice<T>(list: T[], match: (item: T) => boolean): boolean {
  const index = list.findIndex(match)
  if (index < 0) return false
  list.splice(index, 1)
  return true
}

/** Deletes the matching Pods of a deleted owner, or orphans them. */
function releasePods(namespace: string, owned: (pod: (typeof pods)[number]) => boolean, cascade: boolean) {
  for (let i = pods.length - 1; i >= 0; i--) {
    const pod = pods[i]
    if (pod.namespace !== namespace || !owned(pod)) continue
    if (cascade) pods.splice(i, 1)
    else pod.controlledBy = undefined
  }
}
