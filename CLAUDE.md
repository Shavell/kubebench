# KubeBench — notes for AI assistants

VSCode-style Kubernetes workbench. **Frontend**: React 19 + TS + Vite,
zustand, recharts, xterm.js, Monaco (bundled locally), i18next. **Backend**:
single Go binary in `backend/` (client-go, Helm v3 SDK, modernc SQLite) that
embeds the built frontend; ships as image, Helm chart (`charts/kubebench`),
manifests, `.deb`/`.rpm` and `install.sh`. Contract: `docs/design-v0.5.md`
(when code and contract disagree, fix one in the same change). Docs:
`docs/architecture.md`, `docs/api.md` (every route), `docs/plugins.md`,
`docs/configuration.md`, `docs/security.md`, `docs/i18n.md`,
`docs/deployment.md`, `docs/releasing.md`; `docs/design-go-backend.md` is
historical. Status and history: `docs/dev/v0.5-handoff.md`.

Rules that matter here:

- **Frontend layering**: `core/shell` must not import from `plugins/*`;
  plugins must not import each other or `core/shell`. Cross-plugin flows go
  through tab kinds via `layoutStore.openTab` (bottom panel views: kinds via
  `showInPanel` in `core/plugin/panelRequests.ts`). New modules register a
  single `PluginManifest` in `src/plugins/index.ts`; the admin-only plugins
  (`users`, `audit`) and the core `workbench`/`updates` plugins are
  registered at boot (`core/boot/startApp.tsx`). Load tab pages with
  `lazyComponent()`.
- **Data access** only through the provider interfaces in
  `src/data/providers.ts` (plugin-owned data: `definePluginData` in
  `src/data/pluginData.ts`, files in `src/plugins/<id>/data/`). Every
  capability needs BOTH implementations: mock (deterministic — seeded RNG,
  no `Date.now()`/`Math.random()`) and live (the Go backend). Live mode never
  falls back to mock data: a missing capability rejects with
  `capability_unavailable` and the UI shows `<Unavailable>` with the reason.
- After mutations call `refreshData()`; queries reading cluster objects
  declare live-update topics (`scope: {topics, namespace}`, §7.10).
- **i18n**: every user-visible string goes through i18next in every
  language (`en`, `tr`, `de`; the set is the files in `src/i18n/locales`):
  core strings in `src/i18n/locales`, platform strings in `src/core/locales`,
  updates in `src/core/updates/locales`, plugin strings in
  `src/plugins/<id>/locales` (namespace = plugin id). Manifest display
  strings are keys. Kubernetes nouns stay untranslated (Turkish apostrophe
  suffixes, "siz"; German "Sie"). `npm run i18n:check` must pass.
- **Permissions**: gate mutation UI with `usePermission`/`<Can>` from
  `src/core/permissions.ts` (`write` for changes, `exec` for shell, debug,
  Secret reveal, Helm values; disabled with the reason as tooltip, or
  hidden); destructive actions ask through `ConfirmDialog` (type-to-confirm
  on production clusters).
- **Backend layering**: `internal/config`, `internal/store` (SQLite),
  `internal/kube` (per-cluster Runtime), `internal/watch` (live updates),
  `internal/updates`, `internal/cloud` + `internal/cloudauth` (EKS/AKS/GKE),
  `internal/api` (route table, router, handlers), `internal/web` (go:embed).
  The active cluster is **per-user**. Every route is registered once in
  `internal/api/routes.go` with an access level (public/authed/read/write/
  exec/admin, `allowDryRun` only with a real dry run); the router
  (`internal/api/router.go`) authenticates, authorizes, audits and resolves
  the caller's active cluster, which read/write/exec and plugin routes get
  as `call.rt`. Never return Secret values outside the reveal route; never
  record request bodies in the audit log.
- **Backend plugins** implement `internal/plugin.Plugin` (Manifest, Routes,
  Detect; optional Starter), call `plugin.Register` from `init()` and are
  linked by a blank import in `internal/plugins/all/all.go`; their routes
  live under `/api/plugins/<id>/` with `plugin.Access*` levels
  (`DryRunnable` only with a real dry run).
- New API endpoint → `internal/api/routes.go` (or the plugin's `Routes`),
  JSON shape matching `src/data/types.ts` (or the plugin's `data/types.ts`),
  wired in `src/data/api`, mocked in `src/data/mock`, listed in
  `docs/api.md`. Errors are `{"error", "code"}` with the codes of design §3.3.
- **External UI plugins** use the `window.KubeBench` SDK: template in
  `examples/plugins/hello-cluster`, types in `src/core/plugin/sdk/kubebench.d.ts`.
- All colors via `--vscode-*` tokens in `src/core/theme/theme.css` (dark and
  light); charts are single-series (small multiples, no legends).
- TS config: `verbatimModuleSyntax`, `noUnusedLocals`, `erasableSyntaxOnly`
  (no enums, parameter properties or namespaces). Verify frontend with
  `npx tsc -b && npm run build && npm run lint && npm run i18n:check &&
  npm test`; backend with `cd backend && test -z "$(gofmt -l .)" && go vet
  ./... && go test ./... && go build ./...`. Chart/packaging changes: `make
  chart-lint lint manifests`.
- User-visible changes get a line under `## [Unreleased]` in `CHANGELOG.md`
  (Keep a Changelog; the bundled changelog feeds What's new).
- Commits: Conventional Commits; never push or touch remotes unless asked.
- Dev: `npm run dev` (:5173) + `npm run server:go` (:9876). The dev server
  falls back to mock mode when the backend is down; production builds show
  a boot error instead; `?demo` forces mock mode. Test mutations only on
  disposable clusters.
- `backend/internal/web/dist/` holds only `.gitkeep` in git; `make build`
  and `docker build` copy the frontend in.
