/** Map raw Kubernetes API objects to the UI's domain shapes (src/data/types.ts). */

export function formatAge(creationTimestamp) {
  if (!creationTimestamp) return '—'
  const ms = Date.now() - new Date(creationTimestamp).getTime()
  const minutes = Math.floor(ms / 60000)
  if (minutes < 60) return `${Math.max(0, minutes)}m`
  const hours = Math.floor(minutes / 60)
  if (hours < 48) return `${hours}h`
  const days = Math.floor(hours / 24)
  if (days < 365) return `${days}d`
  return `${Math.floor(days / 365)}y${Math.floor((days % 365) / 30)}mo`
}

export function parseCpu(value) {
  if (!value) return 0
  if (value.endsWith('n')) return parseInt(value) / 1e9
  if (value.endsWith('u')) return parseInt(value) / 1e6
  if (value.endsWith('m')) return parseInt(value) / 1e3
  return parseFloat(value)
}

export function parseMemoryMi(value) {
  if (!value) return 0
  const units = { Ki: 1 / 1024, Mi: 1, Gi: 1024, Ti: 1024 * 1024, K: 1e3 / (1024 * 1024), M: 1e6 / (1024 * 1024), G: 1e9 / (1024 * 1024) }
  const match = String(value).match(/^(\d+(?:\.\d+)?)(\w*)$/)
  if (!match) return 0
  const [, num, unit] = match
  return parseFloat(num) * (units[unit] ?? 1 / (1024 * 1024))
}

export function mapNode(n) {
  const ready = (n.status.conditions ?? []).find((c) => c.type === 'Ready')
  const roles = Object.keys(n.metadata.labels ?? {})
    .filter((l) => l.startsWith('node-role.kubernetes.io/'))
    .map((l) => l.split('/')[1])
  const addr = (n.status.addresses ?? []).find((a) => a.type === 'InternalIP')
  return {
    name: n.metadata.name,
    roles: roles.length ? roles : ['worker'],
    status: ready?.status === 'True' ? 'Ready' : 'NotReady',
    version: n.status.nodeInfo?.kubeletVersion ?? '—',
    os: n.status.nodeInfo?.osImage ?? '—',
    kernelVersion: n.status.nodeInfo?.kernelVersion ?? '—',
    containerRuntime: n.status.nodeInfo?.containerRuntimeVersion ?? '—',
    cpuCapacity: Math.round(parseCpu(n.status.capacity?.cpu)),
    memoryCapacityGi: Math.round(parseMemoryMi(n.status.capacity?.memory) / 1024),
    age: formatAge(n.metadata.creationTimestamp),
    internalIP: addr?.address ?? '—',
    labels: n.metadata.labels ?? {},
    annotations: n.metadata.annotations ?? {},
  }
}

export function mapNamespace(ns) {
  return {
    name: ns.metadata.name,
    status: ns.status?.phase ?? 'Active',
    age: formatAge(ns.metadata.creationTimestamp),
  }
}

function containerSpec(c) {
  return {
    name: c.name,
    image: c.image,
    command: c.command,
    args: c.args,
    env: (c.env ?? []).map((e) => ({
      name: e.name,
      value: e.value ?? (e.valueFrom ? `(${Object.keys(e.valueFrom)[0]})` : ''),
    })),
    requests: {
      cpu: c.resources?.requests?.cpu ?? '—',
      memory: c.resources?.requests?.memory ?? '—',
    },
    limits: {
      cpu: c.resources?.limits?.cpu ?? '—',
      memory: c.resources?.limits?.memory ?? '—',
    },
    ports: (c.ports ?? []).map((p) => `${p.containerPort}/${p.protocol ?? 'TCP'}`).join(', ') || undefined,
  }
}

export function mapPod(p) {
  const statuses = p.status?.containerStatuses ?? []
  const waiting = statuses.find((s) => s.state?.waiting)?.state.waiting.reason
  let phase = p.status?.phase ?? 'Unknown'
  if (waiting && ['CrashLoopBackOff', 'ImagePullBackOff', 'ContainerCreating', 'ErrImagePull'].includes(waiting)) {
    phase = waiting === 'ErrImagePull' ? 'ImagePullBackOff' : waiting
  }
  const owner = (p.metadata.ownerReferences ?? [])[0]
  return {
    name: p.metadata.name,
    namespace: p.metadata.namespace,
    status: phase,
    readyContainers: statuses.filter((s) => s.ready).length,
    totalContainers: (p.spec?.containers ?? []).length,
    restarts: statuses.reduce((sum, s) => sum + (s.restartCount ?? 0), 0),
    age: formatAge(p.metadata.creationTimestamp),
    node: p.spec?.nodeName ?? '',
    podIP: p.status?.podIP ?? '',
    controlledBy: owner ? `${owner.kind}/${owner.name}` : undefined,
    labels: p.metadata.labels ?? {},
    containers: (p.spec?.containers ?? []).map(containerSpec),
  }
}

export function mapDeployment(d) {
  return {
    name: d.metadata.name,
    namespace: d.metadata.namespace,
    readyReplicas: d.status?.readyReplicas ?? 0,
    replicas: d.spec?.replicas ?? 0,
    upToDate: d.status?.updatedReplicas ?? 0,
    available: d.status?.availableReplicas ?? 0,
    age: formatAge(d.metadata.creationTimestamp),
    labels: d.spec?.selector?.matchLabels ?? d.metadata.labels ?? {},
  }
}

export function mapService(s) {
  const ports = (s.spec?.ports ?? [])
    .map((p) => (p.nodePort ? `${p.port}:${p.nodePort}/${p.protocol}` : `${p.port}/${p.protocol}`))
    .join(',')
  const lbIngress = s.status?.loadBalancer?.ingress?.[0]
  return {
    name: s.metadata.name,
    namespace: s.metadata.namespace,
    type: s.spec?.type ?? 'ClusterIP',
    clusterIP: s.spec?.clusterIP ?? '—',
    externalIP: lbIngress?.ip ?? lbIngress?.hostname ?? undefined,
    ports,
    age: formatAge(s.metadata.creationTimestamp),
  }
}

export function mapEvent(e) {
  return {
    type: e.type ?? 'Normal',
    reason: e.reason ?? '—',
    message: e.message ?? '',
    age: formatAge(e.lastTimestamp ?? e.eventTime ?? e.metadata.creationTimestamp),
    count: e.count ?? 1,
    source: e.source?.component ?? e.reportingComponent ?? '—',
  }
}
