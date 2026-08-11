import type { ContainerSpec, Deployment, K8sNode, Namespace, Pod, Service } from '../types'

export const nodes: K8sNode[] = [
  {
    name: 'kb-control-plane',
    roles: ['control-plane'],
    status: 'Ready',
    version: 'v1.30.2',
    os: 'Ubuntu 22.04.4 LTS',
    cpuCapacity: 4,
    memoryCapacityGi: 8,
    age: '92d',
    internalIP: '172.18.0.2',
  },
  {
    name: 'kb-worker-1',
    roles: ['worker'],
    status: 'Ready',
    version: 'v1.30.2',
    os: 'Ubuntu 22.04.4 LTS',
    cpuCapacity: 4,
    memoryCapacityGi: 8,
    age: '92d',
    internalIP: '172.18.0.3',
  },
  {
    name: 'kb-worker-2',
    roles: ['worker'],
    status: 'Ready',
    version: 'v1.30.2',
    os: 'Ubuntu 22.04.4 LTS',
    cpuCapacity: 4,
    memoryCapacityGi: 8,
    age: '61d',
    internalIP: '172.18.0.4',
  },
]

export const namespaces: Namespace[] = [
  { name: 'default', status: 'Active', age: '92d' },
  { name: 'kube-system', status: 'Active', age: '92d' },
  { name: 'cert-manager', status: 'Active', age: '77d' },
  { name: 'monitoring', status: 'Active', age: '61d' },
  { name: 'shop', status: 'Active', age: '45d' },
]

interface PodSeed {
  name: string
  namespace: string
  status?: Pod['status']
  ready?: [number, number]
  restarts?: number
  age: string
  node: string
  ip: string
  app: string
  image: string
}

