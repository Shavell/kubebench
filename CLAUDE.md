# KubeBench — notes for AI assistants

VSCode-style Kubernetes workbench. **Frontend**: React 19 + TS + Vite,
zustand, recharts, Monaco. **Backend**: single Go binary in `backend/`
(client-go, Helm v3 SDK, modernc SQLite) that embeds the built frontend and
deploys into the cluster via `charts/kubebench`. Docs: `docs/design-go-backend.md`,
`docs/deployment.md`, `docs/architecture.md`, `docs/backend.md`.

Rules that matter here:

- **Frontend layering**: `core/shell` must not import from `plugins/*`;
  plugins must not import each other or `core/shell`. Cross-plugin flows go
  through tab kinds via `layoutStore.openTab`. New modules register a single
  `PluginManifest` in `src/plugins/index.ts` (the `users` plugin is registered
  at runtime in `main.tsx` for admins only).
- **Data access** only through the provider interfaces in
  `src/data/providers.ts`. Every capability needs BOTH implementations:
  `data/mock` (deterministic — seeded RNG, no `Date.now()`) and `data/api`
  (which calls the Go backend). `data/api` degrades to mock per-capability
  using the flags from `/api/info`.
- After mutations call `refreshData()` (invalidation bus in `data/DataContext.tsx`).
- **Backend layering**: `internal/store` (SQLite), `internal/kube` (per-cluster
  Runtime: clients, sampler, prometheus, helm, crds, exec, capabilities),
  `internal/api` (handlers + auth middleware), `internal/web` (go:embed). The
  active cluster is **per-user**; handlers resolve it via `withRuntime`.
- New API endpoint → add to `internal/api`, keep the JSON shape matching
  `src/data/types.ts`, and wire it in `data/api/index.ts`.
- All colors via `--vscode-*` tokens in `src/core/theme/theme.css`; charts are
  single-series (small multiples, no legends).
- TS config: `verbatimModuleSyntax`, `noUnusedLocals`, `erasableSyntaxOnly`
  (no enums). Verify frontend with `npx tsc -b && npm run build`; backend with
  `go build -C backend ./...`.
- Dev: `npm run dev` (:5173) + `npm run server:go` (:9876). App falls back to
  mock mode (no auth) when the backend is down.
- `backend/internal/web/dist/` holds only `.gitkeep` in git; the real frontend
  build is copied in during `docker build`.
