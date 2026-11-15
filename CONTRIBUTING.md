# Contributing to KubeBench

Thank you for helping. This guide covers the development set-up, the rules
the code base relies on, how to extend KubeBench, the checks to run and how
pull requests are handled. Before you start on something large, open an
issue so we can agree on the approach.

## Development set-up

You need Node.js 24 with npm, Go (the version in `backend/go.mod`) and make.
Docker, Helm, kubectl and a local cluster (k3d or kind) help for end-to-end
work; GoReleaser, shellcheck, actionlint and kubeconform are needed only for
packaging and CI changes.

```bash
git clone https://github.com/Shavell/kube-bench.git && cd kube-bench
npm ci

npm run dev                                         # UI on http://localhost:5173 with demo data
KUBECONFIG=/path/to/kubeconfig npm run server:go    # backend on :9876 (the dev UI proxies /api to it)
make build && ./bin/kubebench                       # the production binary with the UI embedded
```

- The dev server falls back to deterministic demo data while the backend is
  down; `?demo` forces it. Production builds never use demo data.
- Use a **disposable** cluster for anything that changes objects. Never
  point a development backend at a cluster you cannot afford to break.
- `backend/internal/web/dist/` holds only `.gitkeep` in git. `make build`
  copies the frontend there; do not commit the copy (`make clean` removes it).

## Architecture in brief

- Backend: one Go binary in `backend/` — `internal/store` (SQLite),
  `internal/kube` (a Runtime per cluster), `internal/api` (route table,
  router, handlers), `internal/plugin` + `internal/plugins/<id>` (backend
  plugins), `internal/watch` (live updates), `internal/updates`,
  `internal/cloud` and `internal/cloudauth`, `internal/web` (embedded UI).
- Frontend: `src/core` (shell, plugin registry, boot, permissions, live
  updates, palette), `src/plugins/<id>` (every module), `src/data`
  (provider interfaces with live and mock implementations), `src/shared`
  (UI kit), `src/i18n`.

Read [docs/architecture.md](docs/architecture.md), and for behaviour the
contract [docs/design-v0.5.md](docs/design-v0.5.md). When code and contract
disagree, fix one of them in the same change.

## Rules

**Layering (frontend)**

- `src/core/shell` never imports `src/plugins/*`; plugins never import each
  other or `core/shell`.
- Cross-plugin flows use tab kinds (`useLayoutStore.getState().openTab`)
  and bottom panel requests (`showInPanel` in `core/plugin/panelRequests.ts`).
- Each module registers one `PluginManifest` in `src/plugins/index.ts`
  (admin-only modules in `adminPlugins`).

**Data: mock and live**

- Components read data only through `src/data/providers.ts` or plugin data
  (`definePluginData` in `src/data/pluginData.ts`).
- Every capability has **both** implementations: `data/mock` (deterministic:
  seeded RNG, no `Date.now()`/`Math.random()` in generated data) and
  `data/api` (the Go backend).
- **Live mode never falls back to mock data.** A capability the server
  reports missing rejects with `capability_unavailable`; the view shows
  Unavailable with the reason.
- After a mutation call `refreshData()`; declare live-update topics on
  queries that read cluster objects.

**i18n**

- Every user-visible string goes through i18next, in **every** language
  (`en`, `tr`, `de`): core strings in `src/i18n/locales`, platform strings in
  `src/core/locales`, plugin strings in `src/plugins/<id>/locales`
  (namespace = plugin id). Manifest display strings are keys.
- Kubernetes nouns stay untranslated (Turkish adds suffixes with an
  apostrophe: `Pod'lar`); Turkish uses the polite "siz" form, German "Sie".
  See [docs/i18n.md](docs/i18n.md).

**Permissions**

- Gate mutation UI with `usePermission` / `<Can>` from
  `src/core/permissions.ts`: `write` for changes, `exec` for shells, debug
  containers and Secret reveal. Show it disabled with the reason as tooltip,
  or hide it.
- Destructive actions ask through `ConfirmDialog` (type-to-confirm on
  production clusters).
