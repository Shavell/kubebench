/**
 * Demo data of the Helm plugin: chart repositories with their indexes and
 * releases with a revision history. Everything is fixed (no clock, no
 * randomness), so the demo looks the same on every load.
 */

export interface MockChartVersion {
  version: string
  appVersion: string
  created: string
}

export interface MockChart {
  repo: string
  name: string
  description: string
  keywords: string[]
  home: string
  /** Image the default values point at. */
  image: string
  /** Newest first, like a repository index. */
  versions: MockChartVersion[]
}

export interface MockRepo {
  name: string
  url: string
  authenticated: boolean
  updated: string
}

export interface MockRevision {
  revision: number
  status: string
  updated: string
  description: string
  chart: string
  chartVersion: string
  appVersion: string
  /** User-supplied values (YAML). */
  values: string
}

export interface MockRelease {
  name: string
  namespace: string
  /** Newest first. */
  revisions: MockRevision[]
}

const v = (version: string, appVersion: string, created: string): MockChartVersion => ({ version, appVersion, created })

export const mockCharts: MockChart[] = [
  {
    repo: 'jetstack',
    name: 'cert-manager',
    description: 'A Helm chart for cert-manager',
    keywords: ['cert-manager', 'tls', 'certificates'],
    home: 'https://cert-manager.io',
    image: 'quay.io/jetstack/cert-manager-controller',
    versions: [
      v('v1.15.1', 'v1.15.1', '2026-07-02T10:00:00Z'),
      v('v1.14.5', 'v1.14.5', '2026-05-20T10:00:00Z'),
      v('v1.14.4', 'v1.14.4', '2026-04-11T10:00:00Z'),
    ],
  },
  {
    repo: 'prometheus-community',
    name: 'kube-prometheus-stack',
    description: 'kube-prometheus-stack collects Kubernetes manifests, Grafana dashboards and Prometheus rules',
    keywords: ['prometheus', 'monitoring', 'grafana'],
    home: 'https://github.com/prometheus-operator/kube-prometheus',
    image: 'quay.io/prometheus/prometheus',
    versions: [
      v('59.0.0', 'v0.74.0', '2026-08-01T08:00:00Z'),
      v('58.2.1', 'v0.73.2', '2026-07-10T08:00:00Z'),
      v('58.1.0', 'v0.73.1', '2026-06-21T08:00:00Z'),
    ],
  },
  {
    repo: 'prometheus-community',
    name: 'prometheus-node-exporter',
    description: 'A Helm chart for prometheus node-exporter',
    keywords: ['node-exporter', 'prometheus', 'monitoring'],
    home: 'https://github.com/prometheus/node_exporter',
    image: 'quay.io/prometheus/node-exporter',
    versions: [v('4.36.0', '1.8.1', '2026-07-15T08:00:00Z'), v('4.35.0', '1.8.0', '2026-06-01T08:00:00Z')],
  },
  {
    repo: 'traefik',
    name: 'traefik',
    description: 'A Traefik based Kubernetes ingress controller',
    keywords: ['traefik', 'ingress', 'networking'],
    home: 'https://traefik.io',
    image: 'docker.io/traefik',
    versions: [v('28.3.0', 'v3.0.4', '2026-07-20T12:00:00Z'), v('28.0.0', 'v3.0.1', '2026-05-30T12:00:00Z')],
  },
  {
    repo: 'podinfo',
    name: 'podinfo',
    description: 'Podinfo Helm chart for Kubernetes',
    keywords: ['podinfo', 'demo'],
    home: 'https://github.com/stefanprodan/podinfo',
    image: 'ghcr.io/stefanprodan/podinfo',
    versions: [
      v('6.7.1', '6.7.1', '2026-08-05T09:00:00Z'),
      v('6.7.0', '6.7.0', '2026-07-01T09:00:00Z'),
      v('6.6.3', '6.6.3', '2026-05-12T09:00:00Z'),
    ],
  },
]