const podSeeds: PodSeed[] = [
  // kube-system
  { name: 'etcd-kb-control-plane', namespace: 'kube-system', age: '92d', node: 'kb-control-plane', ip: '172.18.0.2', app: 'etcd', image: 'registry.k8s.io/etcd:3.5.12-0' },
  { name: 'kube-apiserver-kb-control-plane', namespace: 'kube-system', age: '92d', node: 'kb-control-plane', ip: '172.18.0.2', app: 'kube-apiserver', image: 'registry.k8s.io/kube-apiserver:v1.30.2' },
  { name: 'kube-controller-manager-kb-control-plane', namespace: 'kube-system', age: '92d', node: 'kb-control-plane', ip: '172.18.0.2', app: 'kube-controller-manager', image: 'registry.k8s.io/kube-controller-manager:v1.30.2' },
  { name: 'kube-scheduler-kb-control-plane', namespace: 'kube-system', age: '92d', node: 'kb-control-plane', ip: '172.18.0.2', app: 'kube-scheduler', image: 'registry.k8s.io/kube-scheduler:v1.30.2' },
  { name: 'coredns-7db6d8ff4d-9x2mp', namespace: 'kube-system', age: '92d', node: 'kb-worker-1', ip: '10.244.1.2', app: 'coredns', image: 'registry.k8s.io/coredns/coredns:v1.11.1' },
  { name: 'coredns-7db6d8ff4d-tk8wz', namespace: 'kube-system', age: '92d', node: 'kb-worker-2', ip: '10.244.2.2', app: 'coredns', image: 'registry.k8s.io/coredns/coredns:v1.11.1' },
  { name: 'kube-proxy-b4mch', namespace: 'kube-system', age: '92d', node: 'kb-control-plane', ip: '172.18.0.2', app: 'kube-proxy', image: 'registry.k8s.io/kube-proxy:v1.30.2' },
  { name: 'kube-proxy-hw9kd', namespace: 'kube-system', age: '92d', node: 'kb-worker-1', ip: '172.18.0.3', app: 'kube-proxy', image: 'registry.k8s.io/kube-proxy:v1.30.2' },
  { name: 'kube-proxy-s2n7f', namespace: 'kube-system', age: '61d', node: 'kb-worker-2', ip: '172.18.0.4', app: 'kube-proxy', image: 'registry.k8s.io/kube-proxy:v1.30.2' },
  // cert-manager
  { name: 'cert-manager-5c9d8879fd-x7x4r', namespace: 'cert-manager', age: '77d', node: 'kb-worker-1', ip: '10.244.1.7', app: 'cert-manager', image: 'quay.io/jetstack/cert-manager-controller:v1.14.5' },
  { name: 'cert-manager-cainjector-7d4868cf69-mlq2b', namespace: 'cert-manager', age: '77d', node: 'kb-worker-2', ip: '10.244.2.8', app: 'cainjector', image: 'quay.io/jetstack/cert-manager-cainjector:v1.14.5' },
  { name: 'cert-manager-webhook-7f9f8648b9-fjp5c', namespace: 'cert-manager', age: '77d', node: 'kb-worker-1', ip: '10.244.1.8', app: 'webhook', image: 'quay.io/jetstack/cert-manager-webhook:v1.14.5' },
  // monitoring
  { name: 'prometheus-server-6bcb7d6b8f-lv4dk', namespace: 'monitoring', ready: [2, 2], age: '61d', node: 'kb-worker-2', ip: '10.244.2.11', app: 'prometheus', image: 'quay.io/prometheus/prometheus:v2.51.2' },
  { name: 'prometheus-node-exporter-c8gtq', namespace: 'monitoring', age: '61d', node: 'kb-control-plane', ip: '172.18.0.2', app: 'node-exporter', image: 'quay.io/prometheus/node-exporter:v1.7.0' },
  { name: 'prometheus-node-exporter-kkr2t', namespace: 'monitoring', age: '61d', node: 'kb-worker-1', ip: '172.18.0.3', app: 'node-exporter', image: 'quay.io/prometheus/node-exporter:v1.7.0' },
  { name: 'prometheus-node-exporter-vx4tn', namespace: 'monitoring', age: '61d', node: 'kb-worker-2', ip: '172.18.0.4', app: 'node-exporter', image: 'quay.io/prometheus/node-exporter:v1.7.0' },
  { name: 'metrics-server-557ff575fb-jq2dn', namespace: 'monitoring', age: '61d', node: 'kb-worker-1', ip: '10.244.1.12', app: 'metrics-server', image: 'registry.k8s.io/metrics-server/metrics-server:v0.7.1' },
  // shop
  { name: 'shop-frontend-745bd7c77b-2fkxx', namespace: 'shop', age: '3d', node: 'kb-worker-1', ip: '10.244.1.21', app: 'shop-frontend', image: 'ghcr.io/acme/shop-frontend:2.4.1' },
  { name: 'shop-frontend-745bd7c77b-8pl6m', namespace: 'shop', age: '3d', node: 'kb-worker-2', ip: '10.244.2.21', app: 'shop-frontend', image: 'ghcr.io/acme/shop-frontend:2.4.1' },
  { name: 'shop-frontend-745bd7c77b-w9tqk', namespace: 'shop', status: 'Pending', ready: [0, 1], age: '26m', node: '', ip: '', app: 'shop-frontend', image: 'ghcr.io/acme/shop-frontend:2.4.1' },
  { name: 'shop-cart-6f8d9b5c44-nr7vd', namespace: 'shop', age: '3d', node: 'kb-worker-1', ip: '10.244.1.23', app: 'shop-cart', image: 'ghcr.io/acme/shop-cart:1.9.0' },
  { name: 'shop-cart-6f8d9b5c44-qh5zs', namespace: 'shop', age: '3d', node: 'kb-worker-2', ip: '10.244.2.23', app: 'shop-cart', image: 'ghcr.io/acme/shop-cart:1.9.0' },
  { name: 'shop-payments-79c6cd8b96-t2m8j', namespace: 'shop', status: 'CrashLoopBackOff', ready: [0, 1], restarts: 14, age: '41m', node: 'kb-worker-2', ip: '10.244.2.25', app: 'shop-payments', image: 'ghcr.io/acme/shop-payments:3.1.0-rc.2' },
  { name: 'shop-postgres-0', namespace: 'shop', age: '45d', node: 'kb-worker-1', ip: '10.244.1.25', app: 'postgres', image: 'postgres:16.2-alpine' },
]

