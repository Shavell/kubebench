# Architecture

KubeBench is one Go binary that serves a React single-page app and a JSON
API, keeps its state in SQLite and talks to Kubernetes clusters with
client-go and the Helm SDK. The browser never sees cluster credentials:
every request goes through the server, which applies KubeBench's own roles
before the cluster's RBAC.

```
 Browser ─────────────────────────────────────────────────────────────────
  core/shell     activity bar · sidebar · tabbed editor · bottom panel ·
                 status bar · command palette            (knows manifests only)
  plugins/*      workloads, editor, terminal, helm, argocd, storage, security,
                 access, topology, metrics, crd, clusters, extensions,
                 settings, users, audit   (+ external plugins via window.KubeBench)
  data/          providers.ts interfaces ─┬─ data/mock (deterministic demo)
                                          └─ data/api  (fetch, SSE, WebSocket)
 ─────────────────────────── HTTP /api, SSE, WebSocket ────────────────────
 kubebench (Go)
  internal/api      router: session → role → read-only → audit → handler
  internal/plugin   backend plugin contract; internal/plugins/<id> implement it
  internal/kube     per-cluster Runtime: clients, probe, sampler, Prometheus,
                    apply/edit, operations, terminals, log streams
  internal/watch    per-cluster informer hub for live updates
  internal/store    SQLite: users, sessions, clusters, assignments, audit, KV
  internal/updates  GitHub release check     internal/cloud(auth)  EKS/AKS/GKE
  internal/web      go:embed of the built frontend
 ──────────────────────────────────────────────────────────────────────────
   Kubernetes API servers (in-cluster ServiceAccount, stored kubeconfigs)
```

The implementation contract is [`design-v0.5.md`](design-v0.5.md); section
numbers below (§n) refer to it. The HTTP API is listed in
[`api.md`](api.md), plugin development in [`plugins.md`](plugins.md).

## Backend (`backend/`)

### Start-up (`main.go`)

`main.go` parses the configuration (`internal/config`, [configuration.md](configuration.md)),
answers `--version` and the `healthcheck` command, opens the store
(migrating it forward), creates the bootstrap admin and seeds the inventory
on a new database (`bootstrap.go`), starts the backend plugins
(`plugin.StartAll`), builds the API server and serves HTTP or HTTPS. Two
background jobs (`jobs.go`) prune expired sessions hourly and old audit
rows daily. On SIGINT/SIGTERM it shuts down gracefully: the API server
first hangs up the terminals and ends the log and watch streams with the
reason (`api.Server.Shutdown`), then the HTTP server lets the remaining
requests finish.

### Packages

