import type { KubeObject } from '../types'

interface EventSpec {
  namespace: string
  type: 'Normal' | 'Warning'
  reason: string
  message: string
  count: number
  /** Last seen (synthetic age, like the other mock objects). */
  age: string
  object: { kind: string; name: string }
  source: string
}

/**
 * Cluster-wide events of the demo cluster (the Events list, the overview's
 * recent warnings). They tell the same story as the per-object events in
 * mockEvents.ts: a crash-looping payments Pod, an unschedulable frontend
 * replica and an OOM on kb-worker-2, among routine lifecycle events.
 */
const SPECS: EventSpec[] = [
  { namespace: 'shop', type: 'Warning', reason: 'BackOff', message: 'Back-off restarting failed container shop-payments in pod shop-payments-79c6cd8b96-t2m8j', count: 14, age: '2m', object: { kind: 'Pod', name: 'shop-payments-79c6cd8b96-t2m8j' }, source: 'kubelet' },
  { namespace: 'shop', type: 'Warning', reason: 'Unhealthy', message: 'Startup probe failed: Get "http://10.244.2.25:9102/healthz": context deadline exceeded', count: 14, age: '3m', object: { kind: 'Pod', name: 'shop-payments-79c6cd8b96-t2m8j' }, source: 'kubelet' },
  { namespace: 'shop', type: 'Warning', reason: 'FailedScheduling', message: '0/3 nodes are available: 3 Insufficient cpu. preemption: 0/3 nodes are available: 3 No preemption victims found for incoming pod.', count: 12, age: '55s', object: { kind: 'Pod', name: 'shop-frontend-745bd7c77b-w9tqk' }, source: 'default-scheduler' },
  { namespace: 'default', type: 'Warning', reason: 'SystemOOM', message: 'System OOM encountered, victim process: shop-payments (pid 21433)', count: 2, age: '38m', object: { kind: 'Node', name: 'kb-worker-2' }, source: 'kubelet' },
  { namespace: 'monitoring', type: 'Warning', reason: 'Unhealthy', message: 'Readiness probe failed: HTTP probe failed with statuscode: 503', count: 3, age: '17m', object: { kind: 'Pod', name: 'prometheus-server-6bcb7d6b8f-lv4dk' }, source: 'kubelet' },
  { namespace: 'cert-manager', type: 'Warning', reason: 'FailedMount', message: 'MountVolume.SetUp failed for volume "webhook-tls" : secret "cert-manager-webhook-ca" not found', count: 4, age: '77d', object: { kind: 'Pod', name: 'cert-manager-webhook-7f9f8648b9-fjp5c' }, source: 'kubelet' },
  { namespace: 'shop', type: 'Normal', reason: 'Pulled', message: 'Container image "ghcr.io/acme/shop-payments:3.1.0-rc.2" already present on machine', count: 15, age: '3m', object: { kind: 'Pod', name: 'shop-payments-79c6cd8b96-t2m8j' }, source: 'kubelet' },
  { namespace: 'shop', type: 'Normal', reason: 'Created', message: 'Created container shop-payments', count: 15, age: '3m', object: { kind: 'Pod', name: 'shop-payments-79c6cd8b96-t2m8j' }, source: 'kubelet' },
  { namespace: 'shop', type: 'Normal', reason: 'Started', message: 'Started container shop-payments', count: 15, age: '3m', object: { kind: 'Pod', name: 'shop-payments-79c6cd8b96-t2m8j' }, source: 'kubelet' },
  { namespace: 'shop', type: 'Normal', reason: 'ScalingReplicaSet', message: 'Scaled up replica set shop-frontend-745bd7c77b to 3 from 2', count: 1, age: '26m', object: { kind: 'Deployment', name: 'shop-frontend' }, source: 'deployment-controller' },
  { namespace: 'shop', type: 'Normal', reason: 'SuccessfulCreate', message: 'Created pod: shop-frontend-745bd7c77b-w9tqk', count: 1, age: '26m', object: { kind: 'ReplicaSet', name: 'shop-frontend-745bd7c77b' }, source: 'replicaset-controller' },
  { namespace: 'shop', type: 'Normal', reason: 'Scheduled', message: 'Successfully assigned shop/shop-payments-79c6cd8b96-t2m8j to kb-worker-2', count: 1, age: '41m', object: { kind: 'Pod', name: 'shop-payments-79c6cd8b96-t2m8j' }, source: 'default-scheduler' },
  { namespace: 'kube-system', type: 'Normal', reason: 'LeaderElection', message: 'kb-control-plane became leader', count: 1, age: '6h', object: { kind: 'Lease', name: 'kube-scheduler' }, source: 'default-scheduler' },
  { namespace: 'monitoring', type: 'Normal', reason: 'Killing', message: 'Stopping container prometheus-server', count: 1, age: '18m', object: { kind: 'Pod', name: 'prometheus-server-6bcb7d6b8f-lv4dk' }, source: 'kubelet' },
  { namespace: 'monitoring', type: 'Normal', reason: 'Started', message: 'Started container prometheus-server', count: 2, age: '17m', object: { kind: 'Pod', name: 'prometheus-server-6bcb7d6b8f-lv4dk' }, source: 'kubelet' },
  { namespace: 'cert-manager', type: 'Normal', reason: 'Issuing', message: 'The certificate has been successfully issued', count: 1, age: '12d', object: { kind: 'Certificate', name: 'shop-tls' }, source: 'cert-manager-certificates-issuing' },
]

export const MOCK_CLUSTER_EVENTS: KubeObject[] = SPECS.map((spec, i) => ({
  apiVersion: 'v1',
  kind: 'Event',
  metadata: { name: `${spec.object.name}.${(0x17a0 + i * 7).toString(16)}`, namespace: spec.namespace, labels: {} },
  age: spec.age,
  type: spec.type,
  reason: spec.reason,
  message: spec.message,
  count: spec.count,
  involvedObject: { kind: spec.object.kind, name: spec.object.name, namespace: spec.object.kind === 'Node' ? undefined : spec.namespace },
  source: { component: spec.source },
}))
