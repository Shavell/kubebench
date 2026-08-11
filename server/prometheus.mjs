/**
 * Prometheus metrics source, reached through the Kubernetes API server's
 * service proxy (needs services/proxy RBAC). Discovers a Prometheus
 * service by well-known names, then serves utilization series from
 * PromQL range queries — including network and filesystem, which
 * metrics-server cannot provide.
 */

const CANDIDATE_NAMES = [
  'prometheus',
  'prometheus-server',
  'prometheus-operated',
  'prometheus-k8s',
  'prometheus-kube-prometheus-prometheus',
  'kube-prometheus-stack-prometheus',
]

export async function discoverPrometheus(kube) {
  let services
  try {
    services = (await kube.getJSON('/api/v1/services')).items
  } catch {
    return null
  }
  const candidates = services.filter((s) => CANDIDATE_NAMES.includes(s.metadata.name))
  for (const svc of candidates) {
    const port = (svc.spec.ports ?? []).find((p) => [80, 9090].includes(p.port))?.port
    if (!port) continue
    const base = `/api/v1/namespaces/${svc.metadata.namespace}/services/${svc.metadata.name}:${port}/proxy`
    try {
      const res = JSON.parse(await kube.getRaw(`${base}/api/v1/query?query=up`))
      if (res.status === 'success') {
        console.log(`[prometheus] using ${svc.metadata.namespace}/${svc.metadata.name}:${port}`)
        return createPromSource(kube, base, `${svc.metadata.namespace}/${svc.metadata.name}`)
      }
    } catch {
      /* try next candidate */
    }
  }
  return null
}

function createPromSource(kube, base, location) {
  async function rangeQuery(query, minutes) {
    const end = Math.floor(Date.now() / 1000)
    const start = end - minutes * 60
    const step = Math.max(15, Math.floor((minutes * 60) / 120))
    const url = `${base}/api/v1/query_range?query=${encodeURIComponent(query)}&start=${start}&end=${end}&step=${step}`
    const res = JSON.parse(await kube.getRaw(url))
    if (res.status !== 'success') throw new Error(`prometheus query failed: ${query}`)
    const series = res.data.result[0]?.values ?? []
    return series.map(([ts, value]) => {
      const d = new Date(ts * 1000)
      return {
        time: `${String(d.getHours()).padStart(2, '0')}:${String(d.getMinutes()).padStart(2, '0')}`,
        value: Math.round(parseFloat(value) * 10) / 10,
      }
    })
  }

  // node-exporter series carry the node name in "kubernetes_node" (Lens
  // stack) or "node"/"instance"; match broadly via regex on any of them.
  const nodeSel = (node) => `kubernetes_node="${node}"`

  const QUERIES = {
    cluster: {
      cpu: '100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle"}[5m])))',
      memory: '100 * (1 - sum(node_memory_MemAvailable_bytes) / sum(node_memory_MemTotal_bytes))',
      network:
        'sum(rate(node_network_receive_bytes_total{device!~"lo|veth.*"}[5m]) + rate(node_network_transmit_bytes_total{device!~"lo|veth.*"}[5m])) / 1e6',
      fs: '100 * (1 - sum(node_filesystem_avail_bytes{mountpoint="/"}) / sum(node_filesystem_size_bytes{mountpoint="/"}))',
    },
    node: {
      cpu: (n) => `100 * (1 - avg(rate(node_cpu_seconds_total{mode="idle",${nodeSel(n)}}[5m])))`,
      memory: (n) =>
        `100 * (1 - sum(node_memory_MemAvailable_bytes{${nodeSel(n)}}) / sum(node_memory_MemTotal_bytes{${nodeSel(n)}}))`,
      network: (n) =>
        `sum(rate(node_network_receive_bytes_total{device!~"lo|veth.*",${nodeSel(n)}}[5m]) + rate(node_network_transmit_bytes_total{device!~"lo|veth.*",${nodeSel(n)}}[5m])) / 1e6`,
      fs: (n) =>
        `100 * (1 - sum(node_filesystem_avail_bytes{mountpoint="/",${nodeSel(n)}}) / sum(node_filesystem_size_bytes{mountpoint="/",${nodeSel(n)}}))`,
    },
    pod: {
      cpu: (ns, pod) =>
        `100 * sum(rate(container_cpu_usage_seconds_total{namespace="${ns}",pod="${pod}"}[5m])) / scalar(count(count by (cpu) (node_cpu_seconds_total)) / count(count by (kubernetes_node) (node_cpu_seconds_total)))`,
      memory: (ns, pod) =>
        `100 * sum(container_memory_working_set_bytes{namespace="${ns}",pod="${pod}"}) / scalar(avg(node_memory_MemTotal_bytes))`,
      network: (ns, pod) =>
        `sum(rate(container_network_receive_bytes_total{namespace="${ns}",pod="${pod}"}[5m]) + rate(container_network_transmit_bytes_total{namespace="${ns}",pod="${pod}"}[5m])) / 1e6`,
      fs: () => null,
    },
  }

  return {
    location,
    clusterSeries: (metric, minutes) => rangeQuery(QUERIES.cluster[metric], minutes),
    nodeSeries: (node, metric, minutes) => rangeQuery(QUERIES.node[metric](node), minutes),
    podSeries: (ns, pod, metric, minutes) => {
      const q = QUERIES.pod[metric](ns, pod)
      if (!q) {
        const err = new Error(`metric "${metric}" not available for pods`)
        err.status = 501
        throw err
      }
      return rangeQuery(q, minutes)
    },
  }
}
