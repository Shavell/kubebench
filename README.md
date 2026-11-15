# KubeBench

A VSCode-style dashboard & workbench for Kubernetes: activity bar, tabbed
editor area, bottom panel and command palette — with pluggable modules for
workloads, CRDs, Helm, metrics, cluster inventory and user management.

Frontend: React 19 + TS + Vite. Backend: a single Go binary (client-go, Helm
SDK, SQLite) that embeds the built frontend and is **deployable into the
cluster it manages**. The UI is written against typed provider interfaces and
falls back to a deterministic **mock** cluster when no backend is reachable.

## Quick start

```bash
npm install

# Mock mode (no backend, no auth)
npm run dev                                       # http://localhost:5173

# Live mode: Go backend against a kubeconfig, then the dev frontend
KUBECONFIG=/path/to/kubeconfig npm run server:go  # http://localhost:9876
npm run dev                                        # sign in as admin/admin
```

## Deploy into a cluster

```bash
docker build -t ghcr.io/your-org/kubebench:0.1.0 .
helm install kubebench charts/kubebench -n kubebench --create-namespace \
  --set image.repository=ghcr.io/your-org/kubebench \
  --set auth.adminPassword=changeme
```

One container, one PVC (SQLite on **`local-path`** by default for k3s), a
ServiceAccount bound to `cluster-admin`, a Service and optional Ingress. The
in-cluster ServiceAccount is auto-registered as the managed cluster. Full
guide: [`docs/deployment.md`](docs/deployment.md).

## Auth & multi-user

- Local username/password login; DB-backed HttpOnly session cookies.
- **admin** manages the cluster inventory and users; **user** sees only the
  clusters assigned to them (Users module). Each user has an active cluster;
  add remote clusters by pasting a kubeconfig (Clusters module).

## Modules

| Module | What it does |
|---|---|
| **Clusters** | Inventory of kubeconfig paths / API addresses; add, remove, activate (switches your active cluster and reloads the app) |
| **Workloads** | Grouped resource tree — **Cluster** (Nodes, Namespaces, Events), **Workloads** (Pods, Deployments, StatefulSets, DaemonSets, ReplicaSets, ReplicationControllers, Jobs, CronJobs), **Config** (ConfigMaps, Secrets, ResourceQuotas, LimitRanges, HPAs, PodDisruptionBudgets, PriorityClasses, RuntimeClasses, Leases, Mutating/Validating Webhooks), **Network** (Services, Ingresses, IngressClasses, EndpointSlices, Endpoints, NetworkPolicies), **Storage** (PVCs, PVs, StorageClasses), **Access Control** (ServiceAccounts, Roles/Bindings, ClusterRoles/Bindings). Rich detail (utilization charts, env, requests/limits, events, logs, exec, delete/scale/restart) for pods/deployments/nodes; generic list+detail+YAML for the rest. Secret values are redacted. |
| **CRDs** | CRD discovery grouped by API group, custom resource lists with CRD-specific columns |
| **Helm** | Releases with revision history, per-revision rollback, uninstall, editable values, repositories |
| **Metrics** | Cluster Overview: usage tiles (CPU/Mem/Disk), **allocation panels** (requests/limits/usage vs allocatable, with overcommit marker), utilization charts (CPU, memory, network, filesystem); per-node metric pages; status bar CPU/Mem item |
| **Settings** | Gear icon (activity bar bottom): theme, **metrics source**, metrics window, auto refresh, terminal font size — persisted in localStorage |
| **Users** (admin) | Create/delete users, assign clusters per user |

Shell features: command palette (`Cmd/Ctrl+Shift+P` or `F1`), tab context menu
(Close / Close Others / Close to the Right / Close All), resizable sidebar and
bottom panel (Problems / Logs / Terminal), namespace filter in the status bar.

## Metrics sources

Utilization series come from one of two backends, selectable in Settings:

- **Prometheus** (preferred, auto-discovered): queried through the Kubernetes
  API server's `services/proxy`. Supports full history and network +
  filesystem metrics (node-exporter). Discovery matches common service names
  (`prometheus`, `prometheus-server`, `kube-prometheus-stack-prometheus`, …).
- **metrics-server**: instant usage only; the backend samples it every 15s
  into a ring buffer, so history accumulates while the backend runs. Network
  and filesystem charts need Prometheus and say so when it is missing.

## Live-mode capability matrix

On activation the backend probes what the cluster credential allows and
reports capabilities to the UI (`/api/info`). Live mode never shows mock
data: a module whose capability is missing explains why instead. Auth
supports bearer tokens **and** client certificates.

| Capability | Needs | When unavailable |
|---|---|---|
| Resource lists / detail / YAML / events | `list`, `get` | error shown |
| Pod logs | `pods/log` | error shown |
| Usage & allocation metrics | metrics-server, or `services/proxy` + Prometheus | unavailable, with the reason |
| Delete pod, scale/restart deployment | `delete pods`, `patch deployments` | RBAC error banner |
| Helm (releases, history, values, install, upgrade, rollback, uninstall) | `secrets` access (Helm SDK in the backend) | unavailable, with the reason |
| CRD browsing (defs + instances + YAML) | `customresourcedefinitions` + CR groups | unavailable, with the reason |
| Exec (terminal, one-shot commands) | `pods/exec` | unavailable, with the reason |

## Documentation

- [`docs/design-go-backend.md`](docs/design-go-backend.md) — Go backend
  architecture: packages, DB schema, auth, cluster runtime
- [`docs/deployment.md`](docs/deployment.md) — Docker image, Helm chart, RBAC,
  configuration
- [`docs/architecture.md`](docs/architecture.md) — frontend layers, plugin
  API, data providers, how to add a module
- [`docs/backend.md`](docs/backend.md) — where the HTTP API is specified
  ([`docs/design-v0.5.md`](docs/design-v0.5.md) §3–§7)

## Scripts

| Command | |
|---|---|
| `npm run dev` | Vite dev server (proxies `/api` to `:9876`) |
| `npm run server:go` | Go backend (reads `KUBECONFIG`, port `9876`) |
| `npm run build` | Type-check + production build |
| `npm run lint` | Oxlint |
| `docker build -t kubebench .` | Build the deployable image |
| `helm install … charts/kubebench` | Deploy into a cluster |
