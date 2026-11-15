# Workflow W1 (resume on the Fedora VM) — verbatim script

Resumed W1 from the WIP commits of the old VM (handoff §4): backend
foundation, frontend UI kit and platform, i18n conversion of the core and
the plugins (shared tree, disjoint file ownership), and packaging in its own
worktree; then an integrate-and-verify agent (contract alignment, every
check, an API end-to-end test with real mutations on the disposable k3d
cluster `kubebench-e2e`, a Playwright smoke test) and a packaging
end-to-end agent (GoReleaser snapshot, `.deb`/`.rpm`, `install.sh`, a local
image imported into k3d, Helm chart and manifests).

Ran 2026-11-14/15; result: commits `443afef`..`5f46352`, packaging merge
`763d3d3` and the packaging fixes `2c8a15a`..`24f29bb`.

The script is reproduced verbatim. `SCRATCH` is the session's scratch
directory on the Fedora VM (it held the k3d kubeconfigs, the Playwright
install and agent scratch files) and must be replaced before the script is
run elsewhere; see [README.md](README.md#adapting-the-scripts-to-a-new-session).

````js
export const meta = {
  name: 'kubebench-v05-w1-resume',
  description: 'Resume KubeBench v0.5 W1 on the Fedora VM: backend foundation, frontend UI kit + platform, i18n conversion, packaging (worktree), then integrate/verify on k3d and packaging e2e',
  phases: [
    { title: 'Build', detail: 'backend, frontend UI kit -> platform, i18n core, i18n plugins (shared tree, disjoint ownership) + packaging in its own worktree' },
    { title: 'Integrate', detail: 'contract alignment, full checks, k3d e2e with real mutations, Playwright smoke test, commits' },
    { title: 'Packaging e2e', detail: 'merge feat/packaging, goreleaser snapshot, deb/rpm/install.sh, local image on k3d, helm + manifests' },
  ],
}

const SCRATCH = '/tmp/claude-0/-root-workspace-tmp-kube-bench/e3b76051-98cd-4ef5-963e-080ccedcb0b7/scratchpad'
const REPO = '/root/workspace/tmp/kube-bench'
const E2E_KUBECONFIG = SCRATCH + '/e2e/kubeconfig'
const TRAILER = 'Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>'

const COMMON = `You are a senior engineer on KubeBench, a VSCode-style Kubernetes workbench (React 19 + TS + Vite frontend embedded in a single Go binary: client-go, Helm SDK, modernc SQLite). Repo: ${REPO} (branch release/v0.5.0). We are building v0.5.0.

READ FIRST: docs/design-v0.5.md (the implementation contract; sections referenced below as §N), CLAUDE.md, and docs/dev/v0.5-handoff.md §4 (what an earlier, interrupted run of this same plan already finished). Then read the existing code you will change before changing it.

THIS IS A RESUMED RUN. The plan started on another machine and was stopped mid-way; its partial output is committed on this branch (WIP commits 60e7297 backend, bcc2cb3 frontend, d6fd92b scripts/version.sh). Continue from that state: keep what is correct, fix what deviates from the contract, finish what is missing. Do not start over and do not rewrite working code for taste.

ENVIRONMENT (Fedora 44 VM, you run as root):
- Start every shell with: export PATH=/opt/kubebench-tools/bin:$PATH E2E_KUBECONFIG=${E2E_KUBECONFIG}
  (tools there: helm 3.22, kubectl 1.35, goreleaser v2, actionlint, kubeconform, nfpm). Also installed: go 1.27, node 24 + npm 11, docker (daemon running), shellcheck, dpkg-deb, jq, python3, make. node_modules is installed in the main repo (npm ci done) and backend Go modules are downloaded.
- Disposable test cluster: k3d cluster "kubebench-e2e" (k3s v1.35.5; nodes k3d-kubebench-e2e-server-0, k3d-kubebench-e2e-agent-0, k3d-kubebench-e2e-agent-1; metrics-server, Traefik ingress reachable from the host at http://127.0.0.1:18080, local-path default StorageClass). Kubeconfig file: ${E2E_KUBECONFIG} (API https://127.0.0.1:6550, context k3d-kubebench-e2e).
- Playwright + Chromium (headless, verified working) live in ${SCRATCH}/pw. Reuse them; never install Playwright into the repo. In your scripts: import { chromium } from '${SCRATCH}/pw/node_modules/playwright/index.mjs'

HARD RULES (on this machine they replace the cluster rules of contract §0):
- ~/.kube/config belongs to an UNRELATED external cluster. Never read it, never use it, never let a tool fall back to it: every kubectl/helm call passes --kubeconfig "$E2E_KUBECONFIG", and every kubebench process you start gets KUBECONFIG="$E2E_KUBECONFIG" or no KUBECONFIG at all. Never register a cluster in KubeBench from any other kubeconfig.
- The k3d cluster is shared with other agents working right now: never delete, stop or recreate it, and never change cluster-wide state (nodes, kube-system, CRDs, cluster RBAC) unless your task explicitly says so. Mutating requests (create/patch/delete/scale/exec, ...) are allowed only inside namespaces you create yourself, named e2e-<your-role>-<anything>; delete them when you are done.
- Never push, never touch git remotes, never push or tag images for a registry, never deploy anywhere except the disposable k3d cluster.
- Every user-visible frontend string goes through i18n (en + tr); npm run i18n:check must pass. Live mode never falls back to mock data. Mock data stays deterministic (seeded RNG, no Date.now()/Math.random() in generated data). Colors only via --vscode-* tokens (dark and light). TS config: verbatimModuleSyntax, noUnusedLocals, erasableSyntaxOnly (no enums, parameter properties or namespaces). Layering: core/shell never imports plugins; plugins never import each other or core/shell.
- Quality bar: production-grade, idiomatic, small focused files, comments where intent is not obvious, no dead code, no stub "TODO" features. The listed scope is expected to be completed.
- Scratch space: ${SCRATCH}/<your-role>/ (create it). Never leave temp files in the repo. Use only the ports assigned to you.

TURKISH UI GLOSSARY (keep consistent across the app): Workloads=İş Yükleri, Overview=Genel Bakış, Details=Ayrıntılar, Events=Olaylar, Logs=Loglar, Settings=Ayarlar, Metrics=Metrikler, Users=Kullanıcılar, Extensions/Plugins=Eklentiler, Audit log=Denetim kaydı, Read-only=Salt okunur, Viewer=İzleyici, Operator=Operatör, Admin(istrator)=Yönetici, Sign in=Giriş yap, Sign out=Çıkış yap, Password=Parola, Username=Kullanıcı adı, Delete=Sil, Restart=Yeniden başlat, Scale=Ölçekle, Rollback=Geri al, Uninstall=Kaldır, Install=Kur, Upgrade=Yükselt, Values=Değerler, Revision=Revizyon, Repository=Depo, Theme=Tema, Language=Dil, Dark=Koyu, Light=Açık, System=Sistem, Enabled=Etkin, Disabled=Devre dışı, Available=Kullanılabilir, Unavailable=Kullanılamıyor. "Cluster" stays "Cluster" (plural "Cluster'lar"). Kubernetes/Helm nouns stay untranslated: Pod, Deployment, StatefulSet, DaemonSet, ReplicaSet, Job, CronJob, Node, Namespace, Service, Ingress, ConfigMap, Secret, PVC, PV, StorageClass, CRD, Helm, release, chart — with Turkish suffixes after an apostrophe (Pod'lar, Node'u, Namespace'ler). Polite "siz" form, sentence case.

FINAL ANSWER: return the structured report.
`

const TEAM = `
TEAM SETUP: Several agents work in THIS SAME working tree (${REPO}) at the same time with strictly disjoint file ownership (yours is listed below).
- Only create/edit files you own. If you need a change elsewhere, do not make it; describe it in your final report under "requests" (file + exact change).
- Never run git commands that modify the index, working tree or branches (no add/commit/stash/checkout/reset/restore/clean/merge). Read-only git (status/diff/log/show) is fine. The integrator commits later.
- Builds may transiently fail because of another agent's in-progress edits in files you do not own; if a failure is outside your files, wait and retry later, and judge your own work by the errors in your files.
- Never run "npm run build" in this tree (it empties the shared dist/ while others build). For your frontend build check use: npx tsc -b && npx vite build --outDir ${SCRATCH}/<your-role>/dist --emptyOutDir
`

const BE = `
YOUR ROLE: backend foundation (Go), agent "w1-be". You own backend/** exclusively. Frontend agents are implementing the client side of the same contract right now, so implement the contract exactly (paths, methods, field names, error codes) and keep JSON shapes aligned with src/data/types.ts where the contract does not say otherwise. Ports: 19876.

STATUS FROM THE INTERRUPTED RUN (docs/dev/v0.5-handoff.md §4.1):
- DONE (compiles, tests pass): internal/config (Config, Load/FromEnv, all §2 variables with validation; KUBEBENCH_ALLOW_PATH_KUBECONFIG defaults off in-cluster; 6 tests) and the new internal/store (store.go, migrate.go with PRAGMA user_version v1->v2 that refuses newer schemas, users.go, sessions.go, clusters.go, audit.go, plugins.go: roles/assignments, cluster flags, password change and must-change, audit insert/list/prune, plugin settings and KV, last-admin protection, foreign-key cascades; tests). Review them against §2-§4 first and fix any gap.
- OPEN: items 1, 2 and 4-9 below. internal/api currently does NOT compile against the new store API (CanAccessCluster, UserClusterIDs and AssignClusters were removed; CreateUser gained a mustChangePassword bool; ListClusters returns []store.UserCluster). internal/plugin, internal/plugins/all and the kube.Runtime changes are not written yet.

Implement:
1. Build info (§1): version/commit/date vars in main.go; --version flag; "healthcheck" subcommand; GET /api/version (public).
2. Config (§2): the internal/config package exists (verify it parses every env var of the table once, with validation and defaults). main.go uses it; native TLS when cert+key are set; http.Server with ReadHeaderTimeout but no WriteTimeout (SSE/WebSocket arrive later); graceful shutdown on SIGINT/SIGTERM; background jobs: prune expired sessions hourly, prune audit rows daily per retention.
3. Store (internal/store): done in the interrupted run — verify: migrations driven by PRAGMA user_version (v1 = the old schema, v2 = the §3/§4 additions: user_clusters.role, clusters.read_only, clusters.environment, users.must_change_password, audit_log, plugin_settings(id, enabled, updated_at), plugin_kv(plugin_id, key, value, updated_at)); open with _pragma=foreign_keys(1); cleanup semantics of §3.5 (deleting the active cluster resets users.active_cluster_id). The bootstrap admin gets must_change_password when the default password is used.
4. Permissions and router (§3): one central route-registration helper with access levels public/authed/read/write/exec/admin; effective permission computation (admin > operator > viewer; read-only cluster blocks write/exec for everyone incl. admins); must_change_password gating; JSON errors {error, code} everywhere (§3.3); audit middleware for write/exec/admin mutations (§3.4) whose ResponseWriter wrapper captures the status but still implements http.Flusher and http.Hijacker (SSE/WebSocket will rely on them); dry-run requests (dryRun=true) need only read and are not audited; Origin check for non-GET; login rate limiter (§3.5); Secure cookie per config (honour X-Forwarded-Proto only when KUBEBENCH_TRUST_PROXY); POST /api/auth/password which revokes the user's other sessions.
5. API changes (§4): /api/info v2 including the capability-probe wait (<=3 s); users v2 (list shape with assignments, PATCH, assignments PUT incl. legacy clusterIds, protections: no self-delete, keep at least one admin); clusters v2 (role/readOnly/environment in the list, PATCH /api/clusters/{id}, path gating); GET /api/audit; events kind filter; logs tail/previous/sinceSeconds; GET /api/k8s/{path...} read passthrough (§4.5: GET only; only /api/, /apis/, /version; reject secrets paths and watch=true; cap 16 MiB; pass status codes through); GET and PUT /api/plugins (§4.6) with a Detect-status cache (60 s per cluster+plugin) and the settings map; external plugin discovery from KUBEBENCH_PLUGINS_DIR (read plugin.json, validate the id regex and entry filename) and GET /plugins/{id}/{file} (authed; traversal-safe via os.DirFS/fs.Sub; correct Content-Type for js/css/json/svg/png; no directory listing). Keep every existing endpoint working with its current JSON unless the contract changes it.
6. Plugin platform (§5): package internal/plugin (types, registry, WriteJSON/WriteError/WriteK8sError helpers, a Router implementation bound to the central route helper so plugin routes get the same auth/role/audit/plugin-enabled/Detect gating, KV backed by plugin_kv) and package internal/plugins/all (blank-import aggregator; no plugins yet; a doc comment shows how to add one); main.go imports it.
7. kube.Runtime (§5): change CS to kubernetes.Interface and Metrics to the metrics clientset interface so tests can inject fakes; add a test constructor (e.g. kube.NewRuntimeForTest(cluster, cs, dyn) that starts no background work); capability-probe readiness (WaitProbed(ctx)); Discovery(); Mapper() (memory-cached discovery + deferred RESTMapper; reset and retry once on no-match); ResolveKind(apiVersion, kind) (gvr, namespaced, err); ResolveResource(group, resource).
8. Tests (go test ./...): migrations (fresh DB, and an upgrade from a v1 DB created with the old DDL that keeps data and maps assignments to operator), permission matrix (viewer write -> 403 forbidden_role, read-only cluster -> 403 cluster_read_only even for admin, admin routes -> 403 admin_required, must-change-password gating), audit rows for mutations (none for GET or dry-run), rate limiter, Origin check, plugin mounting with a fake plugin (disabled -> 404 plugin_disabled, unavailable -> 503 plugin_unavailable), passthrough rejecting secrets and watch, external plugin file serving vs traversal attempts, /api/version, user protections. Use httptest and fake clientsets.
9. Verify: cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./... . Then a live smoke test of the binary from ${SCRATCH}/w1-be/ (fresh KUBEBENCH_DB there, PORT=19876): first without KUBECONFIG (no cluster), then with KUBECONFIG="$E2E_KUBECONFIG", using curl for: login, me, info (capabilities true after the probe wait), version, healthz, password change, users CRUD + assignments, clusters list + PATCH (readOnly/environment), plugins list + toggle, audit list, GET resource endpoints, events with kind, logs with tail/previous, passthrough (namespaces OK, secrets 403, watch rejected). Then exercise real mutations in a namespace e2e-w1-be-smoke that you create (a small Deployment, e.g. 2 replicas of registry.k8s.io/pause:3.10): as an operator user scale and restart it and delete a pod (they succeed and are audited); as a viewer user, and on a read-only cluster even as admin, the same calls return 403 before any Kubernetes call (cite the code path in your report and confirm nothing changed in the cluster). Delete the namespace and stop the server when done.
`

const FEA_UI = `
YOUR ROLE: frontend UI kit, agent "w1-fe-ui". You run in parallel with the backend agent and two i18n agents. When you finish, a "frontend platform" agent continues in the same tree and builds on your components (boot, status bar, Extensions/Users/Clusters/Audit plugins), so make the components complete, documented by their props, and easy to reuse. Ports: Vite dev server 15173.

You own: NEW files in src/shared/** (do not edit existing shared files: an i18n agent converts them right now); src/core/theme/**; src/core/locales/** (the "platform" namespace — put your keys under "ui.*" and "editor.*"; the platform agent adds more after you); src/plugins/settings/**; src/plugins/terminal/YamlEditorTab.tsx (only this file of the terminal plugin; its strings go into the platform namespace under "editor.*"); vite.config.ts, index.html, tsconfig*.json, package.json, package-lock.json. You may read anything.

STATUS FROM THE INTERRUPTED RUN: src/core/theme/themeController.ts (dark/light/system) and the settingsStore theme preference exist; src/core/locales registers the platform namespace but its bundles are empty; src/env.d.ts declares __APP_VERSION__ (not yet defined by Vite); Monaco still loads from a CDN (the @monaco-editor/react default), which breaks offline installs; no shared Dialog/Form/notify/Tabs/Unavailable/editor components exist yet.

Implement:
1. Shared UI (§6.6), new files in src/shared: Dialog (modal; Esc closes; focus first field; focus trap; aria-modal; portal), ConfirmDialog (danger variant; type-to-confirm when the active cluster environment is production or when the caller requests it — read the environment from the data mode/session in src/data/mode.ts, never from plugins), Form primitives (Field with label/help/error, TextInput, NumberInput, Select, Checkbox, Textarea, KeyValueEditor), notify + Toaster (notify.success/error/info; export Toaster — the platform agent mounts it in Shell.tsx), Tabs (in-page tab strip), Unavailable (EmptyState-like with a reason; maps ApiError codes such as capability_unavailable / plugin_unavailable / forbidden_role to friendly i18n text), editor/CodeEditor + editor/DiffEditor with Monaco bundled locally (loader.config({ monaco }) using the monaco-editor package, workers via Vite ?worker imports, lazy-loaded with React.lazy so the main chunk stays small, theme follows the app theme (vs-dark / vs), languages yaml/json/plaintext/shell). Keep files small; components use only --vscode-* tokens.
2. YamlEditorTab uses CodeEditor (read-only stays the default; keep the tab kind "yaml" props {text, readOnly} backwards compatible). Verify the production build contains no CDN references (grep the built output for jsdelivr/unpkg/cdnjs).
3. Themes (§6.7): complete light token set in src/core/theme/theme.css for every token (Light+ palette), settings theme dark|light|system with <html data-theme> (avoid a flash of the wrong theme on load), Monaco follows. Also add status-bar environment tokens for the platform agent — production (red), staging (amber), development (green) background + foreground, for both themes — and list their names in your report. src/shared/charts/chartTheme.ts is owned by an i18n agent: if it hardcodes colours, list the needed change under requests.
4. Settings page (src/plugins/settings): theme, language (SUPPORTED_LANGUAGES from '@/i18n' plus 'auto', using setLanguagePreference), the existing settings, plugin-contributed settingsSections (from the registry); all strings via i18n in the settings plugin namespace (src/plugins/settings/locales/{en,tr}.json + manifest.locales).
5. Define __APP_VERSION__ in vite.config.ts from package.json (declared in src/env.d.ts) and set package.json version to 0.5.0.
Verify: npx tsc -b && npx vite build --outDir ${SCRATCH}/w1-fe-ui/dist --emptyOutDir && npm run lint && npm run i18n:check (errors in files other agents own may be transient: retry later); confirm the bundle has no CDN references and that Monaco sits in a lazy chunk; run the dev server in mock mode (npx vite --port 15173 --strictPort, in the background; stop it when done) and drive it with Playwright: the Settings page switches theme (dark/light) and language (en/tr), a YAML tab renders Monaco with no requests to CDNs, no console errors; screenshots in your scratch folder.
`

function FEA_PLATFORM(ui) {
  return `
YOUR ROLE: frontend platform, agent "w1-fe-platform". Implement the client side of docs/design-v0.5.md §1 (frontend version), §3/§4 (session, permissions, users/clusters/audit/plugins/password APIs), §6.2-§6.5 and §6.8. The Go backend for these endpoints is being finished right now by another agent from the same contract; code against the contract and do not run the Go backend. Ports: Vite dev server 15174.

The UI-kit agent ("w1-fe-ui") finished just before you started. Reuse what it built — shared components in src/shared (Dialog, ConfirmDialog, Form primitives, notify/Toaster, Tabs, Unavailable, editor/CodeEditor, editor/DiffEditor), the light theme and the status-bar environment tokens in src/core/theme/theme.css, the Settings page — and do not re-implement them. Its report (null = it failed; then check what exists and build what you need in NEW src/shared files): ${JSON.stringify(ui)}

You own: src/core/** EXCEPT src/core/theme/** and the shell components an i18n agent converts (src/core/shell/ActivityBar.tsx, SideBar.tsx, CommandPalette.tsx, TabStrip.tsx, BottomPanel.tsx, EditorArea.tsx, LoginScreen.tsx and their .css files). You DO own src/core/shell/Shell.tsx, StatusBar.tsx (+css) and any NEW file in src/core/shell; src/core/locales/** (platform namespace; keep the ui.* and editor.* keys of the UI-kit agent); all of src/data/**; src/plugins/clusters/**, src/plugins/users/**, NEW src/plugins/extensions/** and src/plugins/audit/**, src/plugins/index.ts; src/main.tsx; NEW examples/**; NEW files in src/shared/** if really needed; package.json/package-lock.json only if you truly need a dependency. Do NOT edit src/i18n/locales/*.json (owned by an i18n agent); you may use "core" keys that already exist there (common.*). Strings: core-level UI goes into the "platform" namespace (src/core/locales/{en,tr}.json); plugin UI into the plugin's own namespace (src/plugins/<id>/locales/{en,tr}.json + manifest.locales).

STATUS FROM THE INTERRUPTED RUN (docs/dev/v0.5-handoff.md §4.2):
- DONE: src/data/apiClient.ts (ApiError/codes, qs, apiFetch/apiGet/apiPost/apiPut/apiPatch/apiDelete, onSessionLost); src/data/api split into cluster, crd, helm, metrics, admin (inventory, users, session, audit, plugins) and unavailable.ts (live mode rejects with capability_unavailable instead of falling back to mock); src/data/mock split (mockK8s, mockInventory, mockUsers + session, mockAudit, mockPlugins, delay); types/providers/mode extended for v2 (session, permissions, audit, plugins, assignments; mode.ts appVersion()); src/data/pluginData.ts (pluginApi, definePluginData, usePluginQuery, usePluginStatus); src/core/plugin types.ts (manifest v2), registry.ts v2 (active/enabled filtering, resourceActionsFor, settingsSections, subscribe/revision), pluginStatusStore.ts, useRegistry.ts; src/core/permissions.ts (deniedReason, usePermission).
- OPEN: boot in main.tsx; status bar; Extensions plugin; Users v2 UI (src/plugins/users/UsersPage.tsx has the only tsc error: it still passes number[] to assignClusters, which now takes {clusterId, role}[]); Clusters v2 UI; Audit plugin; external plugin loader + window.KubeBench SDK + example plugin; platform locale bundles.

Implement:
1. Review the WIP data layer against §6.2/§6.5 and the backend contract §3-§4 (paths, methods, query/body field names, response shapes, error codes) and fix gaps: every parameter through qs(); the events call passes kind; logs pass tail/previous/sinceSeconds; no hand-built query strings anywhere in src/data.
2. Boot (§6.3) in main.tsx: mock-mode policy (mock only in DEV, with ?demo, or VITE_DEMO_MODE=true; a production build with an unreachable/5xx backend shows a BootError screen with Retry — never mock); parse /api/info v2 into dataMode (version, commit, cluster, permissions, features, mustChangePassword); mustChangePassword -> forced ChangePasswordScreen (then reload); 401 -> the existing LoginScreen; then load plugin status (GET /api/plugins) and enabled external plugins before rendering the shell.
3. Live mode never uses mock data: confirm the helm/crd/exec/metrics paths in src/data/api reject with ApiError (status 503, code capability_unavailable, or the metric source error) so pages show the reason; no generateSeries in live mode.
4. Permissions UI (Can/usePermissions from src/core/permissions.ts, which you may extend) in your files: users, clusters, status bar. Status bar (§6.3): environment colouring (production red / staging amber / development green via the UI-kit tokens), a "Read-only" item when readOnly or role viewer, a user menu (Change password dialog, Sign out), a version item and an About command (version + commit). Mount the Toaster in Shell.tsx.
5. Plugin SDK v2 (§6.4): finish/verify the manifest + registry (version, description, category, core, backendPlugin, resourceActions, settingsSections); pluginStatusStore loaded from GET /api/plugins at boot (mock: all enabled/available); the registry filters disabled/unavailable plugins out of the activity bar, sidebar, commands and resource actions; registry.resourceActionsFor(kind). Mark the core plugins core: true (workloads, clusters, settings, terminal, extensions, users, audit). Keep the existing registry method names/signatures stable (the i18n agents' shell components call them) and do the filtering inside the registry. External plugins default to disabled until an admin enables them.
6. Extensions plugin (core): activity bar entry, sidebar list, detail tab; admin enable/disable (PUT /api/plugins/{id}); shows version, category, builtin/external, availability reason and external-plugin load errors.
7. Users plugin v2 (fixes the tsc error): global role + per-cluster assignment roles (viewer/operator) editor, create user with role, reset password (PATCH; tell the admin the user must change it at next login), change global role, delete with ConfirmDialog; server protections (409 conflict) surface as readable errors. Clusters plugin v2: environment/read-only/your-role badges, admin edit dialog (name, environment, read-only) via PATCH, kubeconfig path field only when features.pathKubeconfig. Audit plugin (admin only, registered at runtime like users in main.tsx): table (time, user, cluster, action, target, status, detail), filters (user, cluster, action), "Load more" pagination via nextBefore.
8. External plugins (§6.8): loader after /api/plugins (enabled external entries -> <script> with a 5 s timeout; failures recorded for the Extensions view); window.KubeBench global SDK (src/core/plugin/globalSdk.ts) with the listed surface; a hand-written typed declaration file for plugin authors; example plugin examples/plugins/hello-cluster (package.json, vite.config.ts library IIFE build with react and react/jsx-runtime mapped to KubeBench globals, plugin.json, src/index.tsx registering an activity bar entry + tab that lists namespaces through api.k8sGet('/api/v1/namespaces'), en/tr locales, README with build/install steps). Exclude examples from the app tsconfig/build; build the example once (npm install + npm run build inside its folder) to prove it works; add a .gitignore there so node_modules/dist are never committed.
9. Fill the platform locale bundles (en + tr) for everything you own, with complete natural Turkish per the glossary.
Verify: npx tsc -b && npx vite build --outDir ${SCRATCH}/w1-fe-platform/dist --emptyOutDir && npm run lint && npm run i18n:check; run the dev server in mock mode (npx vite --port 15174 --strictPort, in the background; stop it when done) and drive it with Playwright: the shell renders, Extensions lists plugins and toggles one, the Users page creates a user and assigns a role (mock), the Audit page loads and paginates, the status bar shows the environment/read-only items and the user menu, a production-environment ConfirmDialog asks to type the name; no console errors; screenshots in your scratch folder.
`
}

const FEB_CORE = `
YOUR ROLE: internationalize the existing UI (English + Turkish) and gate mutation UI by permission, part A — agent "w1-i18n-core": the shell, the existing shared components and the workloads plugin. Another agent ("w1-i18n-plugins") converts the metrics, crd, helm and terminal plugins at the same time. The i18n infrastructure already exists (read src/i18n/index.ts, src/i18n/format.ts, scripts/check-i18n.mjs and §6.1). Nothing of this conversion was started in the interrupted run.

You own: src/i18n/locales/en.json + tr.json (namespace "core"; extend them — other agents only read them); src/core/shell/ActivityBar.tsx, SideBar.tsx, CommandPalette.tsx, TabStrip.tsx, BottomPanel.tsx, EditorArea.tsx, LoginScreen.tsx (+ their css); every EXISTING file in src/shared/** (Badge, ConfirmButton, EmptyState, EventsTable, Icon, ListFilterBar, ResizeHandle, Table, Toolbar, Tree, contextMenu, charts/*); and src/plugins/workloads/**. Other agents concurrently own src/core/plugin/**, src/core/store/**, src/core/theme/**, src/core/locales/**, src/core/permissions.ts, src/data/**, Shell.tsx, StatusBar.tsx, NEW files in src/shared/**, src/plugins/{settings,clusters,users,extensions,audit}/**, src/plugins/terminal/YamlEditorTab.tsx, src/plugins/index.ts, main.tsx, vite.config.ts, package.json (frontend agents) and src/plugins/{metrics,crd,helm,terminal}/** (the other i18n agent). Do not edit those.

Implement:
1. Every user-visible string in your files goes through t() (react-i18next useTranslation(ns)). Shared/shell strings use the "core" namespace. The workloads plugin gets src/plugins/workloads/locales/en.json + tr.json and a manifest "locales" field (import the JSON files); manifest display strings (name, activityBar.tooltip, sidebar.title, command titles, bottom panel titles) become keys in the plugin namespace (e.g. "manifest.name", "commands.openPods").
   FIRST, within your first minutes: add a "common" group to the core namespace with generic words other agents can reuse (for example refresh, delete, cancel, close, save, apply, edit, create, search, filter, loading, error, retry, name, namespace, status, age, yes, no, copy, download, actions, details, unknown, none, all). Other agents may only use core keys that already exist, so add these early and never rename them afterwards.
2. Shell components render manifest strings with useManifestText() from '@/i18n'.
3. Cover column headers (resourceCatalog), toolbar buttons, empty states, error banners, confirm prompts, context menu items (rowActions), tooltips, placeholders, aria-labels, unit labels, chart titles and the login screen. Plurals via _one/_other suffixes. Numbers/dates shown to users use src/i18n/format.ts where it makes sense (compact ages like "5m" may stay).
4. Permission gating with usePermissions/can/<Can> from src/core/permissions.ts in your files: delete/restart/scale/rollback/uninstall/edit need canWrite; shell/exec needs canExec. Hide context-menu items and disable buttons with an explanatory i18n tooltip that distinguishes "cluster is read-only" from "your role is viewer" (see deniedReason in permissions.ts).
5. Complete, natural Turkish translations following the glossary; npm run i18n:check must pass (key parity + interpolation variables).
6. Do not change behaviour beyond i18n and gating; keep mock data (event messages, log lines, resource names) in English.
Verify: npx tsc -b && npx vite build --outDir ${SCRATCH}/w1-i18n-core/dist --emptyOutDir && npm run lint && npm run i18n:check (failures caused by other agents' in-progress files: retry later, report if persistent). Then grep your files for leftover user-visible English literals (JSX text nodes and title=/placeholder=/aria-label= props with Capitalized words) and fix them.
`

const FEB_PLUGINS = `
YOUR ROLE: internationalize the existing UI (English + Turkish) and gate mutation UI by permission, part B — agent "w1-i18n-plugins": the metrics, crd, helm and terminal plugins. Another agent ("w1-i18n-core") converts the shell, the shared components and the workloads plugin at the same time and owns the "core" namespace. The i18n infrastructure already exists (read src/i18n/index.ts, src/i18n/format.ts, scripts/check-i18n.mjs and §6.1). Nothing of this conversion was started in the interrupted run.

You own: src/plugins/metrics/**, src/plugins/crd/**, src/plugins/helm/**, and src/plugins/terminal/** EXCEPT YamlEditorTab.tsx (owned by the UI-kit agent). Do not edit anything else.

Implement:
1. Every user-visible string in your plugins goes through t() (react-i18next useTranslation(ns)). Each plugin gets src/plugins/<id>/locales/en.json + tr.json and a manifest "locales" field (import the JSON files); manifest display strings (name, activityBar.tooltip, sidebar.title, command titles, bottom panel titles, status bar item text) become keys in the plugin namespace (e.g. "manifest.name", "commands.openReleases"). Put every string into the plugin's own namespace; you may use core "common.*" keys only if they already exist in src/i18n/locales/en.json (never add core keys).
2. Cover table headers, toolbar buttons, empty states, error banners, confirm prompts, context menu items, tooltips, placeholders, aria-labels, unit labels, chart titles/axis units and panel titles. Plurals via _one/_other suffixes. Numbers/dates/bytes shown to users use src/i18n/format.ts where it makes sense.
3. Permission gating with usePermissions/can/<Can> from src/core/permissions.ts: Helm rollback/uninstall need canWrite; the terminal (exec) needs canExec; hide or disable with an explanatory i18n tooltip that distinguishes "cluster is read-only" from "your role is viewer" (deniedReason).
4. Complete, natural Turkish translations following the glossary; npm run i18n:check must pass.
5. Do not change behaviour beyond i18n and gating; keep mock data in English. The helm and terminal plugins will be extended by later feature work (Helm v2, xterm terminal), so keep their structure as is — but convert them completely now.
Verify: npx tsc -b && npx vite build --outDir ${SCRATCH}/w1-i18n-plugins/dist --emptyOutDir && npm run lint && npm run i18n:check (transient failures caused by other agents: retry later). Then grep your files for leftover user-visible English literals and fix them.
`

const PKG = `
YOUR ROLE: distribution, release and CI scaffolding — agent "w1-packaging". You work in your OWN git worktree of the repo (your current working directory; it was created from release/v0.5.0). Other agents are concurrently changing the application code (src/**, backend/**) in the main tree — do not touch those paths.

FIRST: confirm you are inside a worktree (git rev-parse --show-toplevel must NOT be ${REPO} itself), then create your branch: git checkout -b feat/packaging. node_modules is per worktree: run npm ci in your worktree only if you need it.

STATUS FROM THE INTERRUPTED RUN: only scripts/version.sh was written (prints the build version: $VERSION, else git describe on v* tags, else <package.json version>-<sha>). Everything below is still to do.

LIMITATION OF THIS STEP: in your worktree the backend does NOT compile yet (backend/internal/api is being rewritten right now in the main tree; main.go there gains the version vars, --version and the healthcheck subcommand). So you cannot build the real binary or image here. Write everything against the contract and validate it as described below. A follow-up agent merges your branch after the backend lands and runs the real-artifact tests (goreleaser snapshot, real .deb/.rpm, install.sh, a local image on the k3d cluster, helm install, manifests) — make that easy and list the exact commands in your report.

Read first: docs/design-v0.5.md — especially §1 (versioning/build info), §2 (environment variables), §8 (distribution); this is the contract, follow it exactly. Also CLAUDE.md, README.md, Dockerfile, .dockerignore, .gitignore, charts/kubebench/**, docs/deployment.md, backend/main.go, package.json.

Facts:
- GitHub repo: Shavell/kube-bench (https://github.com/Shavell/kube-bench). Container image: ghcr.io/shavell/kube-bench. Helm chart OCI: oci://ghcr.io/shavell/charts (chart name kubebench). Release version: 0.5.0.
- Build: npm ci && npm run build produces dist/ (Vite); it must be copied to backend/internal/web/dist/ before go build (go:embed). Go module is in backend/ (module name kubebench, main package at backend/). CGO_ENABLED=0 (pure-Go SQLite).
- ldflags: -s -w -X main.version=<v> -X main.commit=<sha> -X main.date=<rfc3339> (variables are being added to main.go in parallel; -X on a missing var is harmless).
- "kubebench healthcheck" (exit 0/1; GET http://127.0.0.1:$PORT/healthz, https when KUBEBENCH_TLS_* are set) and "kubebench --version" (prints "kubebench <version> (<commit>, <date>)") are being added in parallel — use them (Docker HEALTHCHECK, install verification) without implementing them.
- The backend reads the env vars of design §2 (PORT, KUBEBENCH_LISTEN_ADDR, KUBEBENCH_DB, KUBEBENCH_DATA_DIR, KUBEBENCH_PLUGINS_DIR, KUBEBENCH_ADMIN_PASSWORD, KUBEBENCH_SECURE_COOKIES, KUBEBENCH_TRUST_PROXY, KUBEBENCH_TLS_CERT/KEY, KUBEBENCH_SESSION_TTL, KUBEBENCH_AUDIT_RETENTION_DAYS, KUBEBENCH_ALLOW_PATH_KUBECONFIG, KUBECONFIG). When KUBEBENCH_ADMIN_PASSWORD is not set the bootstrap admin is admin/admin and must change the password at first login.
- The project has no LICENSE yet — do not invent one. Leave license fields unset with a "# TODO: set once the project picks a license" comment.
- Node: use node 24 (current LTS, matches the dev machine) in the Dockerfile and CI. Go: from backend/go.mod.
- Network access to proxy.golang.org, npm, GitHub and Docker Hub works.

Your files (exclusive ownership): Dockerfile, .dockerignore, .gitignore (append only), .editorconfig, Makefile, .goreleaser.yaml, install.sh (repo root), CHANGELOG.md (new), charts/kubebench/**, deploy/**, packaging/**, scripts/*.sh (new; do NOT touch scripts/check-i18n.mjs), .github/**, docs/deployment.md (rewrite), docs/releasing.md (new). Do NOT modify src/**, backend/**, package.json, package-lock.json, README*, other docs.

Deliverables:
1. Makefile with documented targets: help (default), deps, frontend (npm ci + build), embed (copy dist -> backend/internal/web/dist, keep .gitkeep handling sane), build (embedded binary into bin/kubebench with ldflags from git), run, test (go test + npm checks: tsc, lint, i18n:check), lint, chart-lint, manifests (render deploy/kubernetes manifests), snapshot (goreleaser snapshot), deb, offline-bundle, clean, version. VERSION from scripts/version.sh (git describe --tags --always --dirty with fallback to the package.json version).
2. Dockerfile: multi-stage, multi-arch friendly (--platform=$BUILDPLATFORM builder stages, TARGETOS/TARGETARCH), ARG VERSION/COMMIT/DATE -> ldflags, npm/go cache mounts, OCI labels (org.opencontainers.image.source=https://github.com/Shavell/kube-bench, title, description, version, revision, created), distroless nonroot runtime, HEALTHCHECK CMD ["/kubebench","healthcheck"], KUBEBENCH_DATA_DIR=/data. Update .dockerignore so required inputs (scripts used by the build, src, public, index.html, configs) are included and junk excluded. Do not build the KubeBench image in this step; validate by reading carefully and with "docker buildx build --check ." (lint only).
3. Helm chart charts/kubebench -> version/appVersion 0.5.0; home/sources -> the GitHub URL; image repo ghcr.io/shavell/kube-bench (tag defaults to appVersion); strategy Recreate (single replica + RWO PVC); readiness/liveness/startup probes on /healthz; pod + container securityContext (runAsNonRoot, readOnlyRootFilesystem true with an emptyDir for /tmp, allowPrivilegeEscalation false, capabilities drop ALL, seccompProfile RuntimeDefault); rbac.preset: cluster-admin (default, binds the built-in ClusterRole), read-only (chart-defined ClusterRole: get/list/watch on all resources except secrets, plus pods/log, metrics, services/proxy get), custom (uses rbac.rules); auth.adminPassword (empty -> random value generated once and persisted across upgrades via lookup; NOTES.txt prints the kubectl command to read it), auth.existingSecret; config.* mapped to the env vars (secureCookies, trustProxy (default true when ingress is enabled), sessionTTL, auditRetentionDays, allowPathKubeconfig default false); extraEnv, extraVolumes/extraVolumeMounts, podAnnotations, podLabels, serviceAccount.annotations, persistence.existingClaim, networkPolicy.enabled (ingress from anywhere on the http port, egress to the API server + DNS + optional extra CIDRs; remember that egress policies see the API server endpoint IP/port, not only the kubernetes Service IP), ingress with a TLS example, values.schema.json, chart README.md with a values table. helm lint --strict must pass; render with helm template and validate with kubeconform (-strict -summary, Kubernetes 1.30+ schemas; skip CRDs if any).
4. Plain manifests deploy/kubernetes/kubebench.yaml rendered from the chart by scripts/render-manifests.sh (namespace kubebench, Namespace object included, NO admin password Secret baked in — rely on the forced first-login change), plus deploy/kubernetes/README.md (kubectl apply, change the image tag, expose, uninstall). Commit the rendered file.
5. Packaging for VMs (nfpm inside GoReleaser): packaging/systemd/kubebench.service (User=kubebench, EnvironmentFile=-/etc/kubebench/kubebench.env, WorkingDirectory=/var/lib/kubebench, KUBEBENCH_DATA_DIR=/var/lib/kubebench, KUBEBENCH_DB=/var/lib/kubebench/kubebench.db, Restart=on-failure, hardening: NoNewPrivileges, ProtectSystem=strict + ReadWritePaths=/var/lib/kubebench, ProtectHome=read-only (kubeconfigs may live in home dirs), PrivateTmp, an empty CapabilityBoundingSet, etc.), packaging/kubebench.env (documented defaults, every variable commented), packaging/scripts/{postinstall,preremove,postremove}.sh (create system user/group kubebench, /var/lib/kubebench 0750, /etc/kubebench/kubebench.env 0640 root:kubebench; systemctl daemon-reload + enable + start/restart only when systemd is running; purge removes user/data only on purge; must not fail in containers without systemd), packaging/README.md.
6. .goreleaser.yaml (v2 schema): custom dist build/release (Vite already uses dist/); before.hooks: build frontend + embed; builds for linux amd64/arm64, darwin amd64/arm64, windows amd64 (the binary also works as a local desktop-style app with KUBECONFIG); -trimpath, ldflags; archives (tar.gz, zip on windows) named kubebench_{{ .Version }}_{{ .Os }}_{{ .Arch }} including README.md, CHANGELOG.md, install.sh; nfpms deb + rpm (package name kubebench, maintainer/homepage/description; contents per item 5; config|noreplace for the env file); checksum checksums.txt (sha256); release to GitHub Shavell/kube-bench, draft: true, prerelease: auto, extra_files glob for the chart tgz / manifests / offline bundle / install.sh produced by the release workflow into build/extra/; changelog disabled (notes come from CHANGELOG.md via --release-notes); snapshot version template. Validate with goreleaser check. A real snapshot is impossible in your tree (see the limitation), so test the package contents with a stand-in binary: write a tiny Go program in your scratch dir that answers --version and healthcheck and serves /healthz on $PORT, package it with nfpm using a temporary nfpm config in your scratch dir that mirrors the goreleaser nfpms section (same contents, scripts, config|noreplace), and test: docker run --rm -v <dir>:/pkg debian:trixie-slim bash -c 'dpkg -i /pkg/kubebench_*_amd64.deb && kubebench --version; id kubebench; ls -la /etc/kubebench /var/lib/kubebench; dpkg -r kubebench; dpkg -P kubebench; id kubebench || echo user-removed', and the .rpm in fedora:44 (rpm -i / rpm -e). There is no systemd in these containers: the scripts must succeed anyway. Pulling these public base images is allowed.
7. install.sh (repo root, bash, set -Eeuo pipefail, shellcheck-clean): usage "curl -fsSL https://raw.githubusercontent.com/Shavell/kube-bench/main/install.sh | sudo bash" and "... | sudo bash -s -- --version v0.5.0". Options: --version <vX.Y.Z|latest> (latest resolved via the https://github.com/Shavell/kube-bench/releases/latest redirect, no jq), --method auto|deb|rpm|tarball, --offline <path> (an offline bundle .tar.gz, a .deb/.rpm, or a kubebench_*_linux_<arch>.tar.gz), --port, --listen, --admin-password (otherwise generate 20 random chars, write them to the env file 0640, print once at the end), --kubeconfig <file> (copied to /etc/kubebench/kubeconfig 0640 root:kubebench, KUBECONFIG set in the env file), --data-dir, --no-start, --uninstall [--purge], --dry-run, -y/--yes, -h/--help. Detect arch (amd64/arm64) and OS (linux; on macOS print tarball instructions), use curl or wget, verify SHA-256 against checksums.txt (sha256sum or shasum -a 256), install via dpkg/rpm or tarball (binary -> /usr/local/bin + systemd unit + env file + user/dirs, same layout as the package), idempotent upgrades preserve the env file and data, work without systemd (print how to run in the foreground). Test it with stand-in artifacts (a tarball laid out exactly like the goreleaser archive, and the stand-in .deb) inside debian:trixie-slim containers: offline install of the tarball and of the .deb, idempotent re-install, --uninstall and --uninstall --purge, --dry-run; run shellcheck.
8. scripts/: render-manifests.sh, offline-bundle.sh <version> (assembles kubebench-offline-<version>.tar.gz from build/release artifacts + helm package of the chart + rendered manifests + install.sh + optional container image tarballs passed via --image-tar (it must not build or pull images itself) + OFFLINE-README.md with air-gapped steps for Kubernetes (docker load / ctr -n k8s.io images import / k3s ctr, pushing to a private registry, helm install from the tgz) and VMs; writes a .sha256), extract-changelog.sh <version> (prints that version's section of CHANGELOG.md, Keep a Changelog format; exits non-zero if missing), version.sh. All shellcheck-clean.
9. CHANGELOG.md (Keep a Changelog 1.1 + SemVer): an [Unreleased] section and a [0.5.0] section that summarises the v0.5 scope from docs/design-v0.5.md (the integrator refines it later), plus backfilled 0.1.0-0.4.0 entries derived from the git history before acc8149 (git log --reverse; group the existing feat commits sensibly into those versions and say in a comment at the top how they were reconstructed).
10. GitHub (.github/): workflows ci.yml (frontend: npm ci, lint, tsc, build, i18n:check; backend: go vet/test/build with an embedded placeholder; helm lint + kubeconform; shellcheck on install.sh/scripts; actionlint), release.yml (inputs: version like v0.5.0, prerelease bool, push_image bool default false, push_chart bool default false: checkout with tags, set up node/go/helm, run tests, create the tag if missing, extract release notes from CHANGELOG.md, build the frontend, goreleaser release (draft), helm package (+ optional OCI push), render manifests, optional multi-arch image build/push with buildx + QEMU and docker save tarballs for the offline bundle, build the offline bundle, upload extra files), docker-image.yml (manual multi-arch build, optional push, inputs tag/platforms), helm-chart.yml (manual lint/package/push OCI), codeql.yml (manual, go + javascript-typescript). Every workflow must trigger ONLY on workflow_dispatch; put the future triggers (push/pull_request/tags/schedule) in a commented block with a one-line note that CI is intentionally manual for now. Least-privilege permissions, concurrency groups, pinned major action versions (check what exists today via the GitHub API, e.g. actions/checkout, actions/setup-node, actions/setup-go, docker/*, goreleaser/goreleaser-action, azure/setup-helm), Go from backend/go.mod (go-version-file). Validate all with actionlint. Also dependabot.yml (version 2; ecosystems npm "/", gomod "/backend" with groups for k8s.io/* + sigs.k8s.io/*, helm.sh/*, golang.org/x; npm groups for react, vite/rolldown/typescript tooling, i18n, monaco/xterm; github-actions "/"; docker "/"; weekly schedule; open-pull-requests-limit 5; label dependencies; commit-message prefix chore(deps)), ISSUE_TEMPLATE/bug_report.yml, feature_request.yml, config.yml, PULL_REQUEST_TEMPLATE.md, CODEOWNERS (owner Shavell).
11. Docs: rewrite docs/deployment.md (Kubernetes via Helm (OCI + tgz) and plain manifests; VM via installer / .deb / .rpm / tarball; Docker; offline/air-gapped for both; configuration reference (env vars and chart values, by link); TLS and ingress; upgrades; backup/restore of the SQLite DB; uninstall) and create docs/releasing.md (versioning (SemVer), CHANGELOG discipline, how to cut a release with the manual workflows or locally with make snapshot/goreleaser, what artifacts a release contains, how to activate CI later by uncommenting triggers). English.

Rules for this step:
- Never push, never touch git remotes, never push images. Do not build the KubeBench container image in this step. Running containers from public base images (debian:trixie-slim, fedora:44) to test packages is allowed (always --rm). Do not contact any Kubernetes cluster in this step.
- Scratch space: ${SCRATCH}/w1-packaging/ (never leave temp files in your worktree).
- Commit in your worktree on branch feat/packaging in a few logical Conventional Commits (build: ..., ci: ..., docs: ...), each ending with the trailer line "${TRAILER}". Leave the worktree clean (no stray build artifacts committed; add ignores).

FINAL ANSWER: the structured report — branch, worktree path, commits (sha + subject), what you verified and how (exact commands + results: helm lint, kubeconform, goreleaser check, actionlint, shellcheck, stand-in deb/rpm/installer container tests), the exact follow-up commands for real-artifact testing, anything that could not be verified, contract mismatches and open questions (e.g. license).
`

const REPORT = {
  type: 'object',
  properties: {
    summary: { type: 'string' },
    filesChanged: { type: 'array', items: { type: 'string' } },
    verification: {
      type: 'array',
      items: {
        type: 'object',
        properties: { command: { type: 'string' }, result: { type: 'string' } },
        required: ['command', 'result'],
      },
    },
    requests: {
      type: 'array',
      items: {
        type: 'object',
        properties: { file: { type: 'string' }, change: { type: 'string' } },
        required: ['file', 'change'],
      },
    },
    contractDeviations: { type: 'array', items: { type: 'string' } },
    openIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['summary', 'filesChanged', 'verification', 'requests', 'contractDeviations', 'openIssues'],
}

const CHECKS = {
  type: 'array',
  items: {
    type: 'object',
    properties: { command: { type: 'string' }, result: { type: 'string' } },
    required: ['command', 'result'],
  },
}

const PKG_REPORT = {
  type: 'object',
  properties: {
    branch: { type: 'string' },
    worktreePath: { type: 'string' },
    commits: { type: 'array', items: { type: 'string' } },
    summary: { type: 'string' },
    filesTouched: { type: 'array', items: { type: 'string' } },
    verification: CHECKS,
    followUpCommands: { type: 'array', items: { type: 'string' } },
    notVerified: { type: 'array', items: { type: 'string' } },
    openIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['branch', 'worktreePath', 'commits', 'summary', 'filesTouched', 'verification', 'followUpCommands', 'notVerified', 'openIssues'],
}

const VERIFY = {
  type: 'object',
  properties: {
    summary: { type: 'string' },
    fixes: { type: 'array', items: { type: 'string' } },
    checks: CHECKS,
    e2e: { type: 'string' },
    browser: { type: 'string' },
    commits: { type: 'array', items: { type: 'string' } },
    remainingIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['summary', 'fixes', 'checks', 'e2e', 'browser', 'commits', 'remainingIssues'],
}

const PKG_E2E_REPORT = {
  type: 'object',
  properties: {
    merged: { type: 'boolean' },
    mergeCommit: { type: 'string' },
    commits: { type: 'array', items: { type: 'string' } },
    fixes: { type: 'array', items: { type: 'string' } },
    checks: CHECKS,
    e2e: { type: 'string' },
    remainingIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['merged', 'mergeCommit', 'commits', 'fixes', 'checks', 'e2e', 'remainingIssues'],
}

phase('Build')
// Packaging runs in its own worktree for the whole Build + Integrate span.
const pkgPromise = agent(COMMON + PKG, { label: 'w1-packaging', phase: 'Build', isolation: 'worktree', schema: PKG_REPORT })

const [be, fe, febCore, febPlugins] = await parallel([
  () => agent(COMMON + TEAM + BE, { label: 'w1-backend', phase: 'Build', schema: REPORT }),
  async () => {
    const ui = await agent(COMMON + TEAM + FEA_UI, { label: 'w1-fe-ui', phase: 'Build', schema: REPORT })
    const platform = await agent(COMMON + TEAM + FEA_PLATFORM(ui), { label: 'w1-fe-platform', phase: 'Build', schema: REPORT })
    return { ui, platform }
  },
  () => agent(COMMON + TEAM + FEB_CORE, { label: 'w1-i18n-core', phase: 'Build', schema: REPORT }),
  () => agent(COMMON + TEAM + FEB_PLUGINS, { label: 'w1-i18n-plugins', phase: 'Build', schema: REPORT }),
])
const ui = fe ? fe.ui : null
const platform = fe ? fe.platform : null
log(`Build done: backend=${be ? 'ok' : 'FAILED'} ui=${ui ? 'ok' : 'FAILED'} platform=${platform ? 'ok' : 'FAILED'} i18n-core=${febCore ? 'ok' : 'FAILED'} i18n-plugins=${febPlugins ? 'ok' : 'FAILED'}`)

phase('Integrate')
const VERIFY_PROMPT = `${COMMON}
YOUR ROLE: integrator and verifier for the v0.5 foundation, agent "w1-verify", in the MAIN working tree ${REPO}. The build agents (backend foundation, frontend UI kit, frontend platform, i18n core, i18n plugins) have FINISHED, so you may now edit ANY file in the main tree. The packaging agent works separately in its own worktree on branch feat/packaging; another agent merges it after you — do not touch it. Ports: binary 19877, Vite 15175.

Their reports (JSON; null means the agent failed — then inspect its area yourself and finish what is missing):
BACKEND: ${JSON.stringify(be)}
UI KIT: ${JSON.stringify(ui)}
PLATFORM: ${JSON.stringify(platform)}
I18N CORE: ${JSON.stringify(febCore)}
I18N PLUGINS: ${JSON.stringify(febPlugins)}

Tasks:
1. Apply the valid "requests" from the reports; resolve the listed contract deviations (fix the side that deviates from docs/design-v0.5.md, or if the contract is wrong, fix docs/design-v0.5.md and say so).
2. Contract alignment: compare the backend handlers (backend/internal/api/**, backend/internal/plugin/**) with the frontend callers (src/data/**, plugins using pluginApi) for every endpoint of §3-§4: paths, methods, query/body field names, response shapes, error codes. Fix mismatches.
3. Run everything and fix all failures: npx tsc -b --force; npm run build; npm run lint (pre-existing only-export-components warnings are fine, new errors are not); npm run i18n:check; cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./... . Also scan for leftover user-visible English literals across src/ and fix them, and check the layering rules (core/shell must not import plugins; plugins must not import each other or core/shell).
4. End-to-end test in ${SCRATCH}/w1-verify/: build the embedded binary (npm run build; copy dist/* into backend/internal/web/dist/ keeping .gitkeep; go build -o ${SCRATCH}/w1-verify/kubebench . from backend/), run it on PORT=19877 with a fresh KUBEBENCH_DB in that folder and KUBECONFIG="$E2E_KUBECONFIG". Create a namespace e2e-w1-verify-app with a small Deployment (2 replicas of registry.k8s.io/pause:3.10). With curl: GET / and one JS asset; login admin/admin; GET /api/nodes -> 403 password_change_required; POST /api/auth/password; /api/info (permissions, capabilities true, cluster); /api/version; /api/plugins; create user viewer1 with role viewer on the cluster and user op1 with role operator; log in as each (forced password change, since the admin set the password); as viewer1: GET /api/pods OK, deleting a pod -> 403 forbidden_role and the pod still exists; as op1: deleting a pod succeeds (the ReplicaSet recreates it) and scaling the Deployment succeeds; as admin PATCH the cluster readOnly=true; admin and op1 write attempts -> 403 cluster_read_only and nothing changed in the cluster (first prove from the code that the checks run before any Kubernetes call); PATCH readOnly=false; passthrough GET /api/k8s/api/v1/namespaces OK, a secrets path -> 403, watch=true -> rejected; GET /api/audit lists the mutations with user, cluster, target and status. Afterwards restore backend/internal/web/dist to contain only .gitkeep, stop the server and delete the namespace.
5. Browser smoke test with the Playwright install in ${SCRATCH}/pw (time-box about 30 minutes): against the binary (rebuild/restart as in step 4 with a fresh DB and the test namespace recreated): log in, complete the forced password change, check the shell renders (activity bar; the Extensions view lists plugins), open a pods list, switch the language to Turkish in Settings and confirm visible text changes (shell, workloads list headers, a plugin page), switch to the light theme, open a YAML tab and confirm Monaco loads with no requests to CDNs (inspect network requests for jsdelivr/unpkg/cdnjs), log in as viewer1 and confirm mutation actions are hidden or disabled with the explanatory tooltip, mark the cluster production and confirm the status bar colour and the type-to-confirm dialog; collect console errors and page errors and save screenshots in ${SCRATCH}/w1-verify/. Fix real bugs you find, then re-run. Restore backend/internal/web/dist afterwards and delete the namespace.
6. Commit the integrated result on release/v0.5.0 in a few logical Conventional Commits (for example feat(backend): ..., feat(frontend): ..., feat(i18n): ..., test(backend): ...), each ending with the trailer line "${TRAILER}". Never commit scratch files, binaries, dist/ contents (backend/internal/web/dist keeps only .gitkeep) or node_modules, and do not touch .claude/. The tree must end clean (git status empty).
7. Final report: what you fixed, check results, e2e and browser results (with screenshot paths), the commits (sha + subject), and anything left.
`
const verify = await agent(VERIFY_PROMPT, { label: 'w1-integrate-verify', phase: 'Integrate', schema: VERIFY })
log(`Integrate done: ${verify ? verify.commits.length + ' commits' : 'FAILED'}`)

const pkg = await pkgPromise
log(`Packaging (worktree) done: ${pkg ? pkg.branch + ' with ' + pkg.commits.length + ' commits' : 'FAILED'}`)

phase('Packaging e2e')
let pkgE2E = null
if (pkg) {
  const PKG_E2E_PROMPT = `${COMMON}
YOUR ROLE: packaging end-to-end, agent "w1-pkg-e2e", in the MAIN working tree ${REPO} (branch release/v0.5.0). The v0.5 foundation (backend + frontend) has just been integrated and committed there. The packaging agent wrote the distribution scaffolding on branch feat/packaging (in its own worktree) but could only validate it statically and with stand-in binaries. You may edit any file, but keep fixes focused on packaging; justify any change outside the packaging files. Ports: 19878 (port-forward), 19879 (local container), 19880 (local binary).

Packaging agent report: ${JSON.stringify(pkg)}
Integrator report (for context): ${JSON.stringify(verify)}

Steps:
1. git status must be clean and the branch release/v0.5.0; if not, stop and report. Merge: git merge --no-ff feat/packaging -m "merge: packaging, CI and release scaffolding" -m "${TRAILER}". Resolve conflicts preserving the intent of both sides.
2. Run every check: npx tsc -b && npm run build && npm run lint && npm run i18n:check; cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./... ; helm lint --strict charts/kubebench; helm template + kubeconform -strict -summary; actionlint; shellcheck install.sh scripts/*.sh packaging/scripts/*.sh; goreleaser check. Restore backend/internal/web/dist to only .gitkeep after any embed step.
3. Real artifacts: make build -> bin/kubebench --version prints the version and commit; run it (PORT=19880, fresh DB in your scratch dir, no KUBECONFIG) and check that kubebench healthcheck exits 0 against it and 1 when it is stopped. goreleaser release --snapshot --clean --skip=publish (you may limit it to linux/amd64 via a temporary copy of the config in your scratch dir; keep the committed config complete). Test the real .deb in debian:trixie-slim (dpkg -i, kubebench --version, user/dirs/env file permissions, start the binary in the foreground as the kubebench user with the packaged env file and curl /healthz, dpkg -r, dpkg -P), the .rpm in fedora:44 the same way, and install.sh: --offline with the real tarball and with the real .deb, re-install (idempotent, env file preserved), --dry-run, --uninstall, --uninstall --purge. Build the offline bundle with scripts/offline-bundle.sh and inspect its contents and checksum.
4. Container image: build a LOCAL test image with docker build (tag kubebench:0.5.0-e2e; pass the VERSION/COMMIT/DATE build args). Never push it and never tag it for a registry. Check: it runs as nonroot, the HEALTHCHECK turns healthy (docker run -d -p 127.0.0.1:19879:9876, then docker inspect), /healthz answers, the image size is reasonable, --version shows the build args. Then: k3d image import kubebench:0.5.0-e2e -c kubebench-e2e.
5. Helm on the disposable cluster (namespace e2e-w1-pkg-helm): helm install with image.repository=kubebench, image.tag=0.5.0-e2e, image.pullPolicy=Never and the ingress enabled for a test host routed by Traefik; wait for Ready; verify through kubectl port-forward (127.0.0.1:19878) and through the ingress (curl http://127.0.0.1:18080 with the Host header): /healthz, login with the generated admin password (read it the way NOTES.txt says), the forced password change, /api/info shows the in-cluster cluster with capabilities, a pods list works. Then: helm upgrade keeps the same admin password secret (lookup); rbac.preset=read-only makes a write through the API fail with a Kubernetes 403 surfaced as code "kubernetes" while reads still work; probes, securityContext (readOnlyRootFilesystem with the /tmp emptyDir), PVC bound (local-path), Recreate strategy; networkPolicy.enabled=true still lets the pod reach the API server and serve requests (k3s enforces NetworkPolicies). helm uninstall; delete the namespace (and any cluster-scoped objects the chart created).
6. Plain manifests: kubectl apply -f deploy/kubernetes/kubebench.yaml with the image overridden to kubebench:0.5.0-e2e / pullPolicy Never (a temporary kustomization or a sed'ed copy in your scratch dir — do not edit the committed file for this), wait for Ready, /healthz through port-forward, then kubectl delete -f the same input and confirm everything is gone.
7. Fix every problem you find (mainly in the packaging files), re-run the failing steps, and commit the fixes as separate Conventional Commits ending with the trailer line "${TRAILER}". Update docs/deployment.md if testing showed it was wrong. Remove the local images and containers you created (docker rmi / docker rm); leave the k3d cluster running. The tree must end clean (git status empty) and backend/internal/web/dist must contain only .gitkeep.
8. Final report: merge commit, fix commits, every check with its result, the e2e results per step, and anything that still does not work.
`
  pkgE2E = await agent(PKG_E2E_PROMPT, { label: 'w1-packaging-e2e', phase: 'Packaging e2e', schema: PKG_E2E_REPORT })
} else {
  log('Packaging agent failed — skipping packaging e2e')
}

return { be, ui, platform, febCore, febPlugins, verify, pkg, pkgE2E }
````
