import type { K8sEvent } from '../types'

/**
 * Object-specific events, keyed by "Kind/namespace/name" (namespace "-"
 * for cluster-scoped). Objects without an entry fall back to a generic
 * healthy lifecycle.
 */
const SPECIAL_EVENTS: Record<string, K8sEvent[]> = {
  'Pod/shop/shop-payments-79c6cd8b96-t2m8j': [
    { type: 'Warning', reason: 'BackOff', message: 'Back-off restarting failed container shop-payments in pod shop-payments-79c6cd8b96-t2m8j', age: '2m', count: 14, source: 'kubelet' },
    { type: 'Warning', reason: 'Unhealthy', message: 'Startup probe failed: Get "http://10.244.2.25:9102/healthz": context deadline exceeded', age: '3m', count: 14, source: 'kubelet' },
    { type: 'Normal', reason: 'Pulled', message: 'Container image "ghcr.io/acme/shop-payments:3.1.0-rc.2" already present on machine', age: '3m', count: 15, source: 'kubelet' },
    { type: 'Normal', reason: 'Created', message: 'Created container shop-payments', age: '3m', count: 15, source: 'kubelet' },
    { type: 'Normal', reason: 'Started', message: 'Started container shop-payments', age: '3m', count: 15, source: 'kubelet' },
    { type: 'Normal', reason: 'Scheduled', message: 'Successfully assigned shop/shop-payments-79c6cd8b96-t2m8j to kb-worker-2', age: '41m', count: 1, source: 'default-scheduler' },
  ],
  'Pod/shop/shop-frontend-745bd7c77b-w9tqk': [
    { type: 'Warning', reason: 'FailedScheduling', message: '0/3 nodes are available: 3 Insufficient cpu. preemption: 0/3 nodes are available: 3 No preemption victims found for incoming pod.', age: '55s', count: 12, source: 'default-scheduler' },
  ],
  'Deployment/shop/shop-payments': [
    { type: 'Normal', reason: 'ScalingReplicaSet', message: 'Scaled up replica set shop-payments-79c6cd8b96 to 1', age: '45d', count: 1, source: 'deployment-controller' },
  ],
  'Deployment/shop/shop-frontend': [
    { type: 'Normal', reason: 'ScalingReplicaSet', message: 'Scaled up replica set shop-frontend-745bd7c77b to 3 from 2', age: '26m', count: 1, source: 'deployment-controller' },
    { type: 'Normal', reason: 'ScalingReplicaSet', message: 'Scaled up replica set shop-frontend-745bd7c77b to 2', age: '3d', count: 1, source: 'deployment-controller' },
  ],
  'Node/-/kb-worker-2': [
    { type: 'Warning', reason: 'SystemOOM', message: 'System OOM encountered, victim process: shop-payments (pid 21433)', age: '38m', count: 2, source: 'kubelet' },
    { type: 'Normal', reason: 'NodeReady', message: 'Node kb-worker-2 status is now: NodeReady', age: '61d', count: 1, source: 'kubelet' },
    { type: 'Normal', reason: 'RegisteredNode', message: 'Node kb-worker-2 event: Registered Node kb-worker-2 in Controller', age: '61d', count: 1, source: 'node-controller' },
  ],
}

export function eventsFor(kind: string, namespace: string | null, name: string): K8sEvent[] {
  const special = SPECIAL_EVENTS[`${kind}/${namespace ?? '-'}/${name}`]
  if (special) return special

  if (kind === 'Pod') {
    return [
      { type: 'Normal', reason: 'Started', message: `Started container ${name.split('-').slice(0, 2).join('-')}`, age: '3d', count: 1, source: 'kubelet' },
      { type: 'Normal', reason: 'Created', message: 'Created container', age: '3d', count: 1, source: 'kubelet' },
      { type: 'Normal', reason: 'Pulled', message: 'Successfully pulled image', age: '3d', count: 1, source: 'kubelet' },
      { type: 'Normal', reason: 'Scheduled', message: `Successfully assigned ${namespace}/${name}`, age: '3d', count: 1, source: 'default-scheduler' },
    ]
  }
  if (kind === 'Node') {
    return [
      { type: 'Normal', reason: 'NodeReady', message: `Node ${name} status is now: NodeReady`, age: '92d', count: 1, source: 'kubelet' },
      { type: 'Normal', reason: 'NodeAllocatableEnforced', message: 'Updated Node Allocatable limit across pods', age: '92d', count: 1, source: 'kubelet' },
      { type: 'Normal', reason: 'RegisteredNode', message: `Node ${name} event: Registered Node ${name} in Controller`, age: '92d', count: 1, source: 'node-controller' },
    ]
  }
  if (kind === 'Deployment') {
    return [
      { type: 'Normal', reason: 'ScalingReplicaSet', message: `Scaled up replica set ${name}`, age: '45d', count: 1, source: 'deployment-controller' },
    ]
  }
  return []
}