/** Per-app container details; anything not listed gets sensible defaults. */
const APP_SPECS: Record<string, Partial<ContainerSpec>> = {
  'shop-frontend': {
    command: ['node', 'server.js'],
    env: [
      { name: 'PORT', value: '8080' },
      { name: 'API_BASE_URL', value: 'http://shop-cart:8080' },
      { name: 'NODE_ENV', value: 'production' },
      { name: 'FEATURE_NEW_CHECKOUT', value: 'true' },
    ],
    requests: { cpu: '100m', memory: '128Mi' },
    limits: { cpu: '500m', memory: '256Mi' },
    ports: '8080/TCP',
  },
  'shop-cart': {
    command: ['/app/cart'],
    args: ['--redis=redis://shop-redis:6379'],
    env: [
      { name: 'PORT', value: '8080' },
      { name: 'SESSION_TTL', value: '3600' },
    ],
    requests: { cpu: '50m', memory: '96Mi' },
    limits: { cpu: '250m', memory: '192Mi' },
    ports: '8080/TCP',
  },
  'shop-payments': {
    command: ['/app/payments'],
    args: ['--config=/etc/payments/config.yaml'],
    env: [
      { name: 'PAYMENT_GATEWAY_URL', value: 'https://gw.pay.example.com' },
      { name: 'TIMEOUT_MS', value: '3000' },
      { name: 'LOG_LEVEL', value: 'debug' },
    ],
    requests: { cpu: '100m', memory: '128Mi' },
    limits: { cpu: '500m', memory: '256Mi' },
    ports: '9102/TCP',
  },
  postgres: {
    command: ['docker-entrypoint.sh'],
    args: ['postgres'],
    env: [
      { name: 'POSTGRES_DB', value: 'shop' },
      { name: 'POSTGRES_USER', value: 'shop' },
      { name: 'PGDATA', value: '/var/lib/postgresql/data/pgdata' },
    ],
    requests: { cpu: '250m', memory: '512Mi' },
    limits: { cpu: '1', memory: '1Gi' },
    ports: '5432/TCP',
  },
  prometheus: {
    args: ['--config.file=/etc/prometheus/prometheus.yml', '--storage.tsdb.retention.time=15d'],
    requests: { cpu: '200m', memory: '1Gi' },
    limits: { cpu: '1', memory: '2Gi' },
    ports: '9090/TCP',
  },
  coredns: {
    args: ['-conf', '/etc/coredns/Corefile'],
    requests: { cpu: '100m', memory: '70Mi' },
    limits: { cpu: '100m', memory: '170Mi' },
    ports: '53/UDP',
  },
}

const CONTROLLERS: Record<string, string> = {
  'shop-frontend': 'Deployment/shop-frontend',
  'shop-cart': 'Deployment/shop-cart',
  'shop-payments': 'Deployment/shop-payments',
  postgres: 'StatefulSet/shop-postgres',
  coredns: 'Deployment/coredns',
  'cert-manager': 'Deployment/cert-manager',
  cainjector: 'Deployment/cert-manager-cainjector',
  webhook: 'Deployment/cert-manager-webhook',
  prometheus: 'Deployment/prometheus-server',
  'node-exporter': 'DaemonSet/prometheus-node-exporter',
  'metrics-server': 'Deployment/metrics-server',
  'kube-proxy': 'DaemonSet/kube-proxy',
}

function containerFor(app: string, image: string): ContainerSpec {
  const spec = APP_SPECS[app] ?? {}
  return {
    name: app,
    image,
    command: spec.command,
    args: spec.args,
    env: spec.env ?? [
      { name: 'POD_NAMESPACE', value: 'metadata.namespace (fieldRef)' },
      { name: 'POD_NAME', value: 'metadata.name (fieldRef)' },
    ],
    requests: spec.requests ?? { cpu: '50m', memory: '64Mi' },
    limits: spec.limits ?? { cpu: '200m', memory: '128Mi' },
    ports: spec.ports,
  }
}

function buildPod(seed: PodSeed): Pod {
  const [ready, total] = seed.ready ?? [1, 1]
  return {
    name: seed.name,
    namespace: seed.namespace,
    status: seed.status ?? 'Running',
    readyContainers: seed.status && !seed.ready ? 0 : ready,
    totalContainers: total,
    restarts: seed.restarts ?? 0,
    age: seed.age,
    node: seed.node,
    podIP: seed.ip,
    controlledBy: CONTROLLERS[seed.app],
    labels: { app: seed.app },
    containers: [containerFor(seed.app, seed.image)],
  }
}

