/**
 * The parts of Kubernetes objects the topology reads, as the API returns
 * them (core lists and the /api/k8s passthrough). Every field is optional:
 * the graph is built from whatever an object carries.
 */

export interface ObjectMeta {
  name?: string
  namespace?: string
  labels?: Record<string, string>
  deletionTimestamp?: string
  ownerReferences?: { kind?: string; name?: string; controller?: boolean }[]
}

export interface RawObject {
  kind?: string
  metadata?: ObjectMeta
}

interface ConfigRef {
  name?: string
  optional?: boolean
}

interface KeyRef extends ConfigRef {
  key?: string
}

export interface ContainerRefs {
  name?: string
  envFrom?: { configMapRef?: ConfigRef; secretRef?: ConfigRef }[]
  env?: { valueFrom?: { configMapKeyRef?: KeyRef; secretKeyRef?: KeyRef } }[]
}

export interface Volume {
  name?: string
  persistentVolumeClaim?: { claimName?: string }
  /** A generic ephemeral volume: its PVC is named `<pod>-<volume>`. */
  ephemeral?: unknown
  configMap?: ConfigRef
  secret?: { secretName?: string; optional?: boolean }
  projected?: { sources?: { configMap?: ConfigRef; secret?: ConfigRef }[] }
}

export interface ContainerStatus {
  name?: string
  ready?: boolean
  state?: { waiting?: { reason?: string }; terminated?: { reason?: string } }
}

export interface RawPod extends RawObject {
  spec?: {
    volumes?: Volume[]
    containers?: ContainerRefs[]
    initContainers?: ContainerRefs[]
  }
  status?: {
    phase?: string
    reason?: string
    containerStatuses?: ContainerStatus[]
  }
}

/** Deployment, ReplicaSet and StatefulSet. */
export interface RawReplicated extends RawObject {
  spec?: { replicas?: number }
  status?: { readyReplicas?: number }
}

export interface RawDaemonSet extends RawObject {
  status?: { desiredNumberScheduled?: number; numberReady?: number }
}

export interface RawService extends RawObject {
  spec?: { type?: string; selector?: Record<string, string>; externalName?: string }
}

export interface IngressBackend {
  service?: { name?: string }
}

/** An OpenShift Route: its Service and the alternate backends that share its traffic. */
export interface RawRoute extends RawObject {
  spec?: {
    to?: { kind?: string; name?: string }
    alternateBackends?: { kind?: string; name?: string }[]
  }
}

export interface RawIngress extends RawObject {
  spec?: {
    defaultBackend?: IngressBackend
    rules?: { host?: string; http?: { paths?: { backend?: IngressBackend }[] } }[]
    tls?: { secretName?: string }[]
  }
}

export interface RawPvc extends RawObject {
  spec?: { volumeName?: string }
  status?: { phase?: string }
}

export interface RawPv extends RawObject {
  status?: { phase?: string }
}

/** ConfigMap and Secret. */
export interface RawConfig extends RawObject {
  data?: Record<string, unknown>
  binaryData?: Record<string, unknown>
}

/** Whether `labels` satisfy a Service's equality selector (an empty one selects nothing). */
export function matchesSelector(selector: Record<string, string> | undefined, labels: Record<string, string> | undefined) {
  if (!selector) return false
  const entries = Object.entries(selector)
  return entries.length > 0 && entries.every(([key, value]) => labels?.[key] === value)
}
