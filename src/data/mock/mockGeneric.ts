import type { KubeObject } from '../types'

/**
 * Mock objects for the generic resource browser, keyed by catalog key.
 * Deterministic — objects carry a synthetic `age` (no creationTimestamp,
 * so nothing depends on the clock).
 */

function obj(
  kind: string,
  name: string,
  namespace: string | undefined,
  age: string,
  rest: Record<string, unknown> = {},
): KubeObject {
  return {
    kind,
    metadata: { name, ...(namespace ? { namespace } : {}), labels: {} },
    age,
    ...rest,
  }
}

export const genericMock: Record<string, KubeObject[]> = {
  configmaps: [
    obj('ConfigMap', 'kube-root-ca.crt', 'default', '92d', { data: { 'ca.crt': '<1099 bytes>' } }),
    obj('ConfigMap', 'coredns', 'kube-system', '92d', { data: { Corefile: '<412 bytes>' } }),
    obj('ConfigMap', 'shop-frontend-config', 'shop', '45d', {
      data: { 'API_BASE_URL': 'http://shop-cart:8080', 'FEATURE_NEW_CHECKOUT': 'true' },
    }),
    obj('ConfigMap', 'prometheus-server', 'monitoring', '61d', { data: { 'prometheus.yml': '<3.2 KiB>' } }),
  ],
  secrets: [
    obj('Secret', 'shop-tls', 'shop', '45d', { type: 'kubernetes.io/tls', data: { 'tls.crt': '<1.2 KiB, redacted>', 'tls.key': '<1.7 KiB, redacted>' } }),
    obj('Secret', 'ghcr-creds', 'shop', '45d', { type: 'kubernetes.io/dockerconfigjson', data: { '.dockerconfigjson': '<220 bytes, redacted>' } }),
    obj('Secret', 'sh.helm.release.v1.cert-manager.v2', 'cert-manager', '77d', { type: 'helm.sh/release.v1', data: { release: '<12 KiB, redacted>' } }),
    obj('Secret', 'grafana-admin', 'monitoring', '61d', { type: 'Opaque', data: { 'admin-user': '<5 bytes, redacted>', 'admin-password': '<16 bytes, redacted>' } }),
  ],
  statefulsets: [
    obj('StatefulSet', 'shop-postgres', 'shop', '45d', { status: { readyReplicas: 1 }, spec: { replicas: 1 } }),
    obj('StatefulSet', 'prometheus-server', 'monitoring', '61d', { status: { readyReplicas: 1 }, spec: { replicas: 1 } }),
  ],
  daemonsets: [
    obj('DaemonSet', 'prometheus-node-exporter', 'monitoring', '61d', { status: { numberReady: 3, desiredNumberScheduled: 3 } }),
    obj('DaemonSet', 'kube-proxy', 'kube-system', '92d', { status: { numberReady: 3, desiredNumberScheduled: 3 } }),
  ],
  replicasets: [
    obj('ReplicaSet', 'shop-frontend-745bd7c77b', 'shop', '3d', { status: { readyReplicas: 2, replicas: 3 } }),
    obj('ReplicaSet', 'cert-manager-5c9d8879fd', 'cert-manager', '77d', { status: { readyReplicas: 1, replicas: 1 } }),
  ],
  jobs: [
    obj('Job', 'db-migrate-28934', 'shop', '2h', { status: { succeeded: 1 }, spec: { completions: 1 } }),
  ],
  cronjobs: [
    obj('CronJob', 'shop-nightly-report', 'shop', '45d', { spec: { schedule: '0 2 * * *', suspend: false } }),
  ],
  ingresses: [
    obj('Ingress', 'shop-frontend', 'shop', '45d', {
      spec: { ingressClassName: 'traefik', rules: [{ host: 'shop.example.com' }] },
    }),
    obj('Ingress', 'grafana', 'monitoring', '58d', {
      spec: { ingressClassName: 'traefik', rules: [{ host: 'grafana.example.com' }] },
    }),
  ],
  endpoints: [
    obj('Endpoints', 'shop-frontend', 'shop', '45d', { subsets: [{ addresses: [{ ip: '10.244.1.21' }, { ip: '10.244.2.21' }], ports: [{ port: 8080 }] }] }),
  ],
  networkpolicies: [
    obj('NetworkPolicy', 'shop-default-deny', 'shop', '45d', { spec: { policyTypes: ['Ingress'] } }),
  ],
  persistentvolumeclaims: [
    obj('PersistentVolumeClaim', 'shop-postgres-data', 'shop', '45d', {
      status: { phase: 'Bound', capacity: { storage: '10Gi' } },
      spec: { storageClassName: 'standard', accessModes: ['ReadWriteOnce'], volumeName: 'pvc-8f3a2c' },
    }),
    obj('PersistentVolumeClaim', 'prometheus-data', 'monitoring', '61d', {
      status: { phase: 'Bound', capacity: { storage: '20Gi' } },
      spec: { storageClassName: 'standard', accessModes: ['ReadWriteOnce'], volumeName: 'pvc-2b1e99' },
    }),
  ],
  persistentvolumes: [
    obj('PersistentVolume', 'pvc-8f3a2c', undefined, '45d', {
      status: { phase: 'Bound' },
      spec: { capacity: { storage: '10Gi' }, storageClassName: 'standard', accessModes: ['ReadWriteOnce'], claimRef: { namespace: 'shop', name: 'shop-postgres-data' } },
    }),
  ],
  storageclasses: [
    obj('StorageClass', 'standard', undefined, '92d', { provisioner: 'rancher.io/local-path', reclaimPolicy: 'Delete', volumeBindingMode: 'WaitForFirstConsumer' }),
  ],
  serviceaccounts: [
    obj('ServiceAccount', 'default', 'default', '92d'),
    obj('ServiceAccount', 'cert-manager', 'cert-manager', '77d'),
    obj('ServiceAccount', 'prometheus-server', 'monitoring', '61d'),
  ],
  roles: [
    obj('Role', 'cert-manager-cainjector', 'cert-manager', '77d'),
  ],
  rolebindings: [
    obj('RoleBinding', 'cert-manager-cainjector', 'cert-manager', '77d', { roleRef: { kind: 'Role', name: 'cert-manager-cainjector' } }),
  ],
  clusterroles: [
    obj('ClusterRole', 'cluster-admin', undefined, '92d'),
    obj('ClusterRole', 'cert-manager-controller', undefined, '77d'),
    obj('ClusterRole', 'view', undefined, '92d'),
  ],
  clusterrolebindings: [
    obj('ClusterRoleBinding', 'cluster-admin', undefined, '92d', { roleRef: { kind: 'ClusterRole', name: 'cluster-admin' } }),
  ],
  resourcequotas: [
    obj('ResourceQuota', 'shop-quota', 'shop', '45d', { spec: { scopes: [] }, status: { hard: { 'requests.cpu': '4', 'requests.memory': '8Gi' } } }),
  ],
  limitranges: [obj('LimitRange', 'shop-limits', 'shop', '45d')],
  horizontalpodautoscalers: [
    obj('HorizontalPodAutoscaler', 'shop-frontend', 'shop', '45d', {
      spec: { scaleTargetRef: { kind: 'Deployment', name: 'shop-frontend' }, minReplicas: 2, maxReplicas: 6 },
      status: { currentReplicas: 3 },
    }),
  ],
  poddisruptionbudgets: [
    obj('PodDisruptionBudget', 'shop-frontend', 'shop', '45d', { spec: { minAvailable: 2 }, status: { currentHealthy: 2 } }),
  ],
  priorityclasses: [
    obj('PriorityClass', 'system-cluster-critical', undefined, '92d', { value: 2000000000, globalDefault: false }),
    obj('PriorityClass', 'high-priority', undefined, '45d', { value: 1000000, globalDefault: false }),
  ],
  runtimeclasses: [obj('RuntimeClass', 'nvidia', undefined, '30d', { handler: 'nvidia' })],
  leases: [
    obj('Lease', 'kube-controller-manager', 'kube-system', '92d', { spec: { holderIdentity: 'kb-control-plane_a1b2' } }),
  ],
  mutatingwebhookconfigurations: [
    obj('MutatingWebhookConfiguration', 'cert-manager-webhook', undefined, '77d', { webhooks: [{}] }),
  ],
  validatingwebhookconfigurations: [
    obj('ValidatingWebhookConfiguration', 'cert-manager-webhook', undefined, '77d', { webhooks: [{}, {}] }),
  ],
  replicationcontrollers: [],
  ingressclasses: [
    obj('IngressClass', 'traefik', undefined, '92d', { spec: { controller: 'traefik.io/ingress-controller' } }),
  ],
  endpointslices: [
    obj('EndpointSlice', 'shop-frontend-abcde', 'shop', '45d', { addressType: 'IPv4', endpoints: [{}, {}] }),
  ],
  events: [
    obj('Event', 'shop-payments.17a', 'shop', '2m', { type: 'Warning', reason: 'BackOff', message: 'Back-off restarting failed container', count: 14 }),
    obj('Event', 'shop-frontend.18b', 'shop', '26m', { type: 'Warning', reason: 'FailedScheduling', message: '0/3 nodes are available: insufficient cpu', count: 12 }),
  ],
}
