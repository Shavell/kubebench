import type { CrdDefinition, CustomResource } from '../types.ts'

export const crds: CrdDefinition[] = [
  {
    name: 'certificates.cert-manager.io',
    group: 'cert-manager.io',
    kind: 'Certificate',
    versions: ['v1'],
    scope: 'Namespaced',
    shortNames: ['cert', 'certs'],
    age: '77d',
    printerColumns: [{ name: 'Secret', type: 'string', priority: 0 }, { name: 'Issuer', type: 'string', priority: 1 }],
  },
  {
    name: 'issuers.cert-manager.io',
    group: 'cert-manager.io',
    kind: 'Issuer',
    versions: ['v1'],
    scope: 'Namespaced',
    shortNames: ['iss'],
    age: '77d',
    printerColumns: [{ name: 'Type', type: 'string', priority: 0 }],
  },
  {
    name: 'servicemonitors.monitoring.coreos.com',
    group: 'monitoring.coreos.com',
    kind: 'ServiceMonitor',
    versions: ['v1'],
    scope: 'Namespaced',
    shortNames: ['smon'],
    age: '61d',
    printerColumns: [{ name: 'Port', type: 'string', priority: 0 }, { name: 'Interval', type: 'string', priority: 1 }],
  },
  {
    name: 'ingressroutes.traefik.io',
    group: 'traefik.io',
    kind: 'IngressRoute',
    versions: ['v1alpha1'],
    scope: 'Namespaced',
    shortNames: [],
    age: '58d',
    printerColumns: [{ name: 'EntryPoints', type: 'string', priority: 0 }, { name: 'Host', type: 'string', priority: 0 }],
  },
  {
    name: 'rollouts.argoproj.io',
    group: 'argoproj.io',
    kind: 'Rollout',
    versions: ['v1alpha1'],
    scope: 'Namespaced',
    shortNames: ['ro'],
    age: '40d',
    printerColumns: [
      { name: 'Desired', type: 'integer', description: 'Number of desired pods', priority: 0 },
      { name: 'Current', type: 'integer', description: 'Total number of non-terminated pods', priority: 0 },
    ],
    scale: { specReplicasPath: '.spec.replicas', statusReplicasPath: '.status.replicas', labelSelectorPath: '.status.selector' },
  },
]

export const customResources: CustomResource[] = [
  { name: 'shop-tls', namespace: 'shop', crdName: 'certificates.cert-manager.io', status: 'Ready', age: '45d', extra: { Secret: 'shop-tls', Issuer: 'letsencrypt-prod' } },
  { name: 'grafana-tls', namespace: 'monitoring', crdName: 'certificates.cert-manager.io', status: 'Ready', age: '61d', extra: { Secret: 'grafana-tls', Issuer: 'letsencrypt-prod' } },
  { name: 'staging-tls', namespace: 'default', crdName: 'certificates.cert-manager.io', status: 'Pending', age: '2h', extra: { Secret: 'staging-tls', Issuer: 'letsencrypt-staging' } },
  { name: 'letsencrypt-prod', namespace: 'cert-manager', crdName: 'issuers.cert-manager.io', status: 'Ready', age: '77d', extra: { Type: 'ACME' } },
  { name: 'letsencrypt-staging', namespace: 'cert-manager', crdName: 'issuers.cert-manager.io', status: 'Ready', age: '77d', extra: { Type: 'ACME' } },
  { name: 'selfsigned', namespace: 'cert-manager', crdName: 'issuers.cert-manager.io', status: 'Ready', age: '77d', extra: { Type: 'SelfSigned' } },
  { name: 'prometheus-node-exporter', namespace: 'monitoring', crdName: 'servicemonitors.monitoring.coreos.com', status: 'Active', age: '61d', extra: { Port: 'metrics', Interval: '30s' } },
  { name: 'shop-services', namespace: 'monitoring', crdName: 'servicemonitors.monitoring.coreos.com', status: 'Active', age: '44d', extra: { Port: 'http-metrics', Interval: '15s' } },
  { name: 'shop-frontend-route', namespace: 'shop', crdName: 'ingressroutes.traefik.io', status: 'Active', age: '45d', extra: { EntryPoints: 'websecure', Host: 'shop.example.com' } },
  { name: 'grafana-route', namespace: 'monitoring', crdName: 'ingressroutes.traefik.io', status: 'Active', age: '58d', extra: { EntryPoints: 'websecure', Host: 'grafana.example.com' } },
  { name: 'shop-checkout', namespace: 'shop', crdName: 'rollouts.argoproj.io', status: 'Healthy', age: '40d', extra: { Desired: '3', Current: '3' }, replicas: 3 },
  { name: 'shop-search', namespace: 'shop', crdName: 'rollouts.argoproj.io', status: 'Paused', age: '12d', extra: { Desired: '2', Current: '2' }, replicas: 2 },
]

/** How scaleMockCustomResource ended; the mock provider turns failures into the backend's errors. */
export type MockCrScaleResult = 'scaled' | 'not_served' | 'no_scale' | 'not_found'

/**
 * The mock of POST /api/workloads/scale with apiVersion for custom
 * resources: the CRD serving `kind` at `apiVersion` must have a scale
 * subresource; the object's desired replicas (and its Desired/Current
 * printer columns, which the mock keeps in step at once) become `replicas`.
 */
export function scaleMockCustomResource(apiVersion: string, kind: string, namespace: string, name: string, replicas: number): MockCrScaleResult {
  const crd = crds.find((c) => c.kind === kind && c.versions.some((v) => `${c.group}/${v}` === apiVersion))
  if (!crd) return 'not_served'
  if (!crd.scale) return 'no_scale'
  const cr = customResources.find(
    (r) => r.crdName === crd.name && r.name === name && (crd.scope === 'Cluster' || r.namespace === namespace),
  )
  if (!cr) return 'not_found'
  cr.replicas = replicas
  cr.extra = { ...cr.extra, Desired: String(replicas), Current: String(replicas) }
  return 'scaled'
}
