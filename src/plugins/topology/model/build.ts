import {
  matchesSelector,
  type ContainerRefs,
  type RawConfig,
  type RawDaemonSet,
  type RawIngress,
  type RawObject,
  type RawPod,
  type RawPv,
  type RawPvc,
  type RawReplicated,
  type RawService,
} from './k8s'
import {
  configStatus,
  daemonSetStatus,
  ingressStatus,
  MISSING,
  podStatus,
  pvcStatus,
  pvStatus,
  replicatedStatus,
  serviceStatus,
  UNKNOWN,
} from './status'
import {
  nodeId,
  type BuildOptions,
  type EdgeKind,
  type NodeStatus,
  type TopologyGraph,
  type TopologyKind,
  type TopologyNode,
  type TopologySnapshot,
} from './types'

/** Volumes every Pod gets for its service account token; they reference kube-root-ca.crt only. */
const TOKEN_VOLUME_PREFIX = 'kube-api-access-'

/** Kinds that own Pods or ReplicaSets in the graph. */
const OWNER_KINDS = new Set<TopologyKind>(['Deployment', 'ReplicaSet', 'StatefulSet', 'DaemonSet'])

const nameOf = (o: { metadata?: { name?: string } }) => o.metadata?.name ?? ''

/** Accumulates nodes and de-duplicated edges. */
class GraphBuilder {
  readonly nodes = new Map<string, TopologyNode>()
  readonly edges = new Map<string, { id: string; from: string; to: string; kind: EdgeKind }>()

  add(kind: TopologyKind, namespace: string, name: string, status: NodeStatus): string {
    const id = nodeId(kind, namespace, name)
    this.nodes.set(id, { id, kind, namespace, name, status, missing: status === MISSING })
    return id
  }

  /**
   * The id of a referenced object: the listed node, a missing placeholder
   * when its list loaded without it, or a node of unknown status while
   * that list has not loaded.
   */
  ref(kind: TopologyKind, namespace: string, name: string, loaded: boolean): string {
    const id = nodeId(kind, namespace, name)
    if (!this.nodes.has(id)) this.add(kind, namespace, name, loaded ? MISSING : UNKNOWN)
    return id
  }

  link(from: string, to: string, kind: EdgeKind): void {
    const id = `${from}>${to}`
    if (!this.edges.has(id)) this.edges.set(id, { id, from, to, kind })
  }
}

/** ConfigMaps and Secrets a container reads through envFrom or env valueFrom (optional ones only if present). */
function containerRefs(c: ContainerRefs, add: (kind: 'ConfigMap' | 'Secret', name: string, optional: boolean) => void) {
  for (const from of c.envFrom ?? []) {
    if (from.configMapRef?.name) add('ConfigMap', from.configMapRef.name, !!from.configMapRef.optional)
    if (from.secretRef?.name) add('Secret', from.secretRef.name, !!from.secretRef.optional)
  }
  for (const env of c.env ?? []) {
    const cm = env.valueFrom?.configMapKeyRef
    const secret = env.valueFrom?.secretKeyRef
    if (cm?.name) add('ConfigMap', cm.name, !!cm.optional)
    if (secret?.name) add('Secret', secret.name, !!secret.optional)
  }
}

