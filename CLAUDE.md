# KubeBench — notes for AI assistants

VSCode-style Kubernetes workbench. **Frontend**: React 19 + TS + Vite,
zustand, recharts, Monaco (bundled locally), i18next. **Backend**: single Go
binary in `backend/` (client-go, Helm v3 SDK, modernc SQLite) that embeds the
built frontend and deploys into the cluster via `charts/kubebench`. The v0.5
implementation contract is `docs/design-v0.5.md`; other docs:
`docs/design-go-backend.md`, `docs/deployment.md`, `docs/architecture.md`,
`docs/backend.md`.

Rules that matter here:

- **Frontend layering**: `core/shell` must not import from `plugins/*`;
  plugins must not import each other or `core/shell`. Cross-plugin flows go
  through tab kinds via `layoutStore.openTab` (bottom panel views: kinds via
  `showInPanel` in `core/plugin/panelRequests.ts`). New modules register a
  single `PluginManifest` in `src/plugins/index.ts`; the admin-only plugins
  (`users`, `audit`) are registered at boot (`core/boot/startApp.tsx`) for
  global admins only.
- **Data access** only through the provider interfaces in
  `src/data/providers.ts` (plugin-owned data: `definePluginData` in
  `src/data/pluginData.ts`). Every capability needs BOTH implementations:
  `data/mock` (deterministic — seeded RNG, no `Date.now()`) and `data/api`
  (which calls the Go backend). Live mode never falls back to mock data: a
  capability `/api/info` reports missing rejects with `capability_unavailable`
  and the UI shows the reason.
- After mutations call `refreshData()` (invalidation bus in `data/DataContext.tsx`).
- **i18n**: every user-visible string goes through i18next (`en` + `tr`);
  core strings in `src/i18n/locales`, platform strings in `src/core/locales`,
  plugin strings in `src/plugins/<id>/locales` (namespace = plugin id).
  Manifest display strings are keys. `npm run i18n:check` must pass.
- **Permissions**: gate mutation UI with `usePermission`/`<Can>` from
  `src/core/permissions.ts` (disabled with the reason as tooltip, or hidden);
  destructive actions ask through `ConfirmDialog`.
- **Backend layering**: `internal/store` (SQLite), `internal/kube` (per-cluster
  Runtime: clients, sampler, prometheus, helm, crds, exec, capabilities),
  `internal/api` (handlers + auth middleware), `internal/web` (go:embed). The
  active cluster is **per-user**. Every route is registered once in
  `internal/api/routes.go` with an access level (public/authed/read/write/
  exec/admin); the router (`internal/api/router.go`) authenticates,
  authorizes, audits and resolves the caller's active cluster, which read/
  write/exec and plugin routes get as `call.rt`.
- **Backend plugins** implement `internal/plugin.Plugin`, call
  `plugin.Register` from `init()` and are linked by a blank import in
  `internal/plugins/all/all.go`; their routes live under `/api/plugins/<id>/`.
- New API endpoint → add it to `internal/api/routes.go`, keep the JSON shape
  matching `src/data/types.ts`, and wire it in `data/api/index.ts`. Errors
  are `{"error", "code"}` with the codes of design §3.3.
- **External UI plugins** use the `window.KubeBench` SDK: template in
  `examples/plugins/hello-cluster`, types in `src/core/plugin/sdk/kubebench.d.ts`.
- All colors via `--vscode-*` tokens in `src/core/theme/theme.css` (dark and
  light); charts are single-series (small multiples, no legends).
- TS config: `verbatimModuleSyntax`, `noUnusedLocals`, `erasableSyntaxOnly`
  (no enums). Verify frontend with `npx tsc -b && npm run build && npm run
  lint && npm run i18n:check`; backend with `cd backend && go vet ./... &&
  go test ./... && go build ./...`.
- Dev: `npm run dev` (:5173) + `npm run server:go` (:9876). The dev server
  falls back to mock mode when the backend is down; production builds show
  a boot error instead; `?demo` forces mock mode.
- `backend/internal/web/dist/` holds only `.gitkeep` in git; the real frontend
  build is copied in during `docker build`.
