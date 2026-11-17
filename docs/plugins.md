# Plugin development

Almost everything in KubeBench is a plugin: Workloads, Helm, Argo CD,
Storage, Security, Access, Topology, Notes, Metrics, CRDs, Clusters, the editor,
the terminal, Settings, Extensions, Users and Audit. There are three kinds:

| Kind | Lives in | Ships | Use it for |
|---|---|---|---|
| **Core (built-in) UI plugin** | `src/plugins/<id>/` | in the KubeBench build | views on the core API or on a backend plugin |
| **Backend plugin** | `backend/internal/plugins/<id>/` | in the KubeBench binary | server-side logic and new routes under `/api/plugins/<id>/` |
| **External UI plugin** | `$KUBEBENCH_PLUGINS_DIR/<id>/` | separately, loaded at runtime | views of your own without forking KubeBench |

A feature with server-side logic is usually a pair: a backend plugin and a
core UI plugin with the same id that names it in `backendPlugin`.

Admins enable and disable plugins in **Extensions**. Core plugins marked
`core: true` (workloads, clusters, settings, terminal, editor, extensions,
users, audit) cannot be disabled. A UI plugin whose backend plugin is
disabled, or whose `Detect` reports it unavailable on the active cluster
(for example "Argo CD CRDs not found"), is hidden and Extensions shows the
reason.

Read [`architecture.md`](architecture.md) first; the contracts are §5
(backend) and §6.4–§6.8 (frontend) of [`design-v0.5.md`](design-v0.5.md).

## Core UI plugin

1. Create `src/plugins/<id>/manifest.tsx` exporting a `PluginManifest`
   ([`src/core/plugin/types.ts`](../src/core/plugin/types.ts)):

   ```ts
   export const demoPlugin: PluginManifest = {
     id: 'demo',                      // also the i18n namespace
     name: 'manifest.name',           // display strings are i18n keys
     description: 'manifest.description',
     category: 'tools',
     backendPlugin: 'demo',           // optional: hide while the backend plugin is unavailable
     locales: localeBundles(import.meta.glob('./locales/*.json', { eager: true, import: 'default' })),
     activityBar: { icon: 'beaker', tooltip: 'manifest.tooltip' },   // codicon name
     sidebar: { id: 'demo', title: 'manifest.sidebarTitle', component: DemoSidebar },
     tabRenderers: [{ kind: 'demo-page', component: lazyComponent(() => import('./DemoPage').then((m) => m.DemoPage)) }],
     commands: [{ id: 'demo.open', title: 'commands.open', icon: 'beaker', run: openDemo }],
   }
   ```

   Other contributions: `bottomPanelTabs` (with request `kinds` for
   `showInPanel`), `statusBarItems`, `resourceActions` (added to row menus
   and detail toolbars of other views, with a `permission`),
   `resourceKinds` (kubectl-style `get`/`describe` in the palette),
   `settingsSections`, and `keybinding` on commands.
2. Add `locales/en.json`, `tr.json` and `de.json` (every language in
   `src/i18n/locales`); see [i18n.md](i18n.md).
3. Register the manifest in [`src/plugins/index.ts`](../src/plugins/index.ts)
   (or in `adminPlugins` for views only global admins get).

Rules:

- **No imports** from `core/shell` or other plugins. Open other views by tab
  kind with `useLayoutStore.getState().openTab({...})` (stable kinds:
  `yaml` with `{ref}` or `{text, readOnly}`, `new-resource`,
  `workloads-detail`, `generic-detail`, `crd-instances`, `topology`,
  `helm-release-detail`, …) or helpers in `core/plugin/editorTabs`; open
  bottom panel views with `showInPanel('pod-logs' | 'pod-exec', {namespace,
  pod, container})`, and a tab next to the editor area (the side editor
  group, which keeps its tabs mounted) with `openInSideGroup(tab)` from
  `core/plugin/sideGroup` (the service browser uses kind `service-browser`
  with `{kind: 'services' | 'pods', namespace, name}`).
