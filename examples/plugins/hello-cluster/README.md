# Hello Cluster — example KubeBench plugin

An external (runtime-loaded) UI plugin for KubeBench, as described in
[`docs/design-v0.5.md` §6.8](../../../docs/design-v0.5.md). It adds an
activity bar entry, a side bar view, a command and a tab that lists the
namespaces of the active cluster through KubeBench's read-only Kubernetes
API. Its texts are available in English, Turkish and German.

Use it as the starting point for your own plugin.

## Build

Requires Node.js 20.19 or later (for Vite 8).

```bash
npm install
npm run build
```

`npm run build` type-checks the plugin and writes the installable folder
`dist/`:

```
dist/
├── plugin.json   # id, name, version, description, category, entry
└── index.js      # the plugin, one IIFE script
```

## Install

1. Copy `dist/` to the KubeBench plugins directory, in a folder named after
   the plugin id:

   ```bash
   sudo mkdir -p /var/lib/kubebench/plugins/hello-cluster
   sudo cp dist/* /var/lib/kubebench/plugins/hello-cluster/
   ```

   The plugins directory is `$KUBEBENCH_PLUGINS_DIR`, by default the
   `plugins` folder in `$KUBEBENCH_DATA_DIR` (`/var/lib/kubebench/plugins`
   for the .deb/.rpm packages). KubeBench reads it whenever the plugin list
   is requested, so no restart is needed.

2. Sign in as an administrator, open **Extensions** in the activity bar,
   select **Hello Cluster** under *External* and click **Enable**. External
   plugins stay disabled until an administrator enables them; enabling one
   loads it at once, and every user gets it at their next page load.

3. Click the globe icon in the activity bar, or run *Hello Cluster: Show
   Namespaces* from the command palette (F1).

If the script cannot be loaded, the Extensions view shows why (download
failed, timed out after 5 seconds, threw an error, or never registered).

## How it works

- **`plugin.json`** describes the plugin. `id` must match the folder name
  (lowercase letters, digits and `-`, at most 40 characters) and `entry`
  names the script. The build copies it into `dist/` and the script reads
  its id and version from it.
- **The script** is an IIFE (see `vite.config.ts`). It does not bundle React:
  `react` and `react/jsx-runtime` are mapped to `KubeBench.React` and
  `KubeBench.jsxRuntime`, so the plugin renders with KubeBench's own React.
- **`window.KubeBench`** is the plugin SDK. The script calls
  `KubeBench.registerPlugin(manifest)` once, synchronously, while it runs. The
  manifest has the same shape as the one of a built-in plugin: activity bar
  entry, side bar view, tab renderers, commands, panel tabs, status bar
  items, resource actions and settings sections.
- **Texts** are i18n keys in the plugin's namespace (its id). The bundles in
  `src/locales` are registered through the manifest's `locales`; components
  translate with `KubeBench.i18n.useTranslation(id)`, which follows language
  changes.
- **Data**: `KubeBench.api.k8sGet(path)` reads the Kubernetes API of the active
  cluster with the signed-in user's permissions (GET only; Secrets are not
  readable this way). `KubeBench.data.usePluginQuery` runs a request and
  re-runs it when the user refreshes; plugins with a server part call it
  through `KubeBench.api.pluginApi(id)`.
- **UI building blocks** come from `KubeBench.ui` (Table, Badge, EmptyState,
  Toolbar, Dialog, ConfirmDialog, CodeEditor, notify). Colours should only use
  the `--vscode-*` CSS variables, so the plugin follows the light and dark
  themes.

The SDK types are declared in
[`src/core/plugin/sdk/kubebench.d.ts`](../../../src/core/plugin/sdk/kubebench.d.ts);
`tsconfig.json` maps the module name `@kubebench/sdk` to that file.

## Start your own plugin

1. Copy this folder out of the repository.
2. Copy `src/core/plugin/sdk/kubebench.d.ts` into it and point the
   `@kubebench/sdk` entry of `tsconfig.json` at the copy.
3. Change `id`, `name` and `description` in `plugin.json` and the texts in
   `src/locales`.
4. Build and install as above.

External plugins run in the page with the signed-in user's session: they
can do whatever that user can do. Only install plugins you trust.