/** Every ConfigMap or Secret a Pod uses: volumes (also projected), envFrom and env. */
export function podConfigRefs(pod: RawPod): { kind: 'ConfigMap' | 'Secret'; name: string; optional: boolean }[] {
  const out = new Map<string, { kind: 'ConfigMap' | 'Secret'; name: string; optional: boolean }>()
  const add = (kind: 'ConfigMap' | 'Secret', name: string, optional: boolean) => {
    const key = `${kind}/${name}`
    const known = out.get(key)
    // A reference that is required anywhere is required.
    out.set(key, { kind, name, optional: known ? known.optional && optional : optional })
  }
  for (const v of pod.spec?.volumes ?? []) {
    if (v.configMap?.name) add('ConfigMap', v.configMap.name, !!v.configMap.optional)
    if (v.secret?.secretName) add('Secret', v.secret.secretName, !!v.secret.optional)
    if (v.projected && !v.name?.startsWith(TOKEN_VOLUME_PREFIX)) {
      for (const source of v.projected.sources ?? []) {
        if (source.configMap?.name) add('ConfigMap', source.configMap.name, !!source.configMap.optional)
        if (source.secret?.name) add('Secret', source.secret.name, !!source.secret.optional)
      }
    }
  }
  for (const c of [...(pod.spec?.initContainers ?? []), ...(pod.spec?.containers ?? [])]) containerRefs(c, add)
  return [...out.values()]
}

/** PVC names a Pod mounts, including the claims of generic ephemeral volumes. */
export function podClaims(pod: RawPod): string[] {
  const claims: string[] = []
  for (const v of pod.spec?.volumes ?? []) {
    if (v.persistentVolumeClaim?.claimName) claims.push(v.persistentVolumeClaim.claimName)
    else if (v.ephemeral && v.name) claims.push(`${pod.metadata?.name ?? ''}-${v.name}`)
  }
  return claims
}

/** Services an Ingress routes to: every rule path and the default backend. */
export function ingressBackends(ing: RawIngress): string[] {
  const names = new Set<string>()
  const backend = ing.spec?.defaultBackend?.service?.name
  if (backend) names.add(backend)
  for (const rule of ing.spec?.rules ?? []) {
    for (const path of rule.http?.paths ?? []) {
      const name = path.backend?.service?.name
      if (name) names.add(name)
    }
  }
  return [...names]
}

/** The controller owner of an object among the kinds the graph shows. */
function controllerOf(obj: RawObject): { kind: TopologyKind; name: string } | null {
  const refs = obj.metadata?.ownerReferences ?? []
  const ref = refs.find((r) => r.controller) ?? refs[0]
  if (!ref?.kind || !ref.name || !OWNER_KINDS.has(ref.kind as TopologyKind)) return null
  return { kind: ref.kind as TopologyKind, name: ref.name }
}

/**
 * Builds the namespace's graph (design §7.9): Ingress → Service by backend
 * name, Service → Pod by selector, owner → ReplicaSet/Pod by controller
 * ownerReference, Pod → PVC → PV, and optionally Pod/Ingress → ConfigMap
 * and Secret. Pure and deterministic: nodes and edges come out sorted, so
 * the same lists always give the same graph.
 */
