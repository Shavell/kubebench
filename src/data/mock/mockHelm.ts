import type { HelmRelease, HelmRepo } from '../types'

export const releases: HelmRelease[] = [
  {
    name: 'cert-manager',
    namespace: 'cert-manager',
    chart: 'cert-manager',
    chartVersion: 'v1.14.5',
    appVersion: 'v1.14.5',
    revision: 2,
    status: 'deployed',
    updated: '2026-06-30 14:12',
    history: [
      { revision: 2, status: 'deployed', updated: '2026-06-30 14:12', description: 'Upgrade complete' },
      { revision: 1, status: 'superseded', updated: '2026-05-26 09:41', description: 'Install complete' },
    ],
  },
  {
    name: 'kube-prometheus-stack',
    namespace: 'monitoring',
    chart: 'kube-prometheus-stack',
    chartVersion: '58.2.1',
    appVersion: 'v0.73.2',
    revision: 5,
    status: 'deployed',
    updated: '2026-08-02 11:05',
    history: [
      { revision: 5, status: 'deployed', updated: '2026-08-02 11:05', description: 'Upgrade complete' },
      { revision: 4, status: 'superseded', updated: '2026-07-18 16:44', description: 'Upgrade complete' },
      { revision: 3, status: 'superseded', updated: '2026-07-01 10:02', description: 'Upgrade complete' },
    ],
  },
  {
    name: 'traefik',
    namespace: 'kube-system',
    chart: 'traefik',
    chartVersion: '28.0.0',
    appVersion: 'v3.0.1',
    revision: 1,
    status: 'deployed',
    updated: '2026-06-14 08:27',
    history: [
      { revision: 1, status: 'deployed', updated: '2026-06-14 08:27', description: 'Install complete' },
    ],
  },
  {
    name: 'shop-app',
    namespace: 'shop',
    chart: 'shop-app',
    chartVersion: '0.7.3',
    appVersion: '2.4.1',
    revision: 7,
    status: 'failed',
    updated: '2026-08-10 22:51',
    history: [
      { revision: 7, status: 'failed', updated: '2026-08-10 22:51', description: 'Upgrade "shop-app" failed: post-upgrade hooks failed' },
      { revision: 6, status: 'superseded', updated: '2026-08-08 13:19', description: 'Upgrade complete' },
      { revision: 5, status: 'superseded', updated: '2026-07-29 17:33', description: 'Upgrade complete' },
    ],
  },
]

export const repositories: HelmRepo[] = [
  { name: 'jetstack', url: 'https://charts.jetstack.io', chartCount: 12 },
  { name: 'prometheus-community', url: 'https://prometheus-community.github.io/helm-charts', chartCount: 64 },
  { name: 'traefik', url: 'https://traefik.github.io/charts', chartCount: 9 },
]

export const valuesYaml: Record<string, string> = {
  'cert-manager': `installCRDs: true

replicaCount: 1

image:
  repository: quay.io/jetstack/cert-manager-controller
  tag: v1.14.5

resources:
  requests:
    cpu: 10m
    memory: 32Mi

prometheus:
  enabled: true
  servicemonitor:
    enabled: true

webhook:
  replicaCount: 1

cainjector:
  enabled: true
`,
  'kube-prometheus-stack': `fullnameOverride: prometheus

defaultRules:
  create: true
  rules:
    etcd: true
    kubeScheduler: true

alertmanager:
  enabled: true
  config:
    route:
      receiver: slack-ops
      group_wait: 30s

prometheus:
  prometheusSpec:
    retention: 15d
    scrapeInterval: 30s
    resources:
      requests:
        cpu: 200m
        memory: 1Gi
    storageSpec:
      volumeClaimTemplate:
        spec:
          resources:
            requests:
              storage: 20Gi

grafana:
  enabled: true
  adminPassword: "***"
  ingress:
    enabled: true
    hosts:
      - grafana.example.com

nodeExporter:
  enabled: true
`,
  traefik: `deployment:
  replicas: 1

ingressClass:
  enabled: true
  isDefaultClass: true

ports:
  web:
    port: 8000
    exposedPort: 80
  websecure:
    port: 8443
    exposedPort: 443
    tls:
      enabled: true

providers:
  kubernetesCRD:
    enabled: true
  kubernetesIngress:
    enabled: true
`,
  'shop-app': `global:
  imageRegistry: ghcr.io/acme
  imagePullSecrets:
    - name: ghcr-creds

frontend:
  replicas: 3
  image:
    tag: 2.4.1
  ingressRoute:
    host: shop.example.com
    tls: shop-tls

cart:
  replicas: 2
  image:
    tag: 1.9.0

payments:
  replicas: 1
  image:
    tag: 3.1.0-rc.2   # rollback candidate: rc build failing hooks
  env:
    PAYMENT_GATEWAY_URL: https://gw.pay.example.com
    TIMEOUT_MS: "3000"

postgres:
  persistence:
    size: 10Gi
    storageClass: standard
`,
}
