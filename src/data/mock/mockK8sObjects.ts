import type { Deployment, KubeObject, Pod, Service } from '../types'
import { genericMock } from './mockGeneric'

/**
 * Kubernetes-shaped Pods, Deployments and Services of the mock dataset,
 * as the read-only passthrough (GET /api/k8s) returns them: with the owner
 * references, selectors, volumes and config references that tie them
 * together (the topology view is built from these), consistent with the
 * generic mock objects (ReplicaSets, PVCs, ConfigMaps, Secrets, Ingresses).
 */

/** Pod labels each Service selects, by namespace/name; `kubernetes` selects nothing. */
const SERVICE_SELECTORS: Record<string, Record<string, string>> = {
  'kube-system/kube-dns': { app: 'coredns' },
  'cert-manager/cert-manager-webhook': { app: 'webhook' },
  'monitoring/prometheus-server': { app: 'prometheus' },
  'shop/shop-frontend': { app: 'shop-frontend' },
  'shop/shop-cart': { app: 'shop-cart' },
  'shop/shop-postgres': { app: 'postgres' },
}

interface PodRefs {
  volumes?: Record<string, unknown>[]
  envFrom?: Record<string, unknown>[]
  env?: Record<string, unknown>[]
}

/**
 * Volumes and config references of each app's Pods, by namespace/app.
 * shop-payments reads a Secret that does not exist, which is why it is
 * crash-looping.
 */
const POD_REFS: Record<string, PodRefs> = {
  'kube-system/coredns': { volumes: [{ name: 'config-volume', configMap: { name: 'coredns' } }] },
  'monitoring/prometheus': {
    volumes: [
      { name: 'config-volume', configMap: { name: 'prometheus-server' } },
      { name: 'storage-volume', persistentVolumeClaim: { claimName: 'prometheus-data' } },
    ],
  },
  'shop/shop-frontend': { envFrom: [{ configMapRef: { name: 'shop-frontend-config' } }] },
  'shop/shop-cart': { volumes: [{ name: 'uploads', persistentVolumeClaim: { claimName: 'shop-uploads' } }] },
  'shop/shop-payments': {
    env: [{ name: 'GATEWAY_API_KEY', valueFrom: { secretKeyRef: { name: 'shop-payments-keys', key: 'api-key' } } }],
  },
  'shop/postgres': { volumes: [{ name: 'data', persistentVolumeClaim: { claimName: 'shop-postgres-data' } }] },
}

/** Every Pod's service account token volume, as the API server injects it. */
function tokenVolume(pod: Pod): Record<string, unknown> {
  return {
    name: `kube-api-access-${pod.name.slice(-5)}`,
    projected: { sources: [{ serviceAccountToken: { path: 'token' } }, { configMap: { name: 'kube-root-ca.crt' } }] },
  }
}

const ownerRef = (kind: string, name: string) => ({
  apiVersion: 'apps/v1',
  kind,
  name,
  controller: true,
  blockOwnerDeletion: true,
})

/** The ReplicaSet of a Deployment that currently runs its Pods. */
function currentReplicaSet(namespace: string, deployment: string): string | undefined {
  return (genericMock.replicasets ?? []).find((rs) => {
    const spec = rs.spec as { replicas?: number } | undefined
    const owners = rs.metadata?.ownerReferences as { name?: string }[] | undefined
    return rs.metadata?.namespace === namespace && owners?.[0]?.name === deployment && (spec?.replicas ?? 0) > 0
  })?.metadata?.name
}

function podOwners(pod: Pod): Record<string, unknown>[] {
  const [kind, name] = (pod.controlledBy ?? '').split('/')
  if (kind === 'Deployment') {
    const rs = currentReplicaSet(pod.namespace, name)
    return rs ? [ownerRef('ReplicaSet', rs)] : []
  }
  return kind && name ? [ownerRef(kind, name)] : []
}

function containerState(pod: Pod, ready: boolean): Record<string, unknown> {
  switch (pod.status) {
    case 'CrashLoopBackOff':
      return ready ? { running: {} } : { waiting: { reason: 'CrashLoopBackOff' } }
    case 'ContainerCreating':
      return { waiting: { reason: 'ContainerCreating' } }
    case 'Succeeded':
      return { terminated: { reason: 'Completed', exitCode: 0 } }
    case 'Failed':
      return { terminated: { reason: 'Error', exitCode: 1 } }
    default:
      return { running: {} }
  }
}

/** Phase and container states the API reports for the mock's display status. */
function podStatus(pod: Pod): Record<string, unknown> {
  const phase = pod.status === 'CrashLoopBackOff' ? 'Running' : pod.status === 'ContainerCreating' ? 'Pending' : pod.status
  // A Pending Pod of the mock is unscheduled: it has no containers yet.
  const containerStatuses =
    pod.status === 'Pending'
      ? []
      : pod.containers.map((c, i) => ({
          name: c.name,
          image: c.image,
          ready: i < pod.readyContainers,
          restartCount: i === 0 ? pod.restarts : 0,
          state: containerState(pod, i < pod.readyContainers),
        }))
  return { phase, podIP: pod.podIP || undefined, containerStatuses }
}

export function podObject(pod: Pod): KubeObject {
  const refs = POD_REFS[`${pod.namespace}/${pod.labels.app}`] ?? {}
  return {
    kind: 'Pod',
    apiVersion: 'v1',
    metadata: { name: pod.name, namespace: pod.namespace, labels: pod.labels, ownerReferences: podOwners(pod) },
    spec: {
      nodeName: pod.node || undefined,
      volumes: [...(refs.volumes ?? []), tokenVolume(pod)],
      containers: pod.containers.map((c) => ({ name: c.name, image: c.image, envFrom: refs.envFrom, env: refs.env })),
    },
    status: podStatus(pod),
    age: pod.age,
  }
}

/** The labels of a Deployment's Pods (the mock links them through controlledBy). */
function podLabels(d: Deployment, pods: Pod[]): Record<string, string> {
  return pods.find((p) => p.namespace === d.namespace && p.controlledBy === `Deployment/${d.name}`)?.labels ?? d.labels
}

export function deploymentObject(d: Deployment, pods: Pod[]): KubeObject {
  return {
    kind: 'Deployment',
    apiVersion: 'apps/v1',
    metadata: { name: d.name, namespace: d.namespace, labels: d.labels },
    spec: { replicas: d.replicas, selector: { matchLabels: podLabels(d, pods) } },
    status: { replicas: d.replicas, updatedReplicas: d.upToDate, readyReplicas: d.readyReplicas, availableReplicas: d.available },
    age: d.age,
  }
}

export function serviceObject(s: Service): KubeObject {
  return {
    kind: 'Service',
    apiVersion: 'v1',
    metadata: { name: s.name, namespace: s.namespace, labels: {} },
    spec: { type: s.type, clusterIP: s.clusterIP, selector: SERVICE_SELECTORS[`${s.namespace}/${s.name}`] },
    age: s.age,
  }
}
