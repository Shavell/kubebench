# KubeBench — notes for AI assistants

VSCode-style Kubernetes workbench. React 19 + TS + Vite, zustand, recharts,
Monaco. Node proxy in `server/` (ESM, undici + js-yaml). Docs:
`docs/architecture.md` (layers, plugin API) and `docs/backend.md` (endpoints,
RBAC).

Rules that matter here:

- **Layering**: `core/shell` must not import from `plugins/*`; plugins must
  not import each other or `core/shell`. Cross-plugin flows go through tab
  kinds via `layoutStore.openTab`. New modules register a single
  `PluginManifest` in `src/plugins/index.ts`.
- **Data access** only through the provider interfaces in
  `src/data/providers.ts`. Any new capability needs BOTH implementations:
  `data/mock` (deterministic — seeded RNG, no `Date.now()` in data) and
  `data/api` (+ endpoint in `server/index.mjs`).
- After mutations call `refreshData()` (invalidation bus in
  `data/DataContext.tsx`).
- All colors via `--vscode-*` tokens in `src/core/theme/theme.css`; charts
  are single-series (small multiples, no legends).
- TS config uses `verbatimModuleSyntax`, `noUnusedLocals`,
  `erasableSyntaxOnly` (no enums). Verify with `npx tsc -b && npm run build`.
- Dev: `npm run dev` (:5173) + `KUBECONFIG=... npm run server` (:9876); the
  app falls back to mock mode when the proxy is down.