export function buildGraph(namespace: string, snap: TopologySnapshot, options: BuildOptions): TopologyGraph {
  const g = new GraphBuilder()
  const pods = (snap.pods ?? []) as RawPod[]

  // Workloads first, so the Pods' owner references find them.
  for (const d of snap.deployments ?? []) g.add('Deployment', namespace, nameOf(d), replicatedStatus(d as RawReplicated))
  for (const s of snap.statefulSets ?? []) g.add('StatefulSet', namespace, nameOf(s), replicatedStatus(s as RawReplicated))
  for (const d of snap.daemonSets ?? []) g.add('DaemonSet', namespace, nameOf(d), daemonSetStatus(d as RawDaemonSet))

  const owning = new Set(pods.map((p) => controllerOf(p)).filter((o) => o?.kind === 'ReplicaSet').map((o) => o!.name))
  for (const rs of (snap.replicaSets ?? []) as RawReplicated[]) {
    const name = nameOf(rs)
    if (!options.showInactiveReplicaSets && (rs.spec?.replicas ?? 1) === 0 && !owning.has(name)) continue
    const id = g.add('ReplicaSet', namespace, name, replicatedStatus(rs))
    const owner = controllerOf(rs)
    const ownerId = owner && nodeId(owner.kind, namespace, owner.name)
    if (ownerId && g.nodes.has(ownerId)) g.link(ownerId, id, 'owns')
  }

  const podEntries = pods.map((pod) => ({ pod, id: nodeId('Pod', namespace, nameOf(pod)), status: podStatus(pod) }))
  for (const { pod, id, status } of podEntries) {
    g.add('Pod', namespace, nameOf(pod), status)
    const owner = controllerOf(pod)
    const ownerId = owner && nodeId(owner.kind, namespace, owner.name)
    if (ownerId && g.nodes.has(ownerId)) g.link(ownerId, id, 'owns')
  }

  // Storage: every claim of the namespace, the PVs bound to them, and claims Pods mount.
  const pvs = new Map(((snap.pvs ?? []) as RawPv[]).map((pv) => [pv.metadata?.name ?? '', pv]))
  for (const pvc of (snap.pvcs ?? []) as RawPvc[]) {
    const id = g.add('PersistentVolumeClaim', namespace, nameOf(pvc), pvcStatus(pvc))
    const volume = pvc.spec?.volumeName
    if (!volume) continue
    const pv = pvs.get(volume)
    const pvId = pv ? g.add('PersistentVolume', '', volume, pvStatus(pv)) : g.ref('PersistentVolume', '', volume, !!snap.pvs)
    g.link(id, pvId, 'binds')
  }
  for (const { pod, id } of podEntries) {
    for (const claim of podClaims(pod)) {
      g.link(id, g.ref('PersistentVolumeClaim', namespace, claim, !!snap.pvcs), 'mounts')
    }
  }

  // ConfigMaps and Secrets appear only where something references them.
  const configs = {
    ConfigMap: { loaded: !!snap.configMaps, byName: new Map((snap.configMaps ?? []).map((o) => [nameOf(o), o as RawConfig])) },
    Secret: { loaded: !!snap.secrets, byName: new Map((snap.secrets ?? []).map((o) => [nameOf(o), o as RawConfig])) },
  }
  const configNode = (kind: 'ConfigMap' | 'Secret', name: string) => {
    const obj = configs[kind].byName.get(name)
    return obj ? g.add(kind, namespace, name, configStatus(obj)) : g.ref(kind, namespace, name, configs[kind].loaded)
  }
  if (options.showConfig) {
    for (const { pod, id } of podEntries) {
      for (const ref of podConfigRefs(pod)) {
        if (ref.optional && !configs[ref.kind].byName.has(ref.name)) continue
        g.link(id, configNode(ref.kind, ref.name), 'references')
      }
    }
  }

  // Services by selector; their status follows the Pods they select.
  for (const svc of (snap.services ?? []) as RawService[]) {
    const id = nodeId('Service', namespace, nameOf(svc))
    const selected = podEntries.filter((p) => matchesSelector(svc.spec?.selector, p.pod.metadata?.labels))
    const tones = snap.pods ? selected.map((p) => p.status.tone) : undefined
    g.add('Service', namespace, nameOf(svc), serviceStatus(svc, tones))
    for (const pod of selected) g.link(id, pod.id, 'selects')
  }

  const listedServices = new Set((snap.services ?? []).map(nameOf))
  for (const ing of (snap.ingresses ?? []) as RawIngress[]) {
    const backends = ingressBackends(ing)
    const status = snap.services
      ? ingressStatus(backends.length, backends.filter((name) => !listedServices.has(name)).length)
      : UNKNOWN
    const id = g.add('Ingress', namespace, nameOf(ing), status)
    for (const name of backends) g.link(id, g.ref('Service', namespace, name, !!snap.services), 'routes')
    if (!options.showConfig) continue
    for (const tls of ing.spec?.tls ?? []) {
      if (tls.secretName) g.link(id, configNode('Secret', tls.secretName), 'references')
    }
  }

  return {
    nodes: [...g.nodes.values()].sort((a, b) => a.id.localeCompare(b.id)),
    edges: [...g.edges.values()].sort((a, b) => a.id.localeCompare(b.id)),
  }
}