| Package | Responsibility |
|---|---|
| `internal/config` | Reads and validates every `PORT`/`KUBEBENCH_*` variable once; reports all problems together |
| `internal/store` | SQLite (modernc, no cgo), file mode `0600`, `foreign_keys` on. Users, sessions, clusters (kubeconfig, flags, cloud credentials), per-cluster assignments, audit log, plugin settings and plugin KV, app settings. Schema migrations by `PRAGMA user_version`; a newer schema is refused |
| `internal/password` | The password policy (8 characters to 72 bytes) and bcrypt |
| `internal/kube` | `Manager` caches one `Runtime` per cluster and rebuilds it when the stored record changes (an edit closes it at once, a deleted cluster's id never gets one again; runtimes are closed in the background, outside the manager's lock). A `Runtime` holds the typed, dynamic and metrics clients (redirects never followed), a cached RESTMapper (`ResolveKind`, `ResolveResource`), the capability probe, the metrics-server sampler, Prometheus discovery, provider detection, the watch hub, and the operations: lists, YAML (with Secret redaction and "neat"), server-side apply and merge-patch edits (field manager `kubebench`), deletes, node/workload/pod/CronJob/namespace operations, drain, debug containers, terminals and log streams |
| `internal/watch` | The live-update hub of one Runtime: metadata-only dynamic informers per resource, started on the first subscriber and stopped 5 min after the last, with per-cluster and per-user limits (§7.10) |
| `internal/api` | The HTTP layer: route table, router, auth, handlers, SSE and WebSocket helpers, Origin check, login rate limit, audit writer, external plugin files, the `/api/k8s` passthrough |
| `internal/plugin` | The backend plugin contract (`Plugin`, `Router`, `Context`, `KV`, access levels, `Starter`) and shared response helpers |
| `internal/plugins/<id>` | The built-in backend plugins `helm`, `argocd`, `storage`, `security`, `access`; linked by blank imports in `internal/plugins/all` |
| `internal/updates` | The update check against GitHub releases (cached 6 h, ETag revalidation, backoff) and SemVer handling of the build version (§7.11) |
| `internal/cloud` | Provider detection (EKS, AKS, GKE, OpenShift, DOKS, LKE, OVH, k3s, RKE2, Kubernetes) and the bundled version support tables (§7.12) |
| `internal/cloudauth` | Recognises the EKS, GKE and AKS exec stanzas in kubeconfigs, removes them and signs requests in process with AWS, Google or Azure credentials (per-cluster or the server's own identity) |
| `internal/web` | Serves the embedded frontend (`go:embed all:dist`), falling back to `index.html` for client-side routes |

### Router and access levels

Every core route is registered once in
[`internal/api/routes.go`](../backend/internal/api/routes.go) with an access
level: `public`, `authed`, `read`, `write`, `exec` or `admin`
(`allowDryRun` can be combined with `write`). Plugin routes declare theirs
through `plugin.Router` and are mounted under `/api/plugins/<id>/`. The
router (`router.go`, `pluginrouter.go`) runs the same gates for both, in a
fixed order, before any handler or cluster call:

1. session (cookie) and `password_change_required`;
2. `admin_required` for admin routes;
3. `plugin_disabled` for plugin routes;
4. the caller's **active cluster** (per user) and its Runtime → `no_cluster`;
5. `cluster_read_only` for write/exec (admins included);
6. the caller's role on that cluster (`viewer`, `operator`, admin) → `forbidden_role`;
7. the plugin's cached `Detect` → `plugin_unavailable`.

Handlers receive the result as a `call` (`call.rt` is the Runtime). Write,
exec and admin mutations are audited by the router: action, target (from
the route and its parameters, refined by handlers through `setAudit` /
`Context.SetAudit`), status and a short detail; never request bodies.
Dry runs on routes that opt in (`allowDryRun`, `plugin.DryRunnable`) are
checked as `read` and not audited. Long-lived streams (terminals, log
streams, watch streams) are re-authorized at every heartbeat.

### Live updates

`GET /api/watch` subscribes to topics (`pods`, `deployments`,
`applications.argoproj.io`, …) in the active cluster's hub. The hub runs
one shared informer per resource for all users, keeps only object
metadata, batches change notifications for 500 ms and reports per topic
whether it is `live` or `polling` (and why). Lists are still read from the
API server; the stream only says what to re-read.

### Updates

`internal/updates` reads GitHub's release list lazily, when an admin's UI
asks (`GET /api/updates`), never at start-up and never in `off` mode. The
mode set in Settings is stored in `app_settings`; each user's last seen
version (for What's new) in `users.last_seen_version`.

### Cloud credentials

When a stored kubeconfig's user runs `aws eks get-token`,
`aws-iam-authenticator`, `gke-gcloud-auth-plugin` or `kubelogin`,
`cloudauth` replaces the exec stanza with a transport that adds a bearer
token it obtains itself: from per-cluster credentials stored with the
cluster (write-only), or from the server's ambient identity (IRSA, Pod
Identity, instance profiles, GKE/Azure workload identity, environment).
Tokens are cached until a minute before expiry. Any other exec plugin stays
behind `KUBEBENCH_ALLOW_KUBECONFIG_EXEC`. See [security.md](security.md).

## Frontend (`src/`)

React 19, TypeScript, Vite, zustand, recharts, xterm.js, Monaco (bundled
locally, so offline installs work) and i18next.

### Layers

| Directory | Contents | May import |
|---|---|---|
| `src/core/shell` | The VS Code-like chrome: activity bar, sidebar, editor area with tabs, bottom panel, status bar (cluster, environment, read-only, namespace, live, user), command palette, login and password screens, boot error | `core/*`, `data`, `shared`, `i18n` — never `plugins/*` |
| `src/core/plugin` | `PluginManifest` types, the registry, plugin status (`/api/plugins`), resource actions, `editorTabs`, `panelRequests` (`showInPanel`), lazy tab pages, the external plugin loader and the `window.KubeBench` SDK | |
| `src/core/*` | `boot` (mock policy, session, `startApp`), `permissions.ts` (`usePermissions`, `usePermission`, `<Can>`), `live` (live-update stream and status), `palette` (kubectl-style commands), `shortcuts`, `updates` (update check, What's new, changelog tab), `session`, `store` (layout, settings, namespace favourites, aliases), `theme` | |
| `src/plugins/<id>` | One folder per built-in plugin with `manifest.tsx`, its views, `locales/` and, for plugin-owned data, `data/` | `core/plugin`, `core/store`, `core/permissions`, `data`, `shared`, `i18n` — never `core/shell` or another plugin |
| `src/data` | Domain types (`types.ts`), provider interfaces (`providers.ts`), the API client (`apiClient.ts`), `api/` (live) and `mock/` (demo) implementations, plugin data (`pluginData.ts`), the invalidation bus and `DataContext` | |
| `src/shared` | UI kit: Table (windowed above 300 rows, checkbox selection), Dialog, ConfirmDialog (type-to-confirm), form controls, Tabs, Toolbar, Badge, EmptyState, Unavailable, notify/Toaster, CodeEditor/DiffEditor, single-series charts, provider badges | |
| `src/i18n` | i18next set-up, language detection, formatting helpers, the `core` namespace bundles | |

### Plugins and the registry

Every module contributes through one `PluginManifest` (activity bar entry,
sidebar view, tab renderers by kind, bottom panel tabs, status bar items,
commands with optional keybindings, resource actions, resource kinds for
the palette, settings sections, translation bundles). Built-in manifests are
listed in [`src/plugins/index.ts`](../src/plugins/index.ts); `users` and
`audit` are registered at boot for global admins only, and the core
`workbench` and `updates` plugins are created by `core/boot/startApp.tsx`.
The registry hides contributions of plugins that are disabled or whose
backend plugin is unavailable on the active cluster.

Plugins never import each other. They open each other's views through tab
kinds (`layoutStore.openTab({kind: 'yaml', props: {ref}})`,
`workloads-detail`, `generic-detail`, `helm-release-detail`, `topology`, …)
and bottom panel requests (`showInPanel('pod-logs', {namespace, pod,
container})`).

### Data

Components read data only through the providers of `src/data/providers.ts`
(`useProviderQuery`) or plugin-owned data (`definePluginData`,
`usePluginQuery`). Each capability has a live implementation in
`data/api` (or `src/plugins/<id>/data/live.ts`) and a deterministic mock
(seeded RNG, fixed clock). Live mode never falls back to mock data: a
capability the server reports missing rejects with `capability_unavailable`
and the view shows Unavailable with the reason.

Boot (`core/boot`) reads `/api/info`. Mock mode is used only by the Vite dev
server when the backend is down, with `?demo`, or in a `VITE_DEMO_MODE=true`
build; a production build whose backend is unreachable shows a boot error.

Queries declare the topics they read (`scope: {topics, namespace}`).
Mutations call `refreshData()`, which re-runs every query; the live-update
stream calls `refreshTopics(changes)`, which re-runs only the affected ones.
The stream subscribes to the union of the mounted queries' topics.

### i18n and themes

Strings are i18next keys in namespaces: `core` (`src/i18n/locales`),
`platform` (`src/core/locales`), `updates` (`src/core/updates/locales`) and
one per plugin (`src/plugins/<id>/locales`). English, Turkish and German
ship; see [i18n.md](i18n.md). Colours come only from the `--vscode-*`
tokens in `src/core/theme/theme.css`, defined for the dark and light
themes; Settings offers dark, light and system.

## Build and distribution

`npm run build` writes `dist/`; `make build` copies it into
`backend/internal/web/dist/` (which holds only `.gitkeep` in git) and
builds `bin/kubebench`. The Dockerfile does the same in a multi-stage
build on `distroless/static-debian13`. GoReleaser builds the release
archives and `.deb`/`.rpm` packages; the Helm chart and the rendered
manifests live in `charts/kubebench` and `deploy/kubernetes`. See
[deployment.md](deployment.md) and [releasing.md](releasing.md).