export const pods: Pod[] = podSeeds.map(buildPod)

let scaleCounter = 0

/** Visual-only mutation: add or remove pods to match a deployment's scale. */
export function syncDeploymentPods(deployment: Deployment, replicas: number): void {
  const app = deployment.labels.app ?? deployment.labels['k8s-app'] ?? deployment.name
  const owned = pods.filter((p) => p.namespace === deployment.namespace && p.labels.app === app)
  const template = owned[0]
  for (let i = owned.length; i < replicas; i++) {
    const suffix = ((scaleCounter++ * 2654435761 + 0x9e3779b9) >>> 8).toString(36).slice(0, 5)
    pods.push({
      ...(template ?? buildPod({
        name: deployment.name,
        namespace: deployment.namespace,
        age: '5s',
        node: 'kb-worker-1',
        ip: '10.244.1.99',
        app,
        image: `registry.example.com/${app}:latest`,
      })),
      name: `${deployment.name}-${suffix}`,
      status: 'Running',
      readyContainers: 1,
      restarts: 0,
      age: '5s',
      node: i % 2 === 0 ? 'kb-worker-1' : 'kb-worker-2',
    })
  }
  for (let i = owned.length; i > replicas; i--) {
    const victim = owned[i - 1]
    const idx = pods.findIndex((p) => p === victim)
    if (idx >= 0) pods.splice(idx, 1)
  }
}

export const deployments: Deployment[] = [
  { name: 'coredns', namespace: 'kube-system', readyReplicas: 2, replicas: 2, upToDate: 2, available: 2, age: '92d', labels: { 'k8s-app': 'kube-dns' } },
  { name: 'cert-manager', namespace: 'cert-manager', readyReplicas: 1, replicas: 1, upToDate: 1, available: 1, age: '77d', labels: { app: 'cert-manager' } },
  { name: 'prometheus-server', namespace: 'monitoring', readyReplicas: 1, replicas: 1, upToDate: 1, available: 1, age: '61d', labels: { app: 'prometheus' } },
  { name: 'metrics-server', namespace: 'monitoring', readyReplicas: 1, replicas: 1, upToDate: 1, available: 1, age: '61d', labels: { app: 'metrics-server' } },
  { name: 'shop-frontend', namespace: 'shop', readyReplicas: 2, replicas: 3, upToDate: 3, available: 2, age: '45d', labels: { app: 'shop-frontend' } },
  { name: 'shop-payments', namespace: 'shop', readyReplicas: 0, replicas: 1, upToDate: 1, available: 0, age: '45d', labels: { app: 'shop-payments' } },
]

export const services: Service[] = [
  { name: 'kubernetes', namespace: 'default', type: 'ClusterIP', clusterIP: '10.96.0.1', ports: '443/TCP', age: '92d' },
  { name: 'kube-dns', namespace: 'kube-system', type: 'ClusterIP', clusterIP: '10.96.0.10', ports: '53/UDP,53/TCP,9153/TCP', age: '92d' },
  { name: 'cert-manager-webhook', namespace: 'cert-manager', type: 'ClusterIP', clusterIP: '10.96.112.44', ports: '443/TCP', age: '77d' },
  { name: 'prometheus-server', namespace: 'monitoring', type: 'ClusterIP', clusterIP: '10.96.201.7', ports: '9090/TCP', age: '61d' },
  { name: 'shop-frontend', namespace: 'shop', type: 'LoadBalancer', clusterIP: '10.96.45.19', externalIP: '203.0.113.42', ports: '80:31204/TCP', age: '45d' },
  { name: 'shop-cart', namespace: 'shop', type: 'ClusterIP', clusterIP: '10.96.45.23', ports: '8080/TCP', age: '45d' },
  { name: 'shop-postgres', namespace: 'shop', type: 'ClusterIP', clusterIP: '10.96.45.31', ports: '5432/TCP', age: '45d' },
]
