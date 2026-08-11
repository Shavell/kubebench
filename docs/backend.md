# Backend proxy (`server/`)

A small Node (ESM) HTTP server that gives the browser a same-origin REST API
over the Kubernetes API. Dependencies: `undici` (HTTPS with the cluster CA),
`js-yaml` (kubeconfig parsing + YAML rendering).

```bash
KUBECONFIG=/path/to/kubeconfig npm run server   # listens on :9876
PORT=9999 npm run server                        # alternate port
```

Vite proxies `/api` → `http://localhost:9876` in dev.

## Authentication

`server/kube.mjs` reads the kubeconfig's current context and expects a
**bearer token** user (service-account kubeconfigs). Exec/cloud-provider auth
plugins and client certificates are not supported. The CA from
`certificate-authority-data` is used to verify the API server.

## Files

| File | Responsibility |
|---|---|
| `index.mjs` | routes, active-cluster state, allocation computation |
| `kube.mjs` | kubeconfig parsing + authenticated HTTP client |
| `mappers.mjs` | raw K8s objects → UI domain shapes; quantity/age parsing |
| `metricsSampler.mjs` | 15s metrics-server polling into a ring buffer (240 samples ≈ 1h) |
| `prometheus.mjs` | Prometheus discovery + PromQL range queries via `services/proxy` |
| `inventory.mjs` | `~/.kubebench/clusters.json` persistence |

## Endpoints

### Info & inventory

| Route | Notes |
|---|---|
| `GET /api/info` | active context, server URL, default namespace |
| `GET /api/clusters` | inventory entries + per-entry context/server (parsed from each kubeconfig) |
| `POST /api/clusters?name=&path=&address=` | add; validates the kubeconfig before saving; `path` or `address` required |
| `DELETE /api/clusters?name=` | remove (deactivates if active) |
| `POST /api/clusters/activate?name=` | rebuild client + sampler + Prometheus discovery for that entry |

The inventory stores **paths only, never credentials**. Address-only entries
are bookmarks and cannot be activated. On first run the inventory is seeded
from `KUBECONFIG`.

### Resources

| Route | Notes |
|---|---|
| `GET /api/nodes` · `/api/namespaces` | cluster-scoped lists |
| `GET /api/pods` · `/api/deployments` · `/api/services` (`?ns=`) | namespaced lists; `ns` omitted = all namespaces |
| `GET /api/yaml?kind=&ns=&name=` | object as YAML, `managedFields` stripped |
| `GET /api/events?ns=&name=` | events for one object (`involvedObject.name` fieldSelector) |
| `GET /api/logs?ns=&pod=&container=` | last 300 lines, timestamps normalized to `HH:MM:SS.mmm` |

### Mutations

| Route | RBAC needed |
|---|---|
| `DELETE /api/pods?ns=&name=` | `delete pods` |
| `POST /api/deployments/scale?ns=&name=&replicas=` | `patch deployments/scale` |
| `POST /api/deployments/restart?ns=&name=` | `patch deployments` (sets `kubectl.kubernetes.io/restartedAt`) |

Kubernetes error messages pass through verbatim with their status codes; the
UI shows them as inline banners.

### Metrics

| Route | Notes |
|---|---|
| `GET /api/metrics/sources` | availability of metrics-server and Prometheus (+ discovered location) |
| `GET /api/metrics/overview` | node/pod/namespace counts + latest usage % |
| `GET /api/metrics/allocation` | Σ pod requests/limits vs Σ node allocatable (cpu cores, memory Gi) + current usage |
| `GET /api/metrics/cluster?metric=&minutes=&source=` | series |
| `GET /api/metrics/node?node=&metric=&minutes=&source=` | series |
| `GET /api/metrics/pod?ns=&pod=&metric=&minutes=&source=` | series |

`metric` = `cpu` | `memory` | `network` | `fs` · `source` = `auto` (default)
| `prometheus` | `metrics-server`.

- **auto** prefers Prometheus when discovered, else the sampler.
- **metrics-server** serves cpu/memory from the ring buffer (history exists
  only while the proxy runs) and returns **501** for network/fs — the client
  then falls back to mock series.
- **prometheus** returns 503 if not discovered.

### Prometheus discovery

On activation the proxy lists services cluster-wide and probes well-known
names on ports 80/9090 through
`/api/v1/namespaces/<ns>/services/<name>:<port>/proxy`:

`prometheus`, `prometheus-server`, `prometheus-operated`, `prometheus-k8s`,
`prometheus-kube-prometheus-prometheus`, `kube-prometheus-stack-prometheus`.

Node series match node-exporter samples via the `kubernetes_node` label; pod
series use cAdvisor `container_*` metrics. Requires `services/proxy` RBAC.

## RBAC requirements

Minimum for a fully-live read experience:

```yaml
rules:
  - apiGroups: [""]
    resources: [nodes, namespaces, pods, services, events, pods/log]
    verbs: [get, list]
  - apiGroups: [apps]
    resources: [deployments]
    verbs: [get, list]
  - apiGroups: [metrics.k8s.io]
    resources: [nodes, pods]
    verbs: [get, list]
  - apiGroups: [""]
    resources: [services/proxy]      # Prometheus access
    verbs: [get]
```

Additional, per feature: `delete pods` (kill), `patch deployments` +
`deployments/scale` (scale/restart), `pods/exec` (real exec), `secrets`
(Helm releases), `customresourcedefinitions` + CR groups (CRD browsing),
`nodes/proxy` (kubelet stats alternative to Prometheus).
