# KubeBench

A VSCode-style dashboard & workbench for Kubernetes. The UI is written entirely
against typed provider interfaces; it runs **live** against a real cluster when
the local proxy is up, and falls back to a deterministic **mock** cluster when
it isn't (the status bar shows which mode you're in).

```bash
npm install
npm run dev                                  # http://localhost:5173 (mock mode)

# live mode: start the proxy with a kubeconfig, then (re)load the app
KUBECONFIG=~/path/to/kubeconfig npm run server   # http://localhost:9876
```

The proxy (`server/`) reads a token-based kubeconfig and translates the app's
provider API to the Kubernetes API: resource lists, object YAML, events, pod
logs, metrics-server sampling (15s ring buffer for utilization history), and
mutations (delete pod, scale/restart deployment). RBAC errors surface in the UI
as inline banners. With the current read-mostly credentials, Helm and CRD
modules and network/fs metrics remain mock.

## Layout

VSCode Dark+ chrome: activity bar → per-module sidebar → tabbed editor area →
bottom panel (Problems / Logs / Terminal) → status bar (cluster context +
namespace filter on the left, module contributions on the right).

Navigation is tab-based like VSCode (no router): opening a resource opens a tab;
reopening it focuses the existing tab.

Command palette: `Cmd/Ctrl+Shift+P` or `F1`. Core `View:` commands are generated
from registered plugins; each plugin can contribute its own commands via the
`commands` field of its manifest.

## Architecture

```
src/
├─ core/
│  ├─ shell/      Shell, ActivityBar, SideBar, EditorArea+TabStrip, BottomPanel, StatusBar
│  ├─ plugin/     PluginManifest types + PluginRegistry
│  ├─ store/      layoutStore (tabs/panels), contextStore (cluster ctx + namespace)
│  └─ theme/      theme.css — all VSCode Dark+ tokens (single source of truth)
├─ shared/        UI kit: Table, Tree, Badge, Icon, Toolbar, ResizeHandle, charts/
├─ data/
│  ├─ types.ts       K8s-shaped domain types
│  ├─ providers.ts   Provider interfaces — the future real-API contract
│  ├─ DataContext    useProviders() / useProviderQuery() hooks
│  └─ mock/          seeded, deterministic fake cluster + YAML + metrics
└─ plugins/       one folder per module, wired only in plugins/index.ts
   ├─ workloads/  nodes, namespaces, pods, deployments, services
   ├─ crd/        CRD discovery + custom resource lists
   ├─ helm/       releases, revision history, values editor, repos
   ├─ metrics/    cluster overview + per-node charts, status bar CPU/Mem
   └─ terminal/   bottom-panel tabs + the shared Monaco "yaml" tab renderer
```

### Plugin model

Everything a module contributes is declared in one `PluginManifest`
(`src/core/plugin/types.ts`): activity bar entry, sidebar view, editor tab
renderers, bottom panel tabs, status bar items. The shell composes manifests from
the registry and never imports plugin internals. Cross-plugin actions go through
tab *kinds* — e.g. any plugin opens a YAML view with
`openTab({ kind: "yaml", props: { text } })`; the renderer for `yaml` happens to
be registered by the terminal plugin.

**Pluggability check:** comment out any entry in `src/plugins/index.ts` — its
icon, sidebar, tabs, panels, and status items all vanish with no other changes.

### Swapping mock for real data

`src/data/providers.ts` defines `ClusterProvider`, `CrdProvider`, `HelmProvider`
and `MetricsProvider` (all async). The mocks implement them in `src/data/mock/`.
When the backend lands, implement the same interfaces (e.g. `src/data/api/`) and
change one line in `main.tsx`:

```ts
const providers = createApiProviders(baseUrl) // was: createMockProviders()
```

Mock data is seeded (mulberry32) so lists and charts are identical across
reloads. Simulated 150–300 ms latency keeps loading states honest.

## Roadmap

- v1.1: URL sync, tab persistence
- v2: real cluster connectivity behind the provider interfaces, Electron/Tauri
  wrapper if a desktop build is wanted
