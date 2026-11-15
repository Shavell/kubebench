# Design: KubeBench v0.5 — plugins, i18n, roles, live ops, packaging

Status: **accepted** (implementation contract for the v0.5.0 release).
Every change in v0.5 implements against this document. When code and this
document disagree, fix one of them in the same change.

Goals, in priority order:

1. **Trust** — never show fabricated data in live mode; per-cluster roles,
   read-only clusters and an audit log for a multi-user web tool.
2. **Plugin platform** — backend + frontend plugin contracts, enable/disable,
   capability detection, runtime-loaded external (frontend) plugins.
3. **Multi-language UI** — i18next, English + Turkish, per-plugin namespaces.
4. **Daily operations parity** (inspired by the KubelizeMe comparison, see
   `docs/notes/kubelizeme-comparison.md`): real PTY shell, streaming logs,
   YAML apply/dry-run/diff, node/workload operations, Helm install/upgrade.
5. **New plugins**: Argo CD, Storage (PV/PVC creator), Security posture,
   Access (RBAC), Topology.
6. **Distribution**: Kubernetes (Helm chart + plain manifests), VM (.deb,
   tarball, `curl | bash` installer), online and offline (air-gapped)
   bundles, GitHub Releases, manual-only GitHub Actions, Dependabot.

Out of scope for v0.5 (roadmap): AI assistant, manifest "Composer",
cluster-to-cluster compare, port-forward, light/dark per-plugin theming.

---

## 0. Ground rules (all changes)

- Read `CLAUDE.md`. Layering rules still hold: `core/shell` never imports
  `plugins/*`; plugins never import each other or `core/shell`; cross-plugin
  flows go through tab kinds (`useLayoutStore.getState().openTab(...)`).
- **Every user-visible string goes through i18n** (`useTranslation(ns)`),
  with both `en.json` and `tr.json` entries. `npm run i18n:check` must pass.
- **Mock + live**: every data capability has a deterministic mock
  implementation (seeded RNG, no `Date.now()`/`Math.random()` in mock data
  generation) and a live implementation. **Live mode never falls back to
  mock data**; it shows an "unavailable" state with the reason instead.
- Colors only via `--vscode-*` tokens (`src/core/theme/theme.css`), which
  must be defined for both dark and light themes. Charts are single-series
  small multiples without legends.
- TS: `verbatimModuleSyntax`, `noUnusedLocals`, `erasableSyntaxOnly` (no
  enums, no parameter properties, no namespaces).
- Verify: `npx tsc -b && npm run build && npm run lint && npm run i18n:check`;
  backend `cd backend && go vet ./... && go test ./... && go build ./...`.
- **The developer's k3s homelab cluster (`~/.kube/config`, context
  `k3s-homelab`) may be used for read-only checks only.** Never send a
  mutating request to it (no create/update/patch/delete/evict/exec-that-
  mutates, not even `dryRun`). Test mutations with client-go fake clients
  (`k8s.io/client-go/kubernetes/fake`, `k8s.io/client-go/dynamic/fake`)
  or the mock frontend.
- Never push, never touch the `origin` (Gitea) remote, never build or push
  Docker images, never deploy to a cluster.
