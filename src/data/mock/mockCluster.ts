import type { Deployment, K8sNode, Namespace, Pod, Service } from '../types'

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

export const pods: Pod[] = podSeeds.map((seed) => {
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
    labels: { app: seed.app },
    containers: [{ name: seed.app, image: seed.image }],
  }
})

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
