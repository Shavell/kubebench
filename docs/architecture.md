# Architecture

KubeBench is three layers with strict boundaries:

```
┌────────────────────────────────────────────────────────────┐
│ core/shell        VSCode chrome: ActivityBar, SideBar,     │
│                   EditorArea+tabs, BottomPanel, StatusBar, │
│                   CommandPalette, ContextMenuHost          │
│   knows only the PluginManifest shape                      │
├────────────────────────────────────────────────────────────┤
│ plugins/*         clusters, workloads, crd, helm, metrics, │
│                   terminal, settings                       │
│   import shared/ + data/ + core/plugin types & stores      │
│   never import core/shell or each other                    │
├────────────────────────────────────────────────────────────┤
│ data/             typed domain types + provider interfaces │
│   mock/  – deterministic fake cluster (seeded RNG)         │
│   api/   – fetch client for the local proxy (server/)      │
└────────────────────────────────────────────────────────────┘
                server/  – Node proxy → Kubernetes API
```

## Core shell (`src/core`)

- `shell/Shell.tsx` — root layout (flex): activity bar | sidebar | main
  (editor + bottom panel) with a status bar underneath. Global keybindings
  (palette) and settings side-effects (terminal font CSS var, auto-refresh
  interval) live here.
- `store/layoutStore.ts` — the navigation model: active plugin, sidebar/panel
  geometry, and the tab list (`openTab` dedupes by id, `closeTab`,
  `closeOtherTabs`, `closeTabsToRight`, `closeAllTabs`).
- `store/contextStore.ts` — active cluster context + global namespace filter.
- `store/settingsStore.ts` — persisted user preferences (zustand persist).
- `theme/theme.css` — every VSCode Dark+ color token as a CSS variable; all
  components consume only these variables.

## Plugin API (`src/core/plugin`)

A module contributes everything through one `PluginManifest`:

```ts
interface PluginManifest {
  id: string
  name: string
  activityBar?: { icon; tooltip; order }     // omit → no icon (terminal, settings)
  sidebar?: { id; title; component }
  tabRenderers?: { kind; component }[]       // render tabs by kind
  bottomPanelTabs?: { id; title; component }[]
  statusBarItems?: { id; alignment; priority; component }[]
  commands?: { id; title; icon; run }[]      // command palette entries
}
```

Manifests are registered once in `src/plugins/index.ts`; the registry
(`core/plugin/registry.ts`) is the only coupling point. **Pluggability check:**
comment a plugin out of that array and every trace of it disappears.

Tabs are plain serializable descriptors (`{id, title, icon, kind, props}`).
Cross-plugin flows go through tab *kinds*, not imports — e.g. any plugin opens
a YAML view with `openTab({kind: 'yaml', props: {text}})`; the `yaml`,
`pod-logs` and `pod-exec` renderers happen to be registered by the terminal
plugin.

## Data layer (`src/data`)

`providers.ts` is the single contract between UI and data:

- `ClusterProvider` — lists, YAML, events, logs, mutations (delete/scale/restart)
- `CrdProvider`
- `MetricsProvider` — overview, allocation, series (cpu/memory/network/fs),
  source discovery
- `InventoryProvider` — cluster inventory CRUD + activate

Plugin-owned capabilities (e.g. Helm, `src/plugins/helm/data/`) use
`definePluginData` from `src/data/pluginData.ts` instead.

Everything is async. Components use `useProviderQuery(fn, deps)`
(`DataContext.tsx`) which also subscribes to a global invalidation bus:
mutations call `refreshData()` and every mounted query refetches. Swappable
for TanStack Query later without touching plugins.

Bootstrap (`main.tsx`) probes `/api/info` with a short timeout and picks
`createApiProviders()` (live) or `createMockProviders()` (mock);
`data/mode.ts` records the choice for display.

### Mock data (`data/mock`)

Deterministic by design: a seeded RNG (mulberry32) and a fixed clock, so
lists, YAML and chart series are identical across reloads. Mutations mutate
the in-memory arrays, so kill/scale/rollback behave realistically until the
page reloads.

## Charts (`src/shared/charts`)

Single-series area charts only (small multiples instead of legends), colored
from the `--vscode-charts-*` tokens; `UsageBar` renders allocation with an
overcommit marker beyond 100%. Chart colors were validated for contrast and
CVD-safety against the dark surface.

## Adding a module

1. `src/plugins/<name>/manifest.tsx` exporting a `PluginManifest`.
2. Sidebar/page components that read data only via `useProviderQuery`.
3. Need new data? Extend the interfaces in `data/providers.ts`, implement in
   `data/mock` **and** `data/api` (+ a proxy endpoint in `server/`).
4. Register the manifest in `src/plugins/index.ts`. Done — icon, sidebar,
   tabs, commands all appear.

## Backend proxy (`server/`)

See [`backend.md`](backend.md). In short: a dependency-light Node HTTP server
that reads a token-based kubeconfig, translates the provider API to the
Kubernetes API, samples metrics-server into a ring buffer, discovers
Prometheus through `services/proxy`, and persists the cluster inventory.