- Commits: Conventional Commits (`feat(argocd): …`, `fix(api): …`), body
  explains why, trailer `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.

---

## 1. Versioning & build info

- Go: `package main` declares `var (version = "dev"; commit = "none"; date = "unknown")`,
  set with `-ldflags "-X main.version=… -X main.commit=… -X main.date=…"`.
- `kubebench --version` prints `kubebench <version> (<commit>, <date>)`.
- `kubebench healthcheck` performs `GET /healthz` on the listen address
  (`KUBEBENCH_LISTEN_ADDR`, else `:$PORT`; `127.0.0.1` when it names no
  host or an unspecified one; honours `KUBEBENCH_TLS_*` → https with
  `InsecureSkipVerify`) and exits 0/1 (2 for an invalid configuration) —
  used by Docker `HEALTHCHECK` (distroless has no shell/curl) and by
  `install.sh` to wait for the service.
- `GET /api/version` (public) → `{"version","commit","date"}`.
- `GET /healthz` (public) → `{"status":"ok"}`.
- Frontend: `__APP_VERSION__` is defined by Vite from `package.json`
  (`define` in vite.config.ts); live mode prefers `/api/info.version`.
- Release version for this work: **0.5.0** (package.json, Chart.yaml
  `version`/`appVersion`).

## 2. Configuration (environment)

| Variable | Default | Meaning |
|---|---|---|
| `PORT` | `9876` | HTTP port |
| `KUBEBENCH_LISTEN_ADDR` | `:$PORT` | Full bind address (overrides PORT), e.g. `127.0.0.1:9876` |
| `KUBEBENCH_DB` | `kubebench.db` | SQLite file |
| `KUBEBENCH_DATA_DIR` | dir of `KUBEBENCH_DB` | Writable state: helm repo cache, uploads, plugins |
| `KUBEBENCH_PLUGINS_DIR` | `$KUBEBENCH_DATA_DIR/plugins` | External (frontend) plugins |
| `KUBEBENCH_ADMIN_PASSWORD` | `admin` | Bootstrap admin password (first run only). The admin must change it at first login, the default and a configured one alike |
| `KUBEBENCH_SECURE_COOKIES` | `auto` | `auto` (Secure when request is TLS or trusted `X-Forwarded-Proto: https`), `true`, `false` |
| `KUBEBENCH_TRUST_PROXY` | `false` | Honour `X-Forwarded-Proto/For` (behind ingress/reverse proxy) |
| `KUBEBENCH_TLS_CERT` / `KUBEBENCH_TLS_KEY` | — | Serve HTTPS natively (VM installs without a proxy) |
| `KUBEBENCH_SESSION_TTL` | `168h` | Session lifetime (Go duration) |
| `KUBEBENCH_AUDIT_RETENTION_DAYS` | `90` | Audit rows older than this are pruned daily (0 = keep forever) |
| `KUBEBENCH_ALLOW_PATH_KUBECONFIG` | `true` outside a cluster, `false` in-cluster | Allow adding clusters by server-side file path |
| `KUBEBENCH_ALLOW_KUBECONFIG_EXEC` | `false` | Let pasted kubeconfigs run exec credential plugins and auth providers and read server-side files (certificate, key and token files). This gives every admin command execution as the server's account |
| `KUBECONFIG` | — | Dev/VM convenience: seeds the inventory on first run (a path list is merged the way kubectl merges it) |
| `KUBEBENCH_NAMESPACE` | in a pod: the pod's namespace | The namespace KubeBench runs in; deleting a namespace of that name is refused unless forced (§7.3) |

The backend parses these once into `internal/config.Config`; invalid values
are all reported at once and the binary exits with status 2. Ports, in `PORT`
and `KUBEBENCH_LISTEN_ADDR`, are numbers from 1 to 65535.
`KUBEBENCH_ADMIN_PASSWORD` is only read for a new database and is checked
then: a value other than the default that breaks the password policy
(§3.5) stops the first start with status 2 before the admin is created.

"First run" means a new database (no users yet). Only then are the
bootstrap admin created and the inventory seeded from `KUBECONFIG` and, in
a pod, the in-cluster service account, so deleting a seeded cluster sticks
across restarts. Seeded kubeconfigs are stored flattened (referenced
certificate files embedded).

## 3. Roles, permissions, audit

### 3.1 Model

- Global user role (`users.role`): `admin` | `user` (unchanged).
- Per-cluster assignment role (`user_clusters.role`): `viewer` | `operator`.
  Existing assignments migrate to `operator` (preserves v0.4 behaviour).
- Cluster safety flags (`clusters` table): `read_only` (bool) and
  `environment` (`""` | `development` | `staging` | `production`).

Effective permission of a user on a cluster:

| | admin | operator | viewer |
|---|---|---|---|
| read (lists, YAML, events, logs, metrics) | ✓ | ✓ | ✓ |
| write (mutations) | ✓ | ✓ | ✗ |
| exec (shell, debug container, reveal secret) | ✓ | ✓ | ✗ |
| manage users/clusters/plugins/audit | ✓ | ✗ | ✗ |

A **read-only cluster** rejects write and exec for everyone (admins
included) with `403 {"code":"cluster_read_only"}`; an admin clears the flag
in the Clusters view. Kubernetes-side authorization is still whatever the
stored credential allows.

### 3.2 Access levels in the API

Every route declares one of:

- `public` — no session (`/healthz`, `/api/version`, `POST /api/auth/login`).
- `authed` — any session (`/api/auth/*`, `/api/info`, `/api/clusters` GET,
  activate, `GET /api/plugins`, external plugin files `/plugins/{id}/…`).
- `read` — session + active cluster runtime + any role on it.
- `write` — `read` + role operator/admin + cluster not read-only. **Audited.**
- `exec` — same gate as write. **Audited.**
- `admin` — global admin. Mutations audited.

Every route is registered once (`backend/internal/api/routes.go`) with its
level. The router runs the gates in this order before the handler, so a
refused request never reaches the cluster: `password_change_required` →
`admin_required` → `plugin_disabled` (plugin routes) → active cluster
(`no_cluster`) → `cluster_read_only` (checked first, admins included) →
`forbidden_role` → `plugin_unavailable` (plugin routes).

Dry-run requests (exactly `dryRun=true`) need `read` and are not audited —
on routes that opt in (core flag `allowDryRun`, plugin flag
`plugin.DryRunnable`) and whose handlers then really perform a dry run
(`plugin.IsDryRun(r)`). The relaxation is per route because a handler that
ignored `dryRun` would otherwise let a viewer mutate.
One exception: a dry run that touches a core Secret (apply, edit) needs
`write` and is audited like the change itself (detail `"dryRun":true`),
because its result (`unchanged` or not) would let a viewer, who sees
Secret values only redacted, confirm guessed values. Helm install and
upgrade are not dry-runnable at all (§7.4): Helm's server-side dry run
runs template `lookup`s with KubeBench's own credential.

### 3.3 Error format

All errors are JSON: `{"error": "<human message>", "code": "<machine code>"}`.
Codes used across the API: `unauthenticated`, `session_expired`,
`forbidden`, `forbidden_role`, `cluster_read_only`, `admin_required`,
`no_cluster`, `not_found`, `bad_request`, `conflict`, `rate_limited`,
`plugin_disabled`, `plugin_unavailable`, `capability_unavailable`,
`redacted_secret`, `password_change_required`, `kubernetes` (Kubernetes API error; HTTP status mirrors
the API status), `internal` (a KubeBench-side 500, e.g. the database).
The frontend `ApiError` exposes `status` and `code`.

- Unknown `/api` paths answer `404 not_found`; a known path with the wrong
  method answers `405 bad_request` with an `Allow` header.
- `no_cluster` is `409` when no usable cluster is active (none chosen, it
  was removed, or the assignment was revoked) and `503` when the active
  cluster's runtime cannot be built.
- A Kubernetes `401` (the stored credential was rejected) is reported as
  `502 kubernetes`, so the browser does not mistake it for the end of its
  own KubeBench session.
- Missing capabilities of the legacy endpoints (Prometheus-only metrics)
  answer `503 capability_unavailable`. So does a metric series
  whose Prometheus query fails, with the reason: only `source=auto` falls
  back to metrics-server, and only for CPU and memory, the metrics it has.
  The metrics endpoints wait (≤3 s) for a fresh runtime's capability probe
  and Prometheus discovery, like `/api/info` (§4.1). Helm is a plugin
  (§7.4): without access to the release Secrets it answers
  `503 plugin_unavailable`, and its Detect runs after the probe too.

### 3.4 Audit log

Table `audit_log(id INTEGER PK, ts TIMESTAMP, username TEXT, cluster TEXT,
action TEXT, target TEXT, status INTEGER, detail TEXT)`.
Written automatically by the router for `write`/`exec`/admin mutations:
`action` = `"<METHOD> <route pattern>"` (e.g. `POST /api/deployments/scale`)
or an explicit action name for plugins/websockets (`exec.session`),
`target` = `<kind>/<ns>/<name>` style string from query/path values,
`status` = HTTP status, `detail` = short JSON (e.g. `{"replicas":3}`) or
error text. Pruned per `KUBEBENCH_AUDIT_RETENTION_DAYS`.

- Refused attempts are audited too: status 4xx, detail `"<code>: <message>"`.
- The default target kind is the first path segment after `/api/` (or
  `/api/plugins/<id>/`), made singular (`/api/deployments/scale` →
  `deployment`); `auditKinds` in `routes.go` overrides it per route. On the
  core routes under `/api/resource` and `/api/workloads/`, which act on the
  kind their `kind` parameter names, that parameter sets the target kind;
  every other route ignores it. Handlers may refine target and detail once
  they have read the body.
- Rows are bounded: action, target and detail keep at most 200, 512 and
  1000 characters (a handler whose record is longer, such as a bulk delete,
  writes it across several rows). An audited request whose URL parameters do not fit (a
  target over 512 or a default detail over 1000 characters) is refused
  with `400 bad_request` before its handler runs, so a row always shows a
  command or any other parameter completely.
- Request bodies are never recorded (they may carry passwords or Secret
  data); dry runs are not audited, except those touching Secrets and
  Helm dry runs (§3.2).
- WebSocket rows are written when the connection is hijacked (status 101);
  extra rows written through a plugin's `Context.Audit` have status 0.

`GET /api/audit?limit=100&before=<id>&user=&cluster=&action=` (admin) →
`{"items":[{"id","ts","user","cluster","action","target","status","detail"}],"nextBefore":<id|null>}`.

### 3.5 Hardening

- SQLite opened with `_pragma=foreign_keys(1)`; deleting a user/cluster
  removes sessions/assignments; a deleted active cluster resets
  `active_cluster_id`.
- Login rate limit: 10 failed attempts per (client IP, username) per 15 min
  → `429 rate_limited` with `Retry-After`. Behind a trusted proxy the client
  IP is the last `X-Forwarded-For` entry. Wrong current passwords in
  `POST /api/auth/password` count against the same limit. The limiter keeps
  a digest of the username, at most 10 000 entries.
- Requests must be read within 1 minute (`ReadTimeout`; headers within
  10 s). The deadline ends once the request has been read, so streams and
  long operations are not limited.
- The SQLite file and its `-wal`/`-shm` files are created `0600` (existing
  ones are tightened on start): they hold cluster credentials and sessions.
- Non-GET requests with an `Origin` header that does not match the request
  host → `403 forbidden` (CSRF defence in depth; WebSocket upgrades too).
  `X-Forwarded-Host` is only compared when `KUBEBENCH_TRUST_PROXY` is set.
- Session cookie `kubebench_session`: HttpOnly, SameSite=Lax, Secure per
  `KUBEBENCH_SECURE_COOKIES`. Expired sessions pruned hourly.
- `users.must_change_password`: set for the bootstrap admin (the default
  password is well known; a configured one stays in the environment, an
  env file or the chart's Secret, readable by whoever may read that
  Secret, through KubeBench too), for users an admin creates, and when an
  admin resets a password — every password someone else knows. While set, every
  `read`/`write`/`exec`/`admin` route returns `403 {"code":"password_change_required"}`;
  the `authed` routes (`/api/auth/*`, `/api/info`, …) keep working.
- Passwords are 8 characters to 72 bytes (the bcrypt limit).
- Cannot delete yourself or demote/delete the last admin (`409 conflict`).

## 4. API changes (core)

### 4.1 `/api/info`

```json
{
  "version": "0.5.0", "commit": "abc1234",
  "user": "alice", "role": "admin",
  "mustChangePassword": false,
  "context": "k3s-homelab", "server": "https://10.0.0.1:6443",
  "cluster": {"id": 1, "name": "k3s-homelab", "environment": "production", "readOnly": false},
  "permissions": {"role": "admin", "canRead": true, "canWrite": true, "canExec": true, "isAdmin": true, "readOnly": false},
  "capabilities": {"crds": true, "helm": true, "exec": true},
  "features": {"pathKubeconfig": true, "externalPlugins": true},
  "clusterError": ""
}
```

`permissions.role` is `admin` | `operator` | `viewer` (effective role on the
active cluster). `/api/info` waits (≤3 s) for the runtime's capability probe
to finish so capabilities are never reported `false` merely because the
probe is still running.

When the caller has no usable active cluster, `/api/info` activates the
first cluster they may use (the app reads it at boot, so nothing in flight
is surprised). It is the only route that switches clusters: every other
route answers `409 no_cluster` instead. Without any usable cluster,
`cluster` is `null` and `clusterError` says why.

`POST /api/auth/login` and `GET /api/auth/me` return
`{"id","username","role","mustChangePassword"}`.

### 4.2 Users & password

- `GET /api/users` (admin) → `[{"id","username","role","mustChangePassword","clusters":["name",…],"assignments":[{"clusterId","cluster","role"}]}]`
  (`clusters` kept for compatibility).
- `POST /api/users` (admin) `{"username","password","role"}` → `{"id"}`; the
  new user has `mustChangePassword` set.
- `PATCH /api/users/{id}` (admin) `{"role"?: "admin"|"user", "password"?: "…"}`
  (a password reset sets `mustChangePassword` and ends the user's sessions,
  except the caller's own when admins reset their own password).
- `DELETE /api/users/{id}` (admin).
- `PUT /api/users/{id}/clusters` (admin) accepts `{"assignments":[{"clusterId":1,"role":"viewer"}]}`
  or legacy `{"clusterIds":[1,2]}` (→ operator).
- `POST /api/auth/password` (authed) `{"currentPassword","newPassword"}`
  (8 characters to 72 bytes and different from the current one, else `400`;
  a wrong current password is `403 forbidden`, throttled like logins;
  clears `mustChangePassword`; other sessions of the user are revoked).

### 4.3 Clusters

- `GET /api/clusters` entries gain `"readOnly"`, `"environment"`, `"role"`
  (the caller's effective role on it).
- `PATCH /api/clusters/{id}` (admin) `{"name"?, "readOnly"?, "environment"?}`.
- `POST /api/clusters` (admin) `{"name", "content"? | "path"? | "address"?, "context"?}`
  → `{"ok": true, "id"}`. With `path` it is rejected with `403 forbidden`
  unless `features.pathKubeconfig`. Unless `KUBEBENCH_ALLOW_KUBECONFIG_EXEC`
  is set, pasted kubeconfigs whose selected context reads server-side files
  (certificate authority, client certificate/key, token file) or runs exec
  plugins or auth providers are rejected the same way: pasted content is
  untrusted, and would otherwise run commands or read files as the server's
  account. A kubeconfig added by path is one of the server's own files and
  may use them. Kubeconfigs added by path are stored flattened (referenced
  certificate files embedded).

### 4.4 Resources (fixes)

- `/api/events?kind=&ns=&name=` filters on `involvedObject.kind` as well as
  name (fixes cross-kind mixing, e.g. Deployment `web` vs Service `web`).
- `/api/logs` accepts `tail` (default 300, max 5000), `previous=true`,
  `sinceSeconds`.
- Secret values are redacted on every read route (`/api/resource`,
  `/api/resource/get`, `/api/yaml`): `data`, `stringData` and kubectl's
  `last-applied-configuration` annotation, which repeats them.

### 4.5 Kubernetes read passthrough (for external plugins)

`GET /api/k8s/{path...}` (read) proxies **GET only** to the cluster API
(`/api/...`, `/apis/...`, `/version`), returning the raw JSON. Requests whose
path targets `secrets` are rejected (`403 forbidden`; use `/api/resource`).
`watch=true` is rejected (use the live-updates SSE). Response size capped
(16 MiB).

- Also rejected: `/watch/` paths and `follow=true` (`400`), and the
  `proxy`, `exec`, `attach` and `portforward` subresources (`403`). Boolean
  flags are read the way the API server reads them (anything but `0` or
  `false` is true).
- Upstream errors become `{"error": <Status.message>, "code": "kubernetes"}`
  with the upstream status (a `401` becomes `502`, see §3.3).
- Redirects are never followed, here or by the runtime's typed, dynamic
  and metrics clients (the stored credential would go along), and an
  upstream `3xx` becomes `502 kubernetes`.
- Responses are served as `application/json` (JSON upstream) or
  `text/plain` (logs), anything else as `application/octet-stream`, with
  `X-Content-Type-Options: nosniff` and `Content-Security-Policy: sandbox;
  default-src 'none'`: the upstream type is never echoed.

### 4.6 Plugins

- `GET /api/plugins` (authed) →
  ```json
  {"plugins": [{"id":"argocd","name":"Argo CD","version":"0.5.0","description":"…",
                "category":"gitops","builtin":true,"external":false,"enabled":true,
                "status":{"available":true,"reason":""},"entry":""}],
   "settings": {"topology": {"enabled": false}}}
  ```
  `plugins` lists backend plugins (with `status` for the caller's active
  cluster; Detect results cached 60 s per cluster) and external plugins
  (`external: true`, `entry: "/plugins/<id>/<entry>"`). `settings` holds the
  admin's enable/disable choices for **any** plugin id, including
  frontend-only plugins the backend does not know.
- `PUT /api/plugins/{id}` (admin) `{"enabled": bool}` — id must match
  `^[a-z0-9][a-z0-9-]{0,39}$`.
- Backend plugin routes live under `/api/plugins/{id}/…`; disabled →
  `404 plugin_disabled`; `Detect` unavailable → `503 plugin_unavailable`.
- External plugin files: `GET /plugins/{id}/{file...}` (authed) served from
  `$KUBEBENCH_PLUGINS_DIR/<id>/` through `os.Root` (no traversal, no
  symlink escapes, no directory listings), only for valid and enabled
  external plugins (disabled → `404 plugin_disabled`) and only `.js`,
  `.mjs`, `.css`, `.json`, `.map`, `.svg` and `.png` files. `plugin.json`
  may carry an optional `category` (default `tools`). An external plugin
  whose id belongs to a built-in backend plugin is skipped; one whose id
  belongs to a built-in frontend plugin is never loaded, its catalog entry
  does not affect the built-in plugin, and Extensions shows the clash.

## 5. Backend plugin contract (`backend/internal/plugin`)

```go
package plugin

type Access int
const (
    AccessRead Access = iota + 1
    AccessWrite
    AccessExec
    AccessAdmin
)

// DryRunnable is combined with AccessWrite or AccessExec for a route whose
// handler performs a dry run whenever IsDryRun(r): such requests are
// checked as AccessRead and not audited (§3.2).
const DryRunnable Access = 1 << 8

func IsDryRun(r *http.Request) bool // exactly dryRun=true

type Manifest struct {
    ID, Name, Version, Description, Category string
}

type Status struct {
    Available bool   `json:"available"`
    Reason    string `json:"reason,omitempty"`
}

// Context is what a plugin handler gets for one request.
type Context struct {
    Runtime *kube.Runtime   // the caller's active cluster
    User    *store.User
    KV      KV              // plugin-scoped persistent key/value (SQLite plugin_kv)
    Audit   func(action, target, detail string) // extra audit rows if needed
    // SetAudit refines the router's row of this (audited) request once the
    // handler has read the body, e.g. target "release/web/shop"; never secrets.
    SetAudit func(target, detail string)
}

type KV interface {
    Get(key string) (string, bool, error)
    Set(key, value string) error
    Delete(key string) error
    List(prefix string) (map[string]string, error)
}

type HandlerFunc func(w http.ResponseWriter, r *http.Request, c *Context)

type Router interface {
    // Handle registers e.g. ("GET", "/applications/{ns}/{name}", AccessRead, h)
    // mounted at /api/plugins/<id>/applications/{ns}/{name}.
    Handle(method, path string, access Access, h HandlerFunc)
}

type Plugin interface {
    Manifest() Manifest
    Routes(r Router)
    Detect(ctx context.Context, rt *kube.Runtime) Status
}

func Register(p Plugin)            // called from init() of each plugin package
func All() []Plugin
func Get(id string) (Plugin, bool)

// Optional: plugins that keep files or run background work implement
// Starter. main.go calls StartAll once before serving; DataDir is
// $KUBEBENCH_DATA_DIR/<id> (created 0700), ctx ends at shutdown.
type Host struct{ DataDir string }
type Starter interface{ Start(ctx context.Context, host Host) error }
func StartAll(ctx context.Context, dataDir string, plugins []Plugin) error

// Helpers shared with core handlers:
func WriteJSON(w http.ResponseWriter, v any)
func WriteError(w http.ResponseWriter, status int, code string, err error)
func WriteK8sError(w http.ResponseWriter, err error) // maps apierrors → status, code "kubernetes"
```

- Built-in plugins live in `backend/internal/plugins/<id>/` and are linked
  via blank imports in `backend/internal/plugins/all/all.go`
  (`main.go` imports `_ "kubebench/internal/plugins/all"`).
- KV keys are scoped per plugin (`plugin_kv(plugin_id, key, value, updated_at)`);
  per-cluster data should prefix keys with `cluster/<clusterID>/`.
- `kube.Runtime` exposes (foundation): `CS`, `Dyn`, `Metrics`, `Config`,
  `Discovery()`, `Mapper()` (cached deferred discovery RESTMapper),
  `ResolveKind(apiVersion, kind) (gvr, namespaced, error)`,
  `ResolveResource(group, resource)`, `Cluster` (store.Cluster incl. ID),
  `Caps()` and `WaitProbed(ctx)` (the capability probe), `RawGet` (the
  §4.5 passthrough). `ResolveKind`/`ResolveResource` reset the mapper and
  retry once on a no-match error (e.g. a CRD installed since the last
  discovery), so plugins should prefer them over `Mapper()`.
  `kube.NewRuntimeForTest(cluster, cs, dyn)` builds one around fake clients
  without background work.
- `WriteK8sError` reports a Kubernetes `401` as `502` (§3.3); a no-match
  from the RESTMapper is `404 not_found`.
- Plugin route paths use `net/http` ServeMux wildcards. `Context.Runtime`
  is the caller's active cluster; requests never switch clusters.

## 6. Frontend contracts

### 6.1 i18n (`src/i18n`)

Already in place: `i18next` + `react-i18next`, languages `en`, `tr`;
`core` namespace in `src/i18n/locales/<lang>.json`; plugin namespaces via
`PluginManifest.locales = { en, tr }` (files `src/plugins/<id>/locales/<lang>.json`);
`useManifestText()`/`translateManifestText()` for manifest strings;
`setLanguagePreference()`; `formatNumber/formatDateTime/formatBytes` in
`src/i18n/format.ts`; `npm run i18n:check`.

Conventions: nested keys grouped by screen (`releases.columns.chart`);
`{{var}}` interpolation; plurals via i18next `_one`/`_other` suffixes;
Kubernetes nouns (Pod, Deployment, namespace, …) stay untranslated in `tr`.

### 6.2 API client (`src/data/apiClient.ts`)

`apiFetch<T>(path, init?)` (prefix `/api`, JSON in/out, throws
`ApiError {status, code, message}`), `qs(params)` (URL-encodes every value;
skips undefined/empty), helpers `apiGet/apiPost/apiPut/apiPatch/apiDelete`.
All providers use it; no hand-built query strings.

### 6.3 Session, permissions, mode

- `src/data/mode.ts` `DataMode` gains `version`, `permissions`, `cluster`,
  `features`, `mustChangePassword`.
- `src/core/permissions.ts`: `usePermissions()` → `{role, canRead, canWrite,
  canExec, isAdmin, readOnly}` (mock mode: everything true) and
  `<Can perm="write|exec|admin" fallback?>`. Mutation UI is hidden or
  disabled (with a tooltip explaining why) when not permitted.
- **Mock mode policy**: mock only when `import.meta.env.DEV`, or the URL has
  `?demo`, or the build sets `VITE_DEMO_MODE=true`. A production build whose
  backend is unreachable shows a boot error screen with Retry — never mock.
  `?demo` and `VITE_DEMO_MODE=true` force demo data without contacting the
  backend; the dev server falls back to it only when the backend is
  unreachable or answers 5xx (a `401` always shows the sign-in screen).
  Sign-out is disabled in demo mode, which has no session.
- Status bar shows the cluster environment (production = red background,
  staging = amber, development = green) and a "Read-only" item when
  `readOnly` or role viewer; the user item opens a menu (Change password,
  Sign out). The About command shows version/commit.

### 6.4 Plugin SDK v2 (`src/core/plugin`)

`PluginManifest` additions:

```ts
version?: string            // shown in Extensions view
description?: string        // i18n key
category?: 'core' | 'workloads' | 'gitops' | 'storage' | 'security' | 'observability' | 'tools'
core?: boolean              // cannot be disabled (workloads, clusters, settings, terminal, editor, extensions, users, audit)
backendPlugin?: string      // backend plugin id this plugin needs; unavailable → greyed with reason
locales?: Record<string, object>
resourceActions?: ResourceActionContribution[]
settingsSections?: SettingsSectionContribution[]
```

```ts
interface ResourceRef { kind: string; apiVersion?: string; namespace?: string; name: string }
interface ResourceActionContribution {
  id: string
  title: string              // i18n key in the plugin namespace
  icon?: string
  kinds: string[]            // e.g. ['Deployment','Service'] or ['*']
  permission?: 'read' | 'write' | 'exec'
  run: (ref: ResourceRef) => void
}
interface SettingsSectionContribution { id: string; title: string; component: ComponentType }
```

- The registry filters disabled/unavailable plugins out of the activity
  bar, sidebar, commands and resource actions, using a zustand
  `pluginStatusStore` loaded from `/api/plugins` at boot (mock mode: all
  enabled & available). Built-in frontend plugins default to enabled;
  external plugins default to **disabled** until an admin enables them.
- `registry.resourceActionsFor(kind)` feeds row context menus and detail
  page toolbars in the workloads/CRD views, through
  `resourceActionItems(ref)` (`core/plugin/resourceActions.ts`) and
  `<ResourceActionButtons resource>`; an action whose permission is not
  held is shown disabled with the reason.
- **Extensions** plugin (core): activity-bar entry listing every plugin
  (name, version, category, builtin/external, enabled, availability +
  reason); admins toggle enable/disable.
- Additions to the shell contract:
  - `TabDescriptor.titleKey` / `titleNs` / `titleValues`: the tab strip
    shows the title translated from that key, so open tabs follow language
    changes; `title` is the fallback.
  - Activity bar entries without `order` sort at 50: after the built-in
    views (0–6), before Extensions (90).
  - Bottom panel tabs may list request `kinds`; `showInPanel(kind, props)`
    (`core/plugin/panelRequests.ts`) shows a request in the tab that handles
    it, e.g. `pod-logs` / `pod-exec` with `{namespace, pod, container}`
    (the terminal plugin). Like `openTab`, it keeps plugins from importing
    each other.
  - External plugins register only under the id of an enabled external
    plugin whose script is loading, and never as `core`.

### 6.5 Plugin-owned data (`src/data/pluginData.ts`)

```ts
export function pluginApi(pluginId: string): {
  get<T>(path: string, params?: Record<string, unknown>): Promise<T>
  post<T>(path: string, body?: unknown, params?: Record<string, unknown>): Promise<T>
  put<T>(…), patch<T>(…), del<T>(path, params?)
}   // → /api/plugins/<pluginId><path>

export function definePluginData<T>(pluginId: string, impl: {
  mock: () => T
  live: (api: ReturnType<typeof pluginApi>) => T
}): () => T          // memoized; picks by dataMode.mode, never falls back

export function usePluginQuery<T>(fn: () => Promise<T>, deps: DependencyList): QueryState<T>
export function usePluginStatus(pluginId: string): { enabled: boolean; available: boolean; reason?: string; loading: boolean }
```

Plugin capabilities live in `src/plugins/<id>/data/` (`types.ts`,
`provider.ts` interface, `mock.ts`, `live.ts`, `index.ts` exporting the
`definePluginData` getter). Core capabilities stay in `src/data/providers.ts`.

### 6.6 Shared UI building blocks (`src/shared`)

`Dialog` (modal; Esc closes; focus first field), `ConfirmDialog`
(danger variant; **type-to-confirm** the resource name when the cluster
environment is `production` or the action is irreversible at scale),
`Form` primitives (`Field`, `TextInput`, `NumberInput`, `Select`,
`Checkbox`, `Textarea`, `KeyValueEditor`), `notify` toasts
(`notify.success/error/info`, `<Toaster/>` mounted by the shell),
`Tabs` (in-page tab strip), `Unavailable` (EmptyState variant with reason),
`CodeEditor` / `DiffEditor` (Monaco **bundled locally** — no CDN; lazy
loaded; theme-aware; languages yaml/json/plaintext/shell).

Destructive and rollback actions of the built-in views (Pod delete, Helm
uninstall and rollback) ask through `ConfirmDialog`, so production
clusters always require the name to be typed.

### 6.7 Themes

`settings.theme`: `dark` | `light` | `system`; `<html data-theme>`;
`theme.css` defines every `--vscode-*` token for both themes (Dark+ /
Light+ palettes). Monaco follows (`vs-dark` / `vs`).

### 6.8 External (runtime) plugins

- Layout on disk: `$KUBEBENCH_PLUGINS_DIR/<id>/plugin.json`
  (`{"id","name","version","description","entry":"index.js"}`) + the IIFE
  bundle.
- Boot: after `/api/info`, the app fetches `/api/plugins`, injects
  `<script src="/plugins/<id>/index.js">` for each enabled external plugin
  (5 s timeout each, failures reported in Extensions), then renders.
- The bundle calls `window.KubeBench.registerPlugin(manifest)`.
  `window.KubeBench` (`src/core/plugin/globalSdk.ts`) exposes
  `{ apiVersion: 1, version, React, jsxRuntime, registerPlugin, i18n: {t, registerTranslations, useTranslation}, api: {apiFetch, pluginApi, k8sGet}, data: {usePluginQuery, refreshData}, layout: {openTab}, permissions: {usePermissions}, ui: {Table, Badge, EmptyState, Toolbar, Dialog, ConfirmDialog, CodeEditor, notify} }`.
  `i18n.useTranslation(ns)` is a hook returning `t` bound to the namespace
  that re-renders on language changes. Types for plugin authors:
  `src/core/plugin/sdk/kubebench.d.ts`, type-checked against the
  implementation.
- Example: `examples/plugins/hello-cluster` (Vite library build, IIFE,
  `react` → `KubeBench.React` globals). Excluded from the app build.

## 7. Feature specifications

Tab kind contract that stays stable: `pod-exec` and `pod-logs` with props
`{namespace, pod, container}`; `yaml` with `{text, readOnly}` or `{ref}`
and `new-resource` with `{namespace?, kind?}` (see 7.2);
`workloads-detail`, `generic-detail`, `crd-instances`.

### 7.1 Terminal & logs (terminal plugin)

- `GET /api/exec/ws?ns=&pod=&container=&command=` (exec, audited as
  `exec.session` at start) — WebSocket. Client→server text frames
  `{"type":"input","data":"…"}` / `{"type":"resize","cols":N,"rows":N}`;
  server→client **binary** frames = terminal output, text frames
  `{"type":"exit","code":N}` / `{"type":"error","message":"…"}`.
  TTY + stdin; default command tries `bash` then `sh`. Uses client-go
  `remotecommand` (WebSocket executor with SPDY fallback) and a
  `TerminalSizeQueue`. Origin checked; 30 s ping; idle timeout 30 min.
- `GET /api/logs/stream?ns=&pod=&container=&tail=&previous=&timestamps=&sinceSeconds=`
  (read) — Server-Sent Events: first `event: pods` with
  `{"pods":["<pod>/<container>"],"total":N}` (followed containers, matching
  pods), then `data:` one log line per event, `event: error`, `event: end`,
  `: ping` every 15 s. Also `?selector=<labelSelector>` (≤10 pods, lines
  prefixed `[pod/container]`). Failures before the stream starts are JSON
  errors (§3.3).
- Both are authorized when they start and checked again at every ping
  (terminal) or heartbeat (logs): once the session ends (logout, password
  change, user deleted), a password change is due, the cluster is removed,
  the caller's assignment is gone or, for a terminal, the role or the
  cluster's read-only flag no longer allows exec, the terminal ends with
  `{"type":"error","message":"the session was closed: …"}` and the log
  stream with `event: error` carrying the same text. Switching the active
  cluster does not end them. A user may have at most 10 terminals and 20
  log streams open; more are refused with `429 rate_limited` before the
  upgrade or stream starts. A log stream whose client stops reading ends
  once a write has waited 10 s.
- UI: xterm.js terminal (`@xterm/xterm` + fit addon), multiple sessions as
  sub-tabs in the bottom Terminal panel, container picker, reconnect;
  logs viewer with follow/pause, tail size, previous container, timestamps,
  wrap, filter, level colouring, download, 10 000-line ring buffer.
  Mock mode: simulated terminal + deterministic simulated stream.

### 7.2 Resource editing (core + editor/workloads plugins)

- `POST /api/resource/apply?dryRun=&force=&ns=` (write; exactly
  `dryRun=true` → read and not audited, a real server-side dry run, unless
  a document is a Secret: then write and audited, §3.2). Body:
  YAML or JSON, any number of documents (`---`-separated YAML or
  concatenated JSON; `kind: List` is flattened, empty documents skipped),
  at most 4 MiB (`413`). Server-side apply with `fieldManager=kubebench`,
  mapping through the RESTMapper (`ResolveKind`). Namespaced objects that
  name no namespace go to `ns`, else `default`; the namespace of a
  cluster-scoped object is ignored with a warning. `dryRun` and `force`
  accept only `true`/`false`.
  - The whole request is refused (`400`) before anything is applied when a
    document is not valid YAML/JSON or lacks apiVersion, kind or
    metadata.name (`bad_request`, the message names the document), or is a
    Secret whose data/stringData values or last-applied annotation are
    redaction placeholders (`redacted_secret`).
  - Otherwise every object is attempted in order and the answer is `200`:
    `{"results":[{"apiVersion","kind","namespace"?,"name","action"?:"created|configured|unchanged","error"?,"code"?,"warnings"?}]}`.
    A failed object carries `error` and `code` instead of `action`:
    `conflict` for a server-side apply conflict with another field manager
    (applying again with `force=true` takes the fields over), `not_found`
    for a kind the cluster does not serve, `kubernetes` otherwise.
    `unchanged`: the object's content without managedFields,
    resourceVersion and status is the same after the apply.
  - Audit target: the first object, `<kind>/<ns>/<name>` with
    ` (+N more)`; detail: the outcome counts, e.g.
    `{"configured":1,"failed":1}`, and `"force":true`.
- `POST /api/resource/edit?apiVersion=&kind=` or `?key=` `&ns=&name=&dryRun=`
  (write; exactly `dryRun=true` → read and not audited, a real dry run,
  except for Secrets, §3.2). Body `{"original","edited"}`: the object's YAML
  or JSON as editing started from and as edited (one document each, at most
  8 MiB together). The YAML editor's Apply: unlike server-side apply, which
  keeps fields other managers own when a manifest leaves them out, the edit
  is sent like `kubectl edit` as the JSON merge patch from original to
  edited (`fieldManager=kubebench`), so a removed field is removed and
  fields the edit leaves alone keep their live values. Both documents must
  be the named object (apiVersion, kind, name; the namespace may be
  omitted), else `400 bad_request`. Redaction placeholders left unchanged
  are not part of the patch, so a Secret's labels can be edited without
  revealing it; a value changed to a placeholder is `400 redacted_secret`.
  Answers like apply with one result (`configured` or `unchanged`; nothing
  is sent without changes); audit target `<kind>/<ns>/<name>`, detail the
  outcome count.
- `GET /api/yaml?apiVersion=&kind=&ns=&name=&neat=true` (read). With
  `apiVersion`, any kind the cluster serves (RESTMapper; a namespaced kind
  needs `ns`); without it, the kinds v0.4 knew by name (`kind`, `name`
  required). managedFields are always stripped; `neat=true` also strips
  status, uid, resourceVersion, generation, creationTimestamp (and the null
  creationTimestamp of templates), selfLink and the
  `kubectl.kubernetes.io/last-applied-configuration` and
  `deployment.kubernetes.io/revision` annotations, dropping annotations
  left empty. Secret values are redacted here as on every read but the
  reveal, and so is the last-applied annotation of Secrets, which repeats
  them.
- `DELETE /api/resource?apiVersion=&kind=` or `?key=` (a generic catalog
  key) `&ns=&name=&propagation=Background|Foreground|Orphan&gracePeriod=`
  (write). Namespaces are refused (`400`): they are deleted with
  `DELETE /api/namespaces` (§7.3) and its protections. Audit target
  `<kind>/<ns>/<name>`.
- `PATCH /api/resource/metadata?apiVersion=&kind=` or `?key=` `&ns=&name=`
  (write), body `{"labels":{…},"annotations":{…}}`: a JSON merge patch, a
  null value removes the key. The audit detail lists the changed keys,
  never values. A Secret's labels or annotations cannot be set to a
  redaction placeholder (`redacted_secret`).
- `POST /api/secrets/reveal?ns=&name=` (exec) → the Secret with its values
  (`data`, base64; managedFields stripped), `Cache-Control: no-store`. It
  has its own route because the access level belongs to the route: a
  query flag on `GET /api/resource/get` could not raise it. It changes
  nothing but is audited like every exec request (action
  `POST /api/secrets/reveal`, target `secret/<ns>/<name>`), refused attempts
  included.
- UI, editor plugin (core): the `yaml` tab kind with `{ref: {apiVersion,
  kind, namespace, name}}` shows the object full or neat, copies and
  downloads it, and edits it: Edit → Review changes (DiffEditor, live neat
  ↔ edited) → Dry run → Apply through `POST /api/resource/edit`, with the
  result. The `new-resource` tab applies with server-side apply, where a
  conflict offers "Force apply" (ConfirmDialog). Without `ref` it keeps the v0.4
  `{text, readOnly}` behaviour. The `new-resource` tab kind
  (`{namespace?, kind?}`): templates for the namespace, import of files
  (picker or drag & drop, any number of documents), dry run, apply.
  Plugins open both through `core/plugin/editorTabs` (`openYamlTab`,
  `openNewResourceTab`). Unapplied edits survive tab switches and mark the
  tab dirty (`TabDescriptor.dirty`, `layoutStore.setTabDirty`); closing a
  dirty tab asks first, and so does leaving the page.
- UI, workloads plugin: rows and detail pages get YAML, a generic Delete
  (propagation choice with the cascade explained, ConfirmDialog;
  cluster-scoped objects always ask for the name), a labels/annotations
  editor (KeyValueEditor) and, for Secrets, "Reveal values" (exec, a
  warning first; the values live in the dialog only).

### 7.3 Workload & node operations (core + workloads plugin)

All write unless noted; audited.

- Nodes: `POST /api/nodes/cordon?name=&unschedulable=true|false`;
  `POST /api/nodes/drain?name=&ignoreDaemonSets=true&deleteEmptyDirData=false&force=false&gracePeriod=-1&timeoutSeconds=120`
  (k8s.io/kubectl/pkg/drain; eviction API; result lists evicted/failed pods
  and PDB blocks); `PUT /api/nodes/taints?name=` body `{"taints":[{key,value,effect}]}`.
- Deployments: `GET /api/deployments/history?ns=&name=` (read; revisions from
  owned ReplicaSets: revision, images, change-cause, created, replicas);
  `POST /api/deployments/rollback?ns=&name=&revision=`;
  `POST /api/deployments/pause?ns=&name=&paused=true|false`.
- `POST /api/workloads/restart?kind=Deployment|StatefulSet|DaemonSet&ns=&name=`;
  `POST /api/workloads/scale?kind=Deployment|StatefulSet|ReplicaSet&ns=&name=&replicas=`.
- Pods: `DELETE /api/pods?…&force=true` (grace 0); `POST /api/pods/bulk-delete`
  `{"items":[{"namespace","name"}],"force":bool}`; `POST /api/pods/debug?ns=&pod=&image=&target=` (exec)
  → ephemeral container name (attach via 7.1 with `container=`).
- CronJobs: `POST /api/cronjobs/trigger?ns=&name=` (Job from template,
  annotation `cronjob.kubernetes.io/instantiate=manual`);
  `POST /api/cronjobs/suspend?ns=&name=&suspend=true|false`.
- Namespaces: `POST /api/namespaces` `{"name","labels"?}`; `DELETE /api/namespaces?name=`.
- UI: node actions (cordon/uncordon, drain dialog with results, taints
  editor), rollout history + rollback, pause/resume, restart/scale for
  StatefulSet/DaemonSet/ReplicaSet, multi-select bulk delete on pod lists,
  force delete, debug container (opens a terminal), CronJob trigger/suspend,
  namespace create/delete (type-to-confirm).

Details settled by the implementation:

- Node list entries gain `unschedulable` and `taints` (`[{key,value,effect}]`),
  Deployment entries `paused`; all omitted when false or empty.
- Boolean switches default to the action the route names: `unschedulable`,
  `paused` and `suspend` are `true` when absent. Mutations without a result
  answer `{"ok":true}`.
- Drain: defaults as in the path above; `timeoutSeconds` 1–600, `gracePeriod`
  −1 (the pod's own) to 3600. It cordons first and answers `200` when the
  evictions ended: `{"node","cordoned","completed","evicted":["ns/pod"],
  "failed":[{"pod","error"}],"blockedByPDB":["ns/pod"],"skipped":[{"pod",
  "reason":"daemonSet"|"mirror"}],"errors":[…]}`. Pods a drain must not lose
  (no controller without `force`, emptyDir without `deleteEmptyDirData`,
  DaemonSet pods without `ignoreDaemonSets`) stop it before any eviction and
  are reported in `errors`, like kubectl. An eviction a PodDisruptionBudget
  refuses (429) is retried every 5 s until the timeout.
- Taints: key a qualified name, value a label value, effect `NoSchedule` |
  `PreferNoSchedule` | `NoExecute`, each key once per effect, else `400`.
  Answers `{"taints":[…]}`; taints kept unchanged keep their `timeAdded`.
- History: `{"currentRevision","paused","revisions":[{"revision","replicaSet",
  "images","changeCause","created","replicas","readyReplicas","current"}]}`,
  newest first. The current revision is the ReplicaSet whose pod template the
  Deployment has (the revision annotations lag behind during a rollout).
- Rollback (kubectl rollout undo): `409 conflict` while paused or for the
  current revision, `404 not_found` for a revision the Deployment does not
  have.
- Restart and scale check `kind` (`400` otherwise); scale goes through the
  `scale` subresource. `/api/deployments/scale|restart` keep working.
- `DELETE /api/pods?force=true` deletes with grace period 0. Bulk delete
  takes 1–200 items (mock mode too; the list's Delete button is disabled
  beyond 200 selected Pods, with the reason as tooltip) and answers `{"results":[{"namespace","name","error"?}]}`
  in order; its audit detail lists the pods (`{"failed","force","pods"}`).
  A list too long for one row is recorded across several rows of the same
  request, each with `"part":"<i>/<n>"`, so no pod is left out.
- Debug: `image` defaults to `busybox:1.37`, `target` must name a container
  of the pod; answers `{"namespace","pod","container"}`, the container being
  `debugger-xxxxx` with stdin and TTY, once that container runs, so the
  terminal opened next can attach. It waits up to 60 s and answers
  `409 conflict` with the kubelet's reason when the container cannot start
  (`ImagePullBackOff`, `InvalidImageName`, …), ended already, or still
  waits; the audit row then records that refusal.
- Trigger answers `{"namespace","job"}`; the Job is the CronJob's controller-
  owned child, like `kubectl create job --from=cronjob/<name>`.
- Namespaces: the name must be a DNS-1123 label and labels valid. Deleting a
  system namespace (`default`, `kube-system`, `kube-public`,
  `kube-node-lease`) or `KUBEBENCH_NAMESPACE` answers `403 forbidden` unless
  `force=true`. The UI does not offer to delete system namespaces; for
  KubeBench's own it offers "Delete anyway" after the refusal.
- Refusals by KubeBench itself use `bad_request`, `not_found`, `conflict` and
  `forbidden`; errors from the cluster keep the code `kubernetes`.
- Frontend: `DataProviders.workloadOps` (`WorkloadOpsProvider`); the shared
  `Table` gains an optional checkbox `selection`.

### 7.4 Helm plugin v2 (backend plugin `helm`)

Backend `internal/plugins/helm`; routes under `/api/plugins/helm`
(old `/api/helm/*` removed). Repos/caches under `$KUBEBENCH_DATA_DIR/helm/`
(`repositories.yaml` 0600 with any credentials, `cache/`,
`registry/config.json`, `uploads/<id>.tgz`), shared by every cluster; the
server account's own Helm configuration and Helm plugins are never used.
Detect: can list Secrets labelled `owner=helm` (run after the capability
probe, cached like every Detect).

`GET /releases?ns=`, `GET /releases/{ns}/{name}` (info, chart metadata,
notes, hooks), `GET /releases/{ns}/{name}/values?all=&revision=` (exec:
values often carry credentials, so reading them is like revealing a
Secret and is audited), `GET /releases/{ns}/{name}/manifest?revision=`,
`POST /releases/{ns}/{name}/rollback {revision}`,
`DELETE /releases/{ns}/{name}?keepHistory=`,
`POST /install {name,namespace,createNamespace,chart:{repo,name,version}|{ref,version}|{uploadId},values,dryRun}` → `{release,manifest,notes}`,
`POST /releases/{ns}/{name}/upgrade {chart…,values,reuseValues,dryRun}` → `{manifest,notes,currentManifest}`,
`GET /repos`, `POST /repos {name,url,username?,password?}` (admin),
`DELETE /repos/{name}` (admin), `POST /repos/update` (write),
`GET /charts/search?q=`, `GET /charts/values?repo=&chart=&version=`,
`GET /charts/oci/values?ref=&version=` (write),
`POST /charts/upload` (multipart `.tgz`, write; for offline installs).
Install and upgrade are `write` with or without `dryRun`; every other
read is `read`.

Shapes (they match `src/plugins/helm/data/types.ts`):

- A release is `{name,namespace,chart,chartVersion,appVersion,revision,status,updated,description}`
  (Helm's status verbatim, RFC 3339 times); the detail adds
  `firstDeployed`, `notes`, `chartInfo` (Chart.yaml), `hooks`
  (`{name,kind,events,weight,phase,startedAt,completedAt}`) and `history`
  (newest first). The list includes releases uninstalled with
  `keepHistory` (status `uninstalled`), which can be rolled back.
- `values` → `{"values": "<yaml>"}` (`""` when empty; `revision` 0 or
  absent = latest; `all=true` merges the chart defaults); `manifest` →
  `{"revision","manifest"}` with hooks appended. Rollback answers
  `{"release"}` (the new revision; `revision` 0 = the previous one),
  uninstall `{"ok":true,"release"}`.
- `values` in install/upgrade bodies is YAML text. An upgrade without
  `chart` keeps the release's current chart (values-only upgrade); with
  `reuseValues` the values are merged over the current ones, without they
  replace them (Helm's `--reset-values`, so an empty text resets to the
  chart defaults). Reuse is Helm's `--reset-then-reuse-values`: the chart's
  own defaults stay the defaults, so a later reset returns to them.
  Upgrades and rollbacks keep 10 revisions; nothing waits for readiness
  (no `--wait`); hooks and deletions time out after 5 min.
- Dry runs (`dryRun=true` in the query or the body flag) use Helm's
  server-side dry run: rendered with the cluster's API versions and
  lookups, nothing created, the namespace not created either. Because a
  template `lookup` reads any object with KubeBench's own credential, dry
  runs need `write` and are audited (detail `"dryRun":true`). Upgrade dry
  runs return the current revision's manifest as `currentManifest` for
  the diff.
- Every manifest returned (release manifests of any revision, install and
  upgrade answers, `currentManifest`) shows core Secrets redacted like
  every other read path (§3.1: `data`/`stringData` values become
  `<N bytes, redacted>`, the last-applied annotation `<redacted>`; a
  redacted Secret is re-serialized with sorted keys, other documents are
  verbatim). A document that may be a Secret but cannot be parsed is
  replaced by a comment.
- Chart references: `{repo,name}` must name an added repository and a
  chart in its cached index; `{ref}` must be `oci://` (no digest);
  `{uploadId}` an unexpired upload. Local paths and plain URLs are never
  accepted. `GET /charts/values` takes `repo`+`chart` or `uploadId`
  (plus `version`) as query parameters, `GET /charts/oci/values` takes
  `ref` (plus `version`); both → `{chart,values}` (values.yaml as
  written). Added repositories are the admin's allow-list of chart
  sources; an OCI reference names any host the server then connects to,
  so every OCI fetch needs `write` (an OCI `ref` on `/charts/values` is a
  `400`).
- The OCI registry client is HTTPS only: Helm's default transport would
  fall back to plain HTTP when a host answers HTTPS with HTTP, and then
  stay on HTTP for every later pull of the shared client.
- `GET /repos` → `[{name,url,authenticated,chartCount,updated,error?}]`:
  credentials are never returned and never audited; a URL with userinfo is
  refused (`400`). Adding downloads the index first (bad URL or
  credentials → `400`, a duplicate name → `409`). `POST /repos/update` →
  `{"repos":[{name,ok,error?}]}`.
- `GET /charts/search` → at most 100 `{repo,name,version,appVersion,description,icon,deprecated,versions:[{version,appVersion,created,deprecated}]}`
  matching name, description or keywords; exact names first.
- Upload: multipart field `chart`, ≤ 20 MiB, validated with
  `loader.LoadArchive` → `{uploadId,chart,expiresAt}`; uploads expire
  (and are removed) after 24 h.
- Audit targets: `release/<ns>/<name>` (install refines its row with the
  body's release and `{"chart","createNamespace"}`), `repo/<name>`,
  `chart/<name>` for uploads.
- Helm builds its own Kubernetes clients from the cluster's rest.Config;
  unlike the runtime's clients (§4.5) they follow redirects. KubeBench
  pins them to the API server's host, so a redirect elsewhere fails
  before the stored credential is sent; redirects within the API server
  are followed. Repository and registry downloads use Helm's getters,
  which follow redirects as well (Go drops the Authorization header on a
  redirect to another host).

UI: releases list/detail (overview, values user/all per revision, manifest,
notes, history with rollback + values diff between two revisions),
upgrade (values editor, version picker, dry-run manifest diff), install
wizard (search → version → values → dry-run → install; or upload `.tgz`),
repositories (add/remove/update), uninstall (keep-history option). Tab
kinds: `helm-releases`, `helm-release-detail` and `helm-upgrade` (props
`{namespace,name}`; tab ids `helm-release:<ns>/<name>`,
`helm-upgrade:<ns>/<name>`), `helm-install`, `helm-repos`.

### 7.5 Argo CD plugin (backend plugin `argocd`, new)

Works through the Argo CD CRDs (no Argo CD API token). Detect: discovery
serves `argoproj.io/v1alpha1` `applications`.

`GET /applications?ns=` → summaries (name, namespace, project, repo, path or
chart, targetRevision, destination, sync, health, operationPhase,
lastSyncedAt, revision, autoSync); `GET /applications/{ns}/{name}` →
detail (sources, status.resources with sync/health, conditions, history,
operationState, summary images/URLs); `POST /applications/{ns}/{name}/sync
{revision?,prune?,dryRun?,resources?,syncOptions?}` (sets `.operation`,
`initiatedBy.username = "kubebench:<user>"`; 409 if an operation is
running); `POST …/refresh?hard=` (annotation
`argocd.argoproj.io/refresh`); `POST …/rollback {historyId}` (409 when
automated sync is on); `DELETE …?cascade=` (adds
`resources-finalizer.argocd.argoproj.io` when cascading);
`GET /projects`; `GET /applicationsets`.

- Sync and rollback build `.operation` like the Argo CD API server: the
  controller does not read `spec.syncPolicy.syncOptions` for a requested
  operation, so they are copied in unless the request names its own
  (`syncOptions: []` syncs without any); an empty `revision` syncs the
  source's `targetRevision` (multi-source apps: each source's, and a
  `revision` is refused with `400`). Rollback (`{historyId, prune?}`)
  copies the history entry's revision and source(s) and uses the apply
  strategy (no hooks). `409 conflict` also while `.operation` is still
  pending; the patch carries the checked `resourceVersion`.
- `DELETE …?cascade=` defaults to `true` (as in Argo CD); `false` removes
  the resources finalizer, so the managed resources stay.
- Every mutation is `write` (not DryRunnable: an Argo CD dry-run sync
  still patches the Application). Sync and rollback add an
  `argocd.sync`/`argocd.rollback` audit row with the requested options.

UI: applications (grouped by project, sync/health badges), detail
(resource tree → open resources in KubeBench via tab kinds, conditions,
history + rollback, operation state), sync dialog (prune, dry-run,
revision, selective resources), refresh/hard refresh, delete (cascade,
type-to-confirm), projects, ApplicationSets; unavailable state when Argo
CD is not installed.

### 7.6 Storage plugin (backend plugin `storage`, new) — PV/PVC creator

Detect: always available. `GET /overview` (storage classes incl. default
marker, counts, capacity), `GET /csidrivers`, `GET /snapshots?ns=` (when
snapshot CRDs exist), `POST /pvcs?dryRun=`, `POST /pvs?dryRun=`,
`POST /storageclasses?dryRun=`, `PATCH /pvcs/{ns}/{name}/resize {size}`.
Bodies are validated server-side (DNS-1123 names, quantities, access modes).

- Also `GET /pvcs?ns=` and `GET /pvs` (read): claim and volume summaries
  for the plugin's lists and wizards (a claim's `expandBlocker`:
  `not_bound` | `no_class` | `class_missing` | `not_allowed` | `""`).
- Create bodies are form-shaped (the server builds the object; unknown
  fields are refused): PVC `{namespace, name, storageClassName?, accessModes,
  size, volumeMode?, volumeName?, dataSource?: {kind, name}, labels?}`
  (omitted class = cluster default, `""` = none); PV `{name, capacity,
  accessModes, reclaimPolicy?, storageClassName?, volumeMode?, mountOptions?,
  labels?, nodes?, hostPath|nfs|local|csi}`; StorageClass `{name,
  provisioner, parameters?, reclaimPolicy?, volumeBindingMode?,
  allowVolumeExpansion?, isDefault?, mountOptions?, labels?}`. They answer
  `{apiVersion, kind, namespace?, name, dryRun, warnings?}`; each create
  route is `write` + DryRunnable. References are checked before the create
  (`400`): the PV to bind must be Available and fit (class, size, modes,
  volume mode); a clone or restore must be at least as large as its source.
  A PV's `nodes` must exist; its required node affinity matches their
  `kubernetes.io/hostname` labels (the node name when unlabelled).
- Resize (write) answers the updated claim summary; a size that does not
  grow is `400`, a claim that is not Bound or whose class does not allow
  expansion `409 conflict`. `GET /snapshots` without the CRDs is
  `503 capability_unavailable`.
- Audit: one router row per mutation. Creates refine it through
  `SetAudit` with the object the body names (`pvc/<ns>/<name>`, `pv/<name>`,
  `storageclass/<name>`) and a short JSON detail (size, class, source);
  resize adds `{"from","to"}` to the row whose target the URL already names.

UI: overview, PVC/PV/StorageClass lists, wizards — PVC (storage class,
access modes, size+unit, volume mode, bind to an Available PV, clone from
PVC/snapshot, labels), PV (capacity, access modes, reclaim policy, storage
class, volume mode, source: hostPath | NFS | local + node affinity | CSI),
StorageClass; live YAML preview, Validate (dry-run), Create → open the new
resource; Expand PVC.

### 7.7 Security plugin (backend plugin `security`, new)

`POST /scan` (read) runs checks and returns a report
`{generatedAt, score:{overall, byCategory}, counts, findings:[{key, checkId,
title, severity, category, frameworks:["CIS 5.2.2","PSS baseline"], resource:{kind,namespace,name},
message, remediation, accepted?:{by,reason,at,expiresAt}}], skipped:[{checkId, reason}]}`;
`GET /report` (last report, cached per cluster); `GET /checks` (catalog);
`POST /accept {key, reason, expiresAt?}` / `DELETE /accept/{key}` (write;
stored in plugin KV, shared by all users of the cluster);
`GET /export?format=md|csv`.

≈25 checks: Pod Security Standards baseline/restricted (privileged, host
namespaces, hostPath, hostPorts, capabilities, privilege escalation,
runAsNonRoot, seccomp, …), workload hygiene (requests/limits, probes,
`:latest`), RBAC (cluster-admin bindings, wildcards, secrets access,
escalate/bind/impersonate, default SA tokens), network (namespaces without
NetworkPolicy), secrets in env vars, PSA labels, workloads in `default`.
System namespaces excluded by default (toggle).

UI: overview (score, per-category small multiples, severity tiles),
findings table (filters, grouping, accept/un-accept, open resource),
checks catalog, export.

Details settled by the implementation (shapes match
`src/plugins/security/data/types.ts`):

- `POST /scan` takes an optional body `{"includeSystem": bool}` (unknown
  fields `400`). System objects are the namespaces `kube-system`,
  `kube-public`, `kube-node-lease` and the cluster's own RBAC: the
  `kubernetes.io/bootstrapping=rbac-defaults` label, `system:*` names,
  bindings whose subjects are all `system:` users/groups or ServiceAccounts
  of a system namespace (also as `system:serviceaccount:<ns>:<name>` users
  and `system:serviceaccounts:<ns>` groups), and ClusterRoles only such
  bindings use. A binding without the bootstrap label that names
  `system:anonymous`, `system:unauthenticated`, `system:authenticated` or
  `system:serviceaccounts` is never a system object: it grants to
  everyone.
- Cost: every kind is listed once, cluster-wide, in pages of 500 and at
  most 5 000 objects per kind; a report holds at most 20 000 findings.
  Cut lists and kinds that cannot be listed become `warnings`; checks
  whose objects could not be listed at all are `skipped` with the reason.
  Nothing listable at all answers the Kubernetes error. One scan runs per
  cluster at a time (≤ 2 min): a request with the same options waits for
  it and gets its result, other options answer `409 conflict`. Scans are
  read and not audited.
- Workload checks read pod templates: Deployments, StatefulSets,
  DaemonSets, CronJobs, and ReplicaSets, Jobs and Pods not controlled by
  one of those, so a problem is reported once, on the object to fix.
- The report adds `cluster`, `options`, `warnings`, `inventory` (objects
  listed per kind) and `systemNamespaces`; `resource` carries
  `apiVersion`; a finding has `evidence` (container names, fields, rules;
  never values) and `message` is the English text with the evidence.
  `byCategory` is a list `[{category, score, checks, evaluated, failed,
  accepted, bySeverity}]` in the order `pss-baseline`, `pss-restricted`,
  `hygiene`, `rbac`, `network`, `secrets`, `namespaces`.
- Score: the severity-weighted share (critical 10, high 6, medium 3,
  low 1) of (check, object) pairs that pass, where accepted findings
  pass; rounded down; `null` when nothing was evaluated. `counts` has the
  open findings per severity plus `accepted` and `total`. Reports are
  computed from the cached scan on every read, so accepting a risk
  changes them without a new scan. Before the cluster's first scan (the
  cache is in memory) `GET /report` answers `null` and `GET /export`
  `404 not_found`.
- Finding key `<checkId>:<Kind>:<namespace>:<name>`. `POST /accept`
  checks the key (known check and kind), a reason of 1–500 characters and
  an `expiresAt` in the future, and answers the acceptance; an expired
  acceptance no longer applies. `GET /accepted` (read) lists every stored
  acceptance with `{key, checkId, resource, by, reason, at, expiresAt?,
  expired}`, so stale ones can be removed. Audit target `finding/<key>`;
  accept adds `{"checkId","expiresAt"}`.
- Export: `Content-Disposition: attachment;
  filename="security-<cluster>-<yyyymmdd-hhmmss>.md|csv"`; CSV cells a
  spreadsheet would run as formulas are prefixed with `'`; Markdown
  escapes HTML (`& < >` as entities) and Markdown punctuation in every
  value taken from the cluster or a user, so a reason cannot add markup,
  links or images.

### 7.8 Access plugin (backend plugin `access`, new) — RBAC

`GET /subjects`, `GET /subjects/{kind}/{name}?namespace=` (effective
rules with the binding path), `GET /who-can?verb=&group=&resource=&subresource=&namespace=&name=`,
`POST /review` (SubjectAccessReview → `{allowed, reason}`), `GET /graph?namespace=`.
UI: Who can…, subjects + effective permissions, risky roles, binding graph.

- Detect: always available (RBAC is core); every route is `read` and
  lists Roles, ClusterRoles, both kinds of bindings and ServiceAccounts
  cluster-wide per request (a credential without that access gets the
  API server's `403 kubernetes`).
- Evaluation follows the Kubernetes RBAC authorizer: `*` in verbs, API
  groups and resources (a plain resource never covers its subresources;
  `*/scale` covers every resource's `scale`), resourceNames (no wildcard;
  a rule with them never matches a request without a name), non-resource
  URLs (`*`, exact, trailing `*` prefix; granted only by
  ClusterRoleBindings), RoleBindings grant in their namespace only (so a
  request without a namespace sees ClusterRoleBindings only), a
  ServiceAccount subject without a namespace defaults to the binding's.
  Aggregated ClusterRoles are used with the rules the controller filled in.
- `who-can` takes `path=` instead of the resource fields for a
  non-resource URL and answers `{request, subjects: [{kind, name,
  namespace?, grants: [{binding, role, scope, rule}]}]}`; subjects are
  listed as the bindings name them (groups stay groups). A subject's
  detail answers `{subject, user, groups, missing?, bindings: [path],
  rules: [{namespace, rules: [rule + path]}]}` (`namespace` `""` =
  cluster-wide), where the identity includes the implicit groups
  (`system:serviceaccounts`, `system:serviceaccounts:<ns>`,
  `system:authenticated`) and a path's `via` names the group it reaches
  the subject through; `roleMissing` marks a binding to a missing role.
- `GET /risky` (read): roles with at least one risk (`full-access`,
  `escalate-bind`, `impersonate`, `wildcard-verbs`, `wildcard-resources`,
  `secrets-read`, `pod-exec`, `token-create`, `rbac-write`, `node-proxy`,
  `webhook-write`, `csr-approve`, `workload-create`,
  `persistentvolume-create`; severities `critical` | `high` | `medium`),
  with their bindings and the number of subjects, most severe first.
- `GET /graph?namespace=&system=true`: `{subjects, bindings, roles, edges,
  truncated}`; with a namespace its RoleBindings plus the
  ClusterRoleBindings naming one of its ServiceAccounts; `system:…`
  bindings only with `system=true`; at most 300 bindings.
- `POST /review` body `{subject: {kind, name, namespace?}, verb, group,
  resource, subresource, namespace, name}` or `{subject, verb, path}`;
  answers `{allowed, denied?, reason?, evaluationError?, user, groups,
  grants}` (`grants`: KubeBench's own evaluation). It is `read` although
  it creates a SubjectAccessReview (which changes nothing), sends the
  subject's user and groups in the review spec and never impersonates;
  without `create` on `subjectaccessreviews` it fails with `403 kubernetes`
  naming that permission. Rate limit: 30 reviews per user per minute,
  then `429 rate_limited` with `Retry-After`.

### 7.9 Topology plugin (frontend-only, new)

Builds a graph from core providers + generic lists: Ingress → Service →
Pods ← ReplicaSet/StatefulSet/DaemonSet ← Deployment; Pods → PVC → PV;
optional ConfigMap/Secret references. Layered SVG layout, pan/zoom, click
opens the resource. Resource action "Show topology" on Deployment,
StatefulSet, DaemonSet, Service, Ingress.

Implementation (`src/plugins/topology`, no backend routes):

- Data, per namespace, each list loaded on its own so the graph fills in
  as they arrive and an unreadable list (e.g. Secrets) leaves the rest:
  Pods, Deployments and Services through the `/api/k8s` passthrough
  (owner references, selectors and volumes intact); ReplicaSets,
  StatefulSets, DaemonSets, Ingresses, PVCs, PVs (cluster-wide, only those
  bound to the namespace's claims are shown), ConfigMaps and Secrets
  through the generic lists. Every list fails all → Unavailable.
- Edges: Ingress → Service (rule paths and default backend), Service →
  Pod (equality selector), controller ownerReference → ReplicaSet/Pod,
  Pod → PVC (also generic ephemeral volumes), PVC → PV (`volumeName`),
  Pod → ConfigMap/Secret (volumes incl. projected, `envFrom`, `env`
  `valueFrom`; not the `kube-api-access-*` token volume) and Ingress →
  TLS Secret. A reference whose list loaded without it is drawn as a
  missing node; optional references only when present.
- Layout: five columns (Deployment | ReplicaSet/StatefulSet/DaemonSet |
  Pod | Service/PVC/ConfigMap/Secret | Ingress/PV), Pods stacked in owner
  groups, every other node level with its placed neighbours; deterministic,
  so refreshes keep positions. Inactive ReplicaSets and config references
  are display options.
- Focus: the object, everything it leads to, and everything leading to it
  or its Pods (shared ConfigMaps do not pull in other workloads). Tabs:
  kind `topology`, props `{namespace, focus: {kind, name} | null}`, one per
  namespace and per focused object; objects open in `workloads-detail`
  (Pod, Deployment, Service), `generic-detail` or `yaml`.

### 7.10 Live updates (core)

`GET /api/watch?topics=pods,deployments,…` (read) — SSE stream of
`event: change` `data: {"topic":"pods","namespace":"default"}` (debounced),
heartbeat comments every 25 s, backed by per-cluster shared informers that
start lazily and stop when unused. Frontend: scoped invalidation
(`refreshData(topic?)`), status bar "Live" indicator, Settings toggle.

## 8. Distribution

- **Kubernetes**: Helm chart `charts/kubebench` (OCI `ghcr.io/shavell/charts/kubebench`
  and attached to releases) and rendered manifests
  `deploy/kubernetes/kubebench.yaml` (`kubectl apply -f`).
- **VM**: `.deb` (and `.rpm`) via nfpm/GoReleaser — binary `/usr/bin/kubebench`,
  systemd unit `kubebench.service`, config `/etc/kubebench/kubebench.env`,
  data `/var/lib/kubebench`, system user `kubebench`.
- **Installer**: `install.sh` — `curl -fsSL https://raw.githubusercontent.com/Shavell/kube-bench/main/install.sh | sudo bash`
  (online: downloads the release for the detected arch, verifies
  SHA-256) or `sudo bash install.sh --offline ./kubebench-offline-<ver>.tar.gz`.
- **Offline bundle** `kubebench-offline-<version>.tar.gz`: binaries,
  .deb/.rpm, container image tarballs (amd64/arm64), Helm chart `.tgz`,
  manifests, `install.sh`, checksums, README.
- **Container image** `ghcr.io/shavell/kube-bench:<version>` (multi-arch).
- **GitHub Actions**: `ci.yml`, `release.yml`, `docker-image.yml`,
  `helm-chart.yml`, `codeql.yml` — all `on: workflow_dispatch` only, with
  the push/PR triggers present but commented out.
- **Dependabot**: npm, gomod (`/backend`), github-actions, docker; weekly,
  grouped.
