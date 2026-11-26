import type { CrdDefinition, CustomResource } from '../types'

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
]