- **Data** through `src/data/providers.ts` (`useProviderQuery`) or your own
  plugin data (below). Declare live-update topics on queries
  (`{ topics: ['pods'], namespace }`) and call `refreshData()` after a
  mutation.
- **Permissions**: gate mutation UI with `usePermission('write')` /
  `<Can perm="write">` (shell, debug and Secret reveal: `exec`; the service
  browser: `browse`, which a read-only cluster does not take away); show it
  disabled with the reason as tooltip, or hide it. Destructive actions go
  through `ConfirmDialog`, which asks for the name on production clusters.
- **Unavailable**: in live mode, show `<Unavailable>` with the reason when a
  capability is missing; never mock data.
- **Styling**: `--vscode-*` tokens only; charts single-series.
- Load tab pages lazily with `lazyComponent()` to keep the main bundle small.

### Plugin-owned data

A plugin keeps its own data capability in `src/plugins/<id>/data/`:

```
data/types.ts      shapes, matching the backend plugin's JSON
data/provider.ts   the interface
data/mock.ts       deterministic implementation (seeded RNG, no Date.now()/Math.random())
data/live.ts       implementation on pluginApi('<id>')  →  /api/plugins/<id>/...
data/index.ts      export const demoData = definePluginData('demo', { mock, live })
```

`definePluginData` picks the implementation by the boot data mode and never
falls back. Components call `usePluginQuery(() => demoData().list(ns), [ns],
{ topics: ['pods'], namespace: ns })` and `usePluginStatus('demo')`.

## Backend plugin

1. Create `backend/internal/plugins/<id>/` with a type that implements
   `plugin.Plugin` ([`backend/internal/plugin`](../backend/internal/plugin/plugin.go))
   and registers itself:

   ```go
   package demo

   func init() { plugin.Register(&Plugin{}) }

   type Plugin struct{}

   func (*Plugin) Manifest() plugin.Manifest {
       return plugin.Manifest{ID: "demo", Name: "Demo", Version: "0.5.0",
           Description: "…", Category: "tools"}
   }

   func (*Plugin) Routes(r plugin.Router) {
       r.Handle(http.MethodGet, "/things", plugin.AccessRead, listThings)
       r.Handle(http.MethodPost, "/things", plugin.AccessWrite|plugin.DryRunnable, createThing)
   }

   // Detect says whether the plugin works on a cluster; cached 60 s per cluster.
   func (*Plugin) Detect(ctx context.Context, rt *kube.Runtime) plugin.Status {
       return plugin.Status{Available: true}
   }

   func listThings(w http.ResponseWriter, r *http.Request, c *plugin.Context) {
       // c.Runtime is the caller's active cluster, c.User the caller.
       plugin.WriteJSON(w, []string{})
   }
   ```

2. Add a blank import to
   [`backend/internal/plugins/all/all.go`](../backend/internal/plugins/all/all.go).
   The routes are now served under `/api/plugins/demo/…`.
3. Write tests with fake clients (`kube.NewRuntimeForTest(cluster, cs, dyn)`,
   `k8s.io/client-go/kubernetes/fake`, `dynamic/fake`), including the access
   level of every route.

The contract:

- **Access levels** `AccessRead`, `AccessWrite`, `AccessExec`, `AccessAdmin`;
  `DryRunnable` on a write/exec route lets `dryRun=true` requests through as
  `read` (unaudited), so the handler **must** then perform a real dry run
  (`plugin.IsDryRun(r)`). The router enforces sessions, roles, read-only
  clusters, plugin enablement and `Detect` before your handler runs.
  `AccessContribute` (v0.6) is for KubeBench-side data that every user of
  the cluster sees (shared notes): operator or admin, also on a read-only
  cluster, mutations audited. `c.Role` is the caller's role on the active
  cluster.
- **Audit**: write/exec/admin requests are audited automatically. Refine the
  row once the body is read with `c.SetAudit(target, detail)` (never
  secrets); add extra rows with `c.Audit(action, target, detail)`.
