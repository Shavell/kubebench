# Design: Go backend

Replaces the Node proxy with a single Go binary that is deployable *into* the
cluster it manages. Drivers for the rewrite:

- **Native SDK** — client-go / helm SDK / metrics clientset instead of
  shelling out to `kubectl`/`helm`; typed interfaces, in-cluster config,
  proper connection reuse.
- **Deployability** — one container: Go binary with the built frontend
  embedded (`go:embed`), Helm chart with SA/RBAC, state on a PVC.
- **Multi-user** — local username/password auth, sessions, per-user cluster
  assignments; state in SQLite.

## Big picture

```
┌─────────────────────────── container ───────────────────────────┐
│  kubebench (Go binary, :9876)                                   │
│                                                                  │
│  web        embedded React frontend (go:embed dist/)             │
│  api        HTTP handlers + session middleware                   │
│  store      SQLite: users, sessions, clusters, assignments       │
│  kube       per-cluster runtime: clientset, dynamic, metrics,    │
│             helm action config, sampler, prometheus, capabilities │
└──────────────┬───────────────────────────────┬──────────────────┘
               │ in-cluster SA (chart)         │ stored kubeconfigs (DB)
        managed cluster                 other clusters (inventory)
```

Key shift from the Node version: **the active cluster is per-user**, not
global. `kube.Manager` keeps a lazy cache of `*ClusterRuntime` keyed by
cluster id; every API request resolves the session user → active cluster →
runtime.

## Packages

```
backend/
  main.go                    flags/env, bootstrap, http server
  internal/store             SQLite (modernc.org/sqlite, no cgo) + models
  internal/kube              ClusterRuntime: clients, mappers, sampler,
                             prometheus, helm, exec, crds, capabilities
  internal/api               router, auth middleware, handlers (JSON contract
                             kept compatible with the Node proxy)
  internal/web               go:embed of the built frontend
```

## Storage (SQLite)

File from `KUBEBENCH_DB` (default `./kubebench.db`; chart mounts a PVC at
`/data`). Pure-Go driver so the image stays `CGO_ENABLED=0`.

```sql
users(id, username UNIQUE, password_hash, role,            -- 'admin' | 'user'
      active_cluster_id NULL, created_at)
sessions(token PK, user_id, expires_at)
clusters(id, name UNIQUE, source,                          -- 'kubeconfig' | 'incluster' | 'address'
         kubeconfig TEXT NULL,                             -- full YAML content
         context TEXT NULL, address TEXT NULL, created_at)
user_clusters(user_id, cluster_id, PK(user_id, cluster_id))
```

- Kubeconfigs are stored as **content** (the server may run in-cluster where
  host paths don't exist). Adding by path is still supported from the UI —
  the server reads the file once and stores the content.
- Bootstrap: if no users exist, create `admin` with
  `KUBEBENCH_ADMIN_PASSWORD` (default `admin`). If `rest.InClusterConfig()`
  succeeds, an `in-cluster` entry is auto-registered; `KUBECONFIG` seeds an
  entry in local dev.

## AuthN/AuthZ

- `POST /api/auth/login` → bcrypt check → random 256-bit session token in an
  `HttpOnly` cookie (7d). `POST /api/auth/logout`, `GET /api/auth/me`.
- Middleware: everything under `/api` except `login` requires a session;
  static assets are public.
- Roles: **admin** manages users and the cluster inventory, sees all
  clusters; **user** sees only assigned clusters. Kubernetes-side authz is
  whatever the cluster credential grants (the chart's SA for `in-cluster`,
  the stored kubeconfig otherwise).

## Cluster runtime (internal/kube)

Per cluster id, built lazily and cached:

| Concern | Implementation |
|---|---|
| clients | `kubernetes.Clientset`, `dynamic.Interface`, `metricsv.Clientset` from `clientcmd` (content) or `rest.InClusterConfig()` |
| resources | typed list/get + the same JSON mappers as before |
| YAML | fetch object, strip `managedFields`, `sigs.k8s.io/yaml` |
| logs | `Pods.GetLogs` (tail 300, timestamps) |
| exec | `remotecommand.NewSPDYExecutor`, one-shot `sh -c`, stdout+stderr |
| metrics | 15s sampler ring buffer (metrics.k8s.io) + Prometheus discovery through `Services.ProxyGet`; same `source=auto|prometheus|metrics-server` semantics |
| helm | moved to the backend plugin `internal/plugins/helm` in v0.5 (design-v0.5 §7.4) |
| CRDs | apiextensions list + dynamic CR listing with `additionalPrinterColumns` |
| capabilities | probes on first build: crds list, helm secrets list, SSAR `pods/exec` |

## API surface

Unchanged (so the React data layer keeps working): `/api/info`, resources,
yaml/events/logs, mutations, metrics (incl. allocation/sources), crds, helm,
exec, clusters. Changed/new:

- `/api/info` → adds `{user, role}`; capabilities are for the user's active
  cluster.
- `/api/clusters` → entries the user can see; `activate` sets the **user's**
  active cluster; `POST` accepts `{name, content | path | address, context}`.
- `/api/auth/*`, `/api/users*` (admin): CRUD + `PUT /api/users/{id}/clusters`.

## Frontend deltas

- Bootstrap: `GET /api/auth/me`; 401 → full-screen VSCode-style login; mock
  mode (proxy down) skips auth entirely.
- Status bar: username + logout on the right.
- New **Users** module (admin only — registered after `me` resolves): list,
  create, delete users; assign clusters with checkboxes.
- Inventory: add-cluster form gains a kubeconfig **paste** field (path stays
  for local dev); entries show who can access them (admin).
- `UsersProvider` joins the provider contract with a mock implementation, so
  the module renders in mock mode too.

## Build & deploy

- **Dockerfile** (multi-stage): `node:22` builds the frontend → `golang`
  builds the binary embedding `dist/` → `gcr.io/distroless/static`. Single
  ~30 MB image, runs as non-root, `PORT`/`KUBEBENCH_DB` env.
- **Helm chart** (`charts/kubebench`): Deployment (1 replica —
  SQLite + RWO PVC), ServiceAccount, ClusterRole/Binding (default:
  cluster-admin-ish rule set, overridable via `rbac.rules`), PVC with
  `persistence.storageClass` (default **`local-path`** for k3s), Service,
  optional Ingress, Secret for the bootstrap admin password.
- The Go backend replaced the original Node proxy, which has been removed;
  `npm run server:go` runs the Go binary in dev.

## Trade-offs / later

- SQLite + RWO PVC pins the Deployment to one replica — right-sized for a
  workbench; Postgres would be the path to HA.
- Kubeconfigs are stored unencrypted in SQLite (volume-level protection);
  at-rest encryption (age/AES with a chart-provided key) is a follow-up.
- Exec stays one-shot request/response; a websocket PTY (client-go
  `remotecommand` stream ↔ xterm.js) is the natural upgrade.
- Sessions are DB-backed → survive restarts; no refresh tokens needed at
  this scale.