export const mockRepos: MockRepo[] = [
  { name: 'jetstack', url: 'https://charts.jetstack.io', authenticated: false, updated: '2026-08-10T07:00:00Z' },
  {
    name: 'prometheus-community',
    url: 'https://prometheus-community.github.io/helm-charts',
    authenticated: false,
    updated: '2026-08-10T07:00:00Z',
  },
  { name: 'traefik', url: 'https://traefik.github.io/charts', authenticated: false, updated: '2026-08-10T07:00:00Z' },
  { name: 'podinfo', url: 'https://stefanprodan.github.io/podinfo', authenticated: false, updated: '2026-08-10T07:00:00Z' },
]

/** Default values.yaml of a chart version. */
export function defaultValues(chart: Pick<MockChart, 'name' | 'image'>, appVersion: string): string {
  return `# Default values for ${chart.name}.

# Number of replicas of the main Deployment.
replicaCount: 1

image:
  repository: ${chart.image}
  # Defaults to the chart's appVersion.
  tag: "${appVersion}"
  pullPolicy: IfNotPresent

service:
  type: ClusterIP
  port: 80

resources: {}
`
}

const rev = (
  revision: number,
  status: string,
  updated: string,
  description: string,
  chart: string,
  chartVersion: string,
  appVersion: string,
  values: string,
): MockRevision => ({ revision, status, updated, description, chart, chartVersion, appVersion, values })

const certManagerValues = `installCRDs: true

replicaCount: 1

prometheus:
  enabled: true
  servicemonitor:
    enabled: true
`

const prometheusValues = (retention: string) => `fullnameOverride: prometheus

alertmanager:
  enabled: true

prometheus:
  prometheusSpec:
    retention: ${retention}
    scrapeInterval: 30s

grafana:
  enabled: true
  ingress:
    enabled: true
    hosts:
      - grafana.example.com
`

const shopValues = (tag: string) => `replicaCount: 3

image:
  repository: ghcr.io/acme/shop
  tag: ${tag}

service:
  port: 8080
`

/** The releases the demo starts with; a fresh copy on every call. */
export function initialReleases(): MockRelease[] {
  return [
    {
      name: 'cert-manager',
      namespace: 'cert-manager',
      revisions: [
        rev(2, 'deployed', '2026-06-30T14:12:00Z', 'Upgrade complete', 'cert-manager', 'v1.14.5', 'v1.14.5', certManagerValues),
        rev(1, 'superseded', '2026-05-26T09:41:00Z', 'Install complete', 'cert-manager', 'v1.14.4', 'v1.14.4', 'installCRDs: true\n'),
      ],
    },
    {
      name: 'kube-prometheus-stack',
      namespace: 'monitoring',
      revisions: [
        rev(5, 'deployed', '2026-08-02T11:05:00Z', 'Upgrade complete', 'kube-prometheus-stack', '58.2.1', 'v0.73.2', prometheusValues('15d')),
        rev(4, 'superseded', '2026-07-18T16:44:00Z', 'Upgrade complete', 'kube-prometheus-stack', '58.2.1', 'v0.73.2', prometheusValues('10d')),
        rev(3, 'superseded', '2026-07-01T10:02:00Z', 'Upgrade complete', 'kube-prometheus-stack', '58.1.0', 'v0.73.1', prometheusValues('10d')),
      ],
    },
    {
      name: 'traefik',
      namespace: 'kube-system',
      revisions: [rev(1, 'deployed', '2026-06-14T08:27:00Z', 'Install complete', 'traefik', '28.0.0', 'v3.0.1', '')],
    },
    {
      name: 'shop-app',
      namespace: 'shop',
      revisions: [
        rev(7, 'failed', '2026-08-10T22:51:00Z', 'Upgrade "shop-app" failed: post-upgrade hooks failed', 'shop-app', '0.7.3', '2.4.1', shopValues('2.4.1')),
        rev(6, 'superseded', '2026-08-08T13:19:00Z', 'Upgrade complete', 'shop-app', '0.7.2', '2.4.0', shopValues('2.4.0')),
        rev(5, 'superseded', '2026-07-29T17:33:00Z', 'Upgrade complete', 'shop-app', '0.7.1', '2.3.9', shopValues('2.3.9')),
      ],
    },
  ]
}