- **Errors**: `plugin.WriteError(w, status, code, err)` with the codes of
  [api.md](api.md#errors), and `plugin.WriteK8sError(w, err)` for errors from
  the cluster.
- **Storage**: `c.KV` is a key/value store scoped to the plugin (SQLite);
  prefix per-cluster keys with `cluster/<clusterID>/`. A plugin that needs
  files or background work implements `plugin.Starter`; `Start` gets
  `Host.DataDir` = `$KUBEBENCH_DATA_DIR/<id>` (mode `0700`) and a context
  that ends at shutdown. A plugin whose data needs queries, counts or
  cascades with users and clusters gets tables of its own through a store
  migration and reaches them through `c.Store` (the notes plugin).
- **Kubernetes access**: use `Runtime.CS`, `Runtime.Dyn`,
  `Runtime.ResolveKind`/`ResolveResource` (prefer them over `Mapper()`; they
  re-discover once when a CRD appeared) and attribute changes to the field
  manager `kubebench`. Never return Secret values; redact them as the core
  read routes do.

## External UI plugin

External plugins are IIFE scripts loaded at runtime from
`$KUBEBENCH_PLUGINS_DIR/<id>/` (default `$KUBEBENCH_DATA_DIR/plugins`). They
are disabled until an admin enables them in Extensions, and then run in the
page with the signed-in user's session: install only plugins you trust.

Start from the example
[`examples/plugins/hello-cluster`](../examples/plugins/hello-cluster/README.md):

```
hello-cluster/
├── plugin.json          {"id","name","version","description","category","entry":"index.js"}
├── src/                 the manifest, components, locales/{en,tr,de}.json
├── vite.config.ts       library build to one IIFE; react → KubeBench.React
└── tsconfig.json        maps @kubebench/sdk to the SDK types
```

```bash
cd examples/plugins/hello-cluster && npm install && npm run build
sudo mkdir -p /var/lib/kubebench/plugins/hello-cluster
sudo cp dist/* /var/lib/kubebench/plugins/hello-cluster/
```

The folder name must equal `id` (`^[a-z0-9][a-z0-9-]{0,39}$`). The server
serves only `.js`, `.mjs`, `.css`, `.json`, `.map`, `.svg` and `.png` files
from it, without directory listings or symlink escapes. At boot the app
loads each enabled plugin's `entry` (5 s timeout); the script must call
`KubeBench.registerPlugin(manifest)` once, synchronously. Extensions reports
plugins that failed to load. An external plugin whose id belongs to a
built-in plugin is never loaded.

### SDK reference

`window.KubeBench` is installed before any plugin script runs. Its types are
in [`src/core/plugin/sdk/kubebench.d.ts`](../src/core/plugin/sdk/kubebench.d.ts)
(type-checked against the implementation in `src/core/plugin/globalSdk.ts`):

| Member | What it is |
|---|---|
| `apiVersion` (`1`), `version` | SDK and KubeBench versions |
| `React`, `jsxRuntime` | KubeBench's React; do not bundle your own |
| `registerPlugin(manifest)` | registers the plugin (same manifest shape as built-in plugins; never `core`) |
| `i18n.t`, `i18n.registerTranslations(ns, bundles)`, `i18n.useTranslation(ns)` | translations; the manifest's `locales` registers the plugin's namespace |
| `api.apiFetch(path, init)` | JSON request to `/api<path>`; rejects with `ApiError {status, code}` |
| `api.pluginApi(id)` | `get/post/put/patch/del` under `/api/plugins/<id>` |
| `api.k8sGet(path, params)` | read-only Kubernetes API through `/api/k8s` (no Secrets, no watch) |
| `data.usePluginQuery(fn, deps, scope?)`, `data.refreshData()`, `data.refreshTopics(changes)` | queries that re-run on refresh and on live updates of their topics |
| `layout.openTab(tab)` | open a tab of any registered kind |
| `permissions.usePermissions()` | `{role, canRead, canWrite, canExec, isAdmin, readOnly}` |
| `ui.Table`, `Badge`, `EmptyState`, `Toolbar`, `Dialog`, `ConfirmDialog`, `CodeEditor`, `notify` | KubeBench's UI building blocks |

External plugins have no server part of their own; they read through
`k8sGet`, the core API and the routes of built-in backend plugins.