- The backend is the authority: register every route once with the right
  access level (`read`, `write`, `exec`, `admin`). A dry-run variant may
  need only `read` (`allowDryRun` / `plugin.DryRunnable`), and then its
  handler must perform a real dry run.

**API**

- New endpoint → `backend/internal/api/routes.go` (or the plugin's
  `Routes`), a JSON shape that matches `src/data/types.ts` (or the plugin's
  `data/types.ts`), wired in `src/data/api` and mocked in `src/data/mock`,
  and listed in [docs/api.md](docs/api.md). Errors are
  `{"error","code"}` with the codes of design §3.3.

**Style**

- Colours only through the `--vscode-*` tokens in
  `src/core/theme/theme.css`, for dark and light. Charts are single-series
  small multiples without legends.
- TypeScript: `verbatimModuleSyntax`, `noUnusedLocals`, `erasableSyntaxOnly`
  (no enums, parameter properties or namespaces).
- Go: `gofmt`, `go vet`, idiomatic errors, table tests with fake clients
  (`k8s.io/client-go/kubernetes/fake`, `dynamic/fake`,
  `kube.NewRuntimeForTest`).
- Small focused files, accurate comments, meaningful tests, no dead code.

## Extending KubeBench

The full guide with code is [docs/plugins.md](docs/plugins.md).

- **Core UI plugin**: `src/plugins/<id>/manifest.tsx` + views + `locales/`
  (+ `data/` with `mock.ts` and `live.ts`), registered in
  `src/plugins/index.ts`.
- **Backend plugin**: `backend/internal/plugins/<id>/` implementing
  `plugin.Plugin` (`Manifest`, `Routes`, `Detect`), registered from `init()`
  and linked by a blank import in `backend/internal/plugins/all/all.go`.
  Its routes are served under `/api/plugins/<id>/`.
- **External UI plugin**: start from `examples/plugins/hello-cluster`; it
  uses the `window.KubeBench` SDK typed in
  `src/core/plugin/sdk/kubebench.d.ts`.
- **A UI language** is data only: add `<code>.json` next to every `en.json`
  under `src/` and `examples/`, set `language.name`, optionally map Monaco's
  message pack in `src/shared/editor/monacoLocale.ts`, and run
  `npm run i18n:check` ([docs/i18n.md](docs/i18n.md)).

## Checks

Run before you push:

```bash
# frontend
npx tsc -b && npm run build && npm run lint && npm run i18n:check && npm test
# backend
cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./...
```

Or `make test lint`. Chart, manifest, packaging and workflow changes also
need `make chart-lint`, `make manifests` (commit its output) and `make lint`
(shellcheck, actionlint). UI changes should be tried in the browser, in
demo mode and against a cluster.

## Commits

Use [Conventional Commits](https://www.conventionalcommits.org/):

```
feat(helm): install charts from an uploaded archive
fix(api): refuse exec plugins in pasted kubeconfigs
docs: describe the update check
```

Common types: `feat`, `fix`, `perf`, `refactor`, `test`, `docs`, `build`,
`ci`, `chore`. The scope is the plugin or area (`api`, `kube`, `ui`,
`chart`, `i18n`, …). The body explains why. Keep each commit focused and
building.

## Pull requests

1. Fork, create a branch from `main`, and keep it up to date by rebasing.
2. Fill in the pull request template: what and why, how you tested it.
3. Add a line under `## [Unreleased]` in [CHANGELOG.md](CHANGELOG.md) for
   every user-visible change, and update the docs it affects.
4. A maintainer reviews; please answer review comments with follow-up
   commits, which are squashed or rebased when merging.

Report security issues privately, not in a pull request: see
[SECURITY.md](SECURITY.md).

## License

KubeBench is source-available. The license has not been chosen yet; the
license file will be added before the first public release.

## Contributor License Agreement

Contributions require agreeing to a Contributor License Agreement (CLA).
The CLA lets the project offer KubeBench under commercial licenses as well
as under its public license. The agreement and how to accept it will be
published together with the license; a pull request is merged only once its
author has accepted it.
