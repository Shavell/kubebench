# KubeBench

A VSCode-style dashboard & workbench for Kubernetes: activity bar, tabbed
editor area, bottom panel and command palette — with pluggable modules for
workloads, CRDs, Helm, metrics and cluster inventory.

The UI is written entirely against typed provider interfaces. It runs **live**
against a real cluster through a small local proxy, and falls back to a
deterministic **mock** cluster when the proxy is down. The status bar always
shows which mode you are in.

## Quick start

```bash
npm install

# Mock mode (no cluster needed)
npm run dev                                      # http://localhost:5173

# Live mode: start the proxy with a token-based kubeconfig, then reload
KUBECONFIG=/path/to/kubeconfig npm run server    # http://localhost:9876
npm run dev
```

The proxy seeds the **cluster inventory** (`~/.kubebench/clusters.json`) from
`KUBECONFIG` on first run; afterwards manage clusters from the UI (Clusters
module, top of the activity bar).

## Modules

| Module | What it does |
|---|---|
| **Clusters** | Inventory of kubeconfig paths / API addresses; add, remove, activate (switches the proxy and reloads the app) |
| **Workloads** | Nodes, namespaces, pods, deployments, services: lists with in-page filters (name / namespace / status) and sortable columns; rich detail pages with utilization charts, resources (requests/limits), env, labels/annotations, events; actions: logs viewer, exec shell (mock), delete pod, scale & restart deployment, YAML view |
| **CRDs** | CRD discovery grouped by API group, custom resource lists with CRD-specific columns *(mock until RBAC allows)* |
| **Helm** | Releases with revision history, per-revision rollback, uninstall, editable values, repositories *(mock until RBAC allows)* |
| **Metrics** | Cluster Overview: usage tiles (CPU/Mem/Disk), **allocation panels** (requests/limits/usage vs allocatable, with overcommit marker), utilization charts (CPU, memory, network, filesystem); per-node metric pages; status bar CPU/Mem item |
| **Settings** | Gear icon (activity bar bottom): theme, **metrics source**, metrics window, auto refresh, terminal font size — persisted in localStorage |

Shell features: command palette (`Cmd/Ctrl+Shift+P` or `F1`), tab context menu
(Close / Close Others / Close to the Right / Close All), resizable sidebar and
bottom panel (Problems / Logs / Terminal), namespace filter in the status bar.

## Metrics sources

Utilization series come from one of two backends, selectable in Settings:

- **Prometheus** (preferred, auto-discovered): queried through the Kubernetes
  API server's `services/proxy`. Supports full history and network +
  filesystem metrics (node-exporter). Discovery matches common service names
  (`prometheus`, `prometheus-server`, `kube-prometheus-stack-prometheus`, …).
- **metrics-server**: instant usage only; the proxy samples it every 15s into
  a ring buffer, so history accumulates while the proxy runs. No network/fs
  data (those charts fall back to mock series).

## Live-mode capability matrix

What is real depends on the kubeconfig's RBAC. With a read-mostly service
account (the current demo credentials):

| Capability | Needs | Status with demo credentials |
|---|---|---|
| Resource lists / detail / YAML / events | `list`, `get` | ✅ live |
| Pod logs | `pods/log` | ✅ live |
| Usage & allocation metrics | metrics-server, or `services/proxy` + Prometheus | ✅ live |
| Delete pod, scale/restart deployment | `delete pods`, `patch deployments` | ⚠️ endpoint wired; RBAC errors shown as inline banners |
| Helm releases | `secrets` | ❌ mock |
| CRD browsing | `customresourcedefinitions` | ❌ mock |
| Exec into container | `pods/exec` | ❌ mock shell |

## Documentation

- [`docs/architecture.md`](docs/architecture.md) — layers, plugin API, data
  providers, how to add a module
- [`docs/backend.md`](docs/backend.md) — proxy endpoints, inventory format,
  metrics sources, RBAC requirements

## Scripts

| Command | |
|---|---|
| `npm run dev` | Vite dev server (proxies `/api` to `:9876`) |
| `npm run server` | Cluster proxy (reads `KUBECONFIG`, port `9876`) |
| `npm run build` | Type-check + production build |
| `npm run lint` | Oxlint |
