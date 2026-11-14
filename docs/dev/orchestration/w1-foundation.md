# Workflow W1 — v0.5 foundation (verbatim script)

This is the exact Workflow script that ran on 2026-11-14 and was **stopped mid-way**
(see `../v0.5-handoff.md` §4 for what each agent finished). It starts three agents
in the same working tree with disjoint file ownership (backend foundation, frontend
platform, i18n conversion) and then one integrate-and-verify agent.

To reuse it on another machine:

- Change `SCRATCH` to that session's scratch directory.
- Tell the build agents that **work already exists** in their area. They should continue from the WIP
  commits instead of starting over. Read the checklists in the handoff first.
- The rule "never mutate the homelab cluster" stays. On the new VM, mutation tests may target
  a disposable k3d/k3s/kind cluster (handoff §6).

````js
export const meta = {
  name: 'kubebench-v05-foundation',
  description: 'KubeBench v0.5 foundation: backend roles/audit/plugins, frontend platform, i18n conversion, then integrate and verify',
  phases: [
    { title: 'Build', detail: 'backend foundation, frontend platform, i18n conversion (parallel, disjoint file ownership)' },
    { title: 'Integrate', detail: 'contract alignment, full checks, e2e + browser smoke test, fixes' },
  ],
}

const SCRATCH = '/tmp/claude-1000/-home-ubuntu-tmp-kube-bench/a1be3ba8-9914-43f9-ab98-a84cbf922e15/scratchpad'

const COMMON = `You are a senior engineer on KubeBench, a VSCode-style Kubernetes workbench (React 19 + TS + Vite frontend embedded in a single Go binary: client-go, Helm SDK, modernc SQLite). Repo: /home/ubuntu/tmp/kube-bench (branch release/v0.5.0). We are building v0.5.0.

READ FIRST: docs/design-v0.5.md (the implementation contract; sections referenced below as §N) and CLAUDE.md. Then read the existing code you will change before changing it.

TEAM SETUP: Three agents work in THIS SAME working tree at the same time with strictly disjoint file ownership (yours is listed below).
- Only create/edit files you own. If you need a change elsewhere, do not make it; describe it in your final report under "requests" (file + exact change).
- Never run git commands that modify the index, working tree or branches (no add/commit/stash/checkout/reset/restore/clean/merge). Read-only git (status/diff/log/show) is fine. The orchestrator commits.
- Builds may transiently fail because of another agent's in-progress edits in files you do not own; if a failure is outside your files, wait and retry later, and judge your own work by the errors in your files.

HARD RULES (§0):
- The k3s homelab cluster in ~/.kube/config (context k3s-homelab) is READ-ONLY for you: never send any mutating request to it (create/update/patch/delete/evict/exec, not even dryRun). Use client-go fake clients for mutation tests.
- Never push, never touch git remotes, never build/push container images, never deploy anything.
- Every user-visible frontend string goes through i18n (en + tr); npm run i18n:check must pass. Live mode never falls back to mock data. Mock data stays deterministic (seeded RNG, no Date.now()/Math.random() in generated data). Colors only via --vscode-* tokens. TS config: verbatimModuleSyntax, noUnusedLocals, erasableSyntaxOnly (no enums, parameter properties or namespaces).
- Quality bar: production-grade, idiomatic, small focused files, comments where intent is not obvious, no dead code, no stub "TODO" features. The listed scope is expected to be completed.
- Scratch space for temporary files: ${SCRATCH} (create a subfolder for yourself). Never leave temp files in the repo.

TURKISH UI GLOSSARY (keep consistent across the app): Workloads=İş Yükleri, Overview=Genel Bakış, Details=Ayrıntılar, Events=Olaylar, Logs=Loglar, Settings=Ayarlar, Metrics=Metrikler, Users=Kullanıcılar, Extensions/Plugins=Eklentiler, Audit log=Denetim kaydı, Read-only=Salt okunur, Viewer=İzleyici, Operator=Operatör, Admin(istrator)=Yönetici, Sign in=Giriş yap, Sign out=Çıkış yap, Password=Parola, Username=Kullanıcı adı, Delete=Sil, Restart=Yeniden başlat, Scale=Ölçekle, Rollback=Geri al, Uninstall=Kaldır, Install=Kur, Upgrade=Yükselt, Values=Değerler, Revision=Revizyon, Repository=Depo, Theme=Tema, Language=Dil, Dark=Koyu, Light=Açık, System=Sistem, Enabled=Etkin, Disabled=Devre dışı, Available=Kullanılabilir, Unavailable=Kullanılamıyor. "Cluster" stays "Cluster" (plural "Cluster'lar"). Kubernetes/Helm nouns stay untranslated: Pod, Deployment, StatefulSet, DaemonSet, ReplicaSet, Job, CronJob, Node, Namespace, Service, Ingress, ConfigMap, Secret, PVC, PV, StorageClass, CRD, Helm, release, chart — with Turkish suffixes after an apostrophe (Pod'lar, Node'u, Namespace'ler). Polite "siz" form, sentence case.

FINAL ANSWER: return the structured report (summary, files changed, verification commands + results, requests for other owners, contract deviations, open issues).
`

const BE = `
YOUR ROLE: backend foundation (Go). You own backend/** exclusively. Frontend agents are implementing the client side of the same contract right now, so implement the contract exactly (paths, methods, field names, error codes).

Implement:
1. Build info (§1): version/commit/date vars in main.go; --version flag; "healthcheck" subcommand; GET /api/version (public).
2. Config (§2): new package internal/config that parses every env var of the table once, with validation and defaults (KUBEBENCH_ALLOW_PATH_KUBECONFIG defaults to true unless running in-cluster). main.go uses it; native TLS when cert+key are set; http.Server with ReadHeaderTimeout but no WriteTimeout (SSE/WebSocket arrive later); graceful shutdown on SIGINT/SIGTERM; background jobs: prune expired sessions hourly, prune audit rows daily per retention.
3. Store (internal/store): migrations driven by PRAGMA user_version (v1 = current schema, v2 = the §3/§4 additions: user_clusters.role, clusters.read_only, clusters.environment, users.must_change_password, audit_log, plugin_settings(id, enabled, updated_at), plugin_kv(plugin_id, key, value, updated_at)); open with _pragma=foreign_keys(1); cleanup semantics of §3.5 (deleting the active cluster resets users.active_cluster_id). The bootstrap admin gets must_change_password when the default password is used.
4. Permissions and router (§3): one central route-registration helper with access levels public/authed/read/write/exec/admin; effective permission computation (admin > operator > viewer; read-only cluster blocks write/exec for everyone incl. admins); must_change_password gating; JSON errors {error, code} everywhere (§3.3); audit middleware for write/exec/admin mutations (§3.4) whose ResponseWriter wrapper captures the status but still implements http.Flusher and http.Hijacker (SSE/WebSocket will rely on them); dry-run requests (dryRun=true) need only read and are not audited; Origin check for non-GET; login rate limiter (§3.5); Secure cookie per config (honour X-Forwarded-Proto only when KUBEBENCH_TRUST_PROXY); POST /api/auth/password which revokes the user's other sessions.
5. API changes (§4): /api/info v2 including the capability-probe wait (<=3 s); users v2 (list shape with assignments, PATCH, assignments PUT incl. legacy clusterIds, protections: no self-delete, keep at least one admin); clusters v2 (role/readOnly/environment in the list, PATCH /api/clusters/{id}, path gating); GET /api/audit; events kind filter; logs tail/previous/sinceSeconds; GET /api/k8s/{path...} read passthrough (§4.5: GET only; only /api/, /apis/, /version; reject secrets paths and watch=true; cap 16 MiB; pass status codes through); GET and PUT /api/plugins (§4.6) with a Detect-status cache (60 s per cluster+plugin) and the settings map; external plugin discovery from KUBEBENCH_PLUGINS_DIR (read plugin.json, validate the id regex and entry filename) and GET /plugins/{id}/{file} (authed; traversal-safe via os.DirFS/fs.Sub; correct Content-Type for js/css/json/svg/png; no directory listing). Keep every existing endpoint working with its current JSON unless the contract changes it.
6. Plugin platform (§5): package internal/plugin (types, registry, WriteJSON/WriteError/WriteK8sError helpers, a Router implementation bound to the central route helper so plugin routes get the same auth/role/audit/plugin-enabled/Detect gating, KV backed by plugin_kv) and package internal/plugins/all (blank-import aggregator; no plugins yet; a doc comment shows how to add one); main.go imports it.
7. kube.Runtime (§5): change CS to kubernetes.Interface and Metrics to the metrics clientset interface so tests can inject fakes; add a test constructor (e.g. kube.NewRuntimeForTest(cluster, cs, dyn) that starts no background work); capability-probe readiness (WaitProbed(ctx)); Discovery(); Mapper() (memory-cached discovery + deferred RESTMapper; reset and retry once on no-match); ResolveKind(apiVersion, kind) (gvr, namespaced, err); ResolveResource(group, resource).
8. Tests (go test ./...): migrations (fresh DB, and an upgrade from a v1 DB created with the old DDL that keeps data and maps assignments to operator), permission matrix (viewer write -> 403 forbidden_role, read-only cluster -> 403 cluster_read_only even for admin, admin routes -> 403 admin_required, must-change-password gating), audit rows for mutations (none for GET or dry-run), rate limiter, Origin check, plugin mounting with a fake plugin (disabled -> 404 plugin_disabled, unavailable -> 503 plugin_unavailable), passthrough rejecting secrets and watch, external plugin file serving vs traversal attempts, /api/version, user protections. Use httptest and fake clientsets.
9. Verify: cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./... . Then a live smoke test of the binary from ${SCRATCH}/w1-be/ (fresh KUBEBENCH_DB there, PORT=19876): first without KUBECONFIG (no cluster), then with KUBECONFIG=/home/ubuntu/.kube/config, using curl ONLY for: login, me, info, version, healthz, password change, users CRUD + assignments, clusters list + PATCH (readOnly/environment only change KubeBench's own DB), plugins list + toggle, audit list, GET resource endpoints, events with kind, passthrough (namespaces OK, secrets 403). Do NOT call endpoints that would mutate the cluster (pod DELETE, scale, restart, exec, helm rollback/uninstall), except to confirm that a viewer or a read-only cluster gets 403 BEFORE any Kubernetes call happens (prove that from the code path first). Stop the server when done.
`

const FEA = `
YOUR ROLE: frontend platform. Implement the client side of docs/design-v0.5.md §1 (frontend version), §3/§4 (session, permissions, users/clusters/audit/plugins/password APIs) and §6.2-§6.8. The Go backend for these endpoints is being written right now by another agent from the same contract; code against the contract and do not run the Go backend.

You own: src/core/** EXCEPT the shell components the i18n agent converts (src/core/shell/ActivityBar.tsx, SideBar.tsx, CommandPalette.tsx, TabStrip.tsx, BottomPanel.tsx, EditorArea.tsx, LoginScreen.tsx and their .css files). You DO own src/core/shell/Shell.tsx, StatusBar.tsx (+css) and any NEW file in src/core/shell; all of src/data/**; NEW files in src/shared/** (do not edit existing shared files: the i18n agent converts them); src/plugins/settings/**, src/plugins/clusters/**, src/plugins/users/**, NEW src/plugins/extensions/** and src/plugins/audit/**, src/plugins/index.ts, src/plugins/terminal/YamlEditorTab.tsx (only this file of the terminal plugin); src/main.tsx, index.html, vite.config.ts, tsconfig*.json, package.json, package-lock.json, NEW examples/**.
Strings in files you own: core-level UI goes into a new i18n namespace "platform" (src/core/locales/en.json + tr.json, registered with registerTranslations('platform', ...) from a module that main.tsx imports); plugin UI goes into the plugin's own namespace (src/plugins/<id>/locales/{en,tr}.json + manifest.locales). Do NOT edit src/i18n/locales/*.json (owned by the i18n agent), but you may use its "core" keys (common.*).

Implement:
1. src/data/apiClient.ts (§6.2) and refactor src/data/api/index.ts to use it, with every parameter encoded; the events call passes kind.
2. Boot (§6.3): mock-mode policy in main.tsx (mock only in DEV, with ?demo, or VITE_DEMO_MODE=true; a production build with an unreachable/5xx backend shows a BootError screen with Retry); parse /api/info v2 into dataMode (version, commit, cluster, permissions, features, mustChangePassword); mustChangePassword -> forced ChangePasswordScreen (then reload); 401 -> the existing LoginScreen.
3. Live mode never uses mock data: helm/crd/exec/metrics currently fall back to mock in src/data/api — replace with errors (ApiError status 503 code capability_unavailable, or the metric source error) so pages show the reason; no generateSeries in live mode.
4. Permissions UI (Can/usePermissions from src/core/permissions.ts, which you may extend) in your files: users, clusters, settings, status bar. Status bar (§6.3): environment colouring (production red / staging amber / development green via new theme tokens), a "Read-only" item, a user menu (Change password dialog, Sign out), version item and an About command.
5. Plugin SDK v2 (§6.4): extend PluginManifest and the registry (version, description, category, core, backendPlugin, resourceActions, settingsSections), a pluginStatusStore (zustand) loaded from GET /api/plugins at boot (mock: all enabled/available); the registry filters disabled/unavailable plugins out of activity bar, sidebar, commands and resource actions; registry.resourceActionsFor(kind). Mark the existing core plugins core: true. Keep existing registry method names/signatures stable (the i18n agent's shell components call them) and do the filtering inside the registry.
6. src/data/pluginData.ts (§6.5): pluginApi, definePluginData, usePluginQuery (sharing the DataContext invalidation bus), usePluginStatus.
7. Extensions plugin (core): activity bar entry, sidebar list, detail tab; admin enable/disable (PUT /api/plugins/{id}); shows version, category, builtin/external, availability reason and external-plugin load errors.
8. Users plugin v2: global role + per-cluster assignment roles (viewer/operator) editor, create user with role, reset password (PATCH), change global role, delete with ConfirmDialog. Clusters plugin v2: environment/read-only/your-role badges, admin edit dialog (name, environment, read-only) via PATCH, kubeconfig path field only when features.pathKubeconfig. Audit plugin (admin only, registered at runtime like users): table (time, user, cluster, action, target, status, detail), filters (user, cluster, action), "Load more" pagination.
9. Shared UI (§6.6, new files in src/shared): Dialog, ConfirmDialog (type-to-confirm when production or requested), Form primitives (Field, TextInput, NumberInput, Select, Checkbox, Textarea, KeyValueEditor), notify + Toaster (mounted in Shell.tsx), Tabs, Unavailable, editor/CodeEditor + editor/DiffEditor with Monaco bundled locally (loader.config({ monaco }) using the monaco-editor package, workers via Vite ?worker imports, lazy-loaded with React.lazy so the main chunk stays small, theme follows the app theme). YamlEditorTab uses CodeEditor. Verify the production build contains no CDN references (grep dist for jsdelivr/unpkg/cdnjs).
10. Themes (§6.7): complete light token set in src/core/theme/theme.css for every token, settings theme dark|light|system with <html data-theme>, Monaco follows. src/shared/charts/chartTheme.ts is an existing shared file owned by the i18n agent: if it hardcodes colours, list the needed change under requests.
11. Settings page: theme, language (SUPPORTED_LANGUAGES from '@/i18n' plus 'auto', using setLanguagePreference), existing settings, plugin-contributed settingsSections; all strings via i18n.
12. External plugins (§6.8): loader after /api/plugins (enabled external entries -> <script> with a 5 s timeout; failures recorded for the Extensions view); window.KubeBench global SDK (src/core/plugin/globalSdk.ts) with the listed surface; a hand-written typed declaration file for plugin authors; example plugin examples/plugins/hello-cluster (package.json, vite.config.ts library IIFE build with react and react/jsx-runtime mapped to KubeBench globals, plugin.json, src/index.tsx registering an activity bar entry + tab that lists namespaces through api.k8sGet('/api/v1/namespaces'), en/tr locales, README with build/install steps). Exclude examples from the app tsconfig/build; build the example once (npm install + build in its folder) to prove it works; add a .gitignore there so node_modules/dist are never committed.
13. Define __APP_VERSION__ in vite.config.ts from package.json (declare it in a .d.ts) and set package.json version to 0.5.0.
Verify: npx tsc -b && npm run build && npm run lint && npm run i18n:check; run the dev server in mock mode (npx vite --port 15173 --strictPort, background, then stop it) and curl it; confirm the bundle has no CDN references.
`

const FEB = `
YOUR ROLE: internationalize the existing UI (English + Turkish) and gate mutation UI by permission. The i18n infrastructure already exists (read src/i18n/index.ts and §6.1).

You own: src/i18n/locales/en.json + tr.json (namespace "core"; extend them); src/core/shell/ActivityBar.tsx, SideBar.tsx, CommandPalette.tsx, TabStrip.tsx, BottomPanel.tsx, EditorArea.tsx, LoginScreen.tsx (+ their css); every EXISTING file in src/shared/** (Badge, ConfirmButton, EmptyState, EventsTable, Icon, ListFilterBar, ResizeHandle, Table, Toolbar, Tree, contextMenu, charts/*); and the plugins src/plugins/workloads/**, src/plugins/metrics/**, src/plugins/crd/**, src/plugins/helm/**, src/plugins/terminal/** except YamlEditorTab.tsx. Another agent (frontend platform) concurrently owns src/core/plugin/**, src/core/store/**, src/data/**, Shell.tsx, StatusBar.tsx, src/plugins/{settings,clusters,users,extensions,audit}, src/plugins/index.ts, main.tsx and NEW shared files; do not edit those.

Implement:
1. Every user-visible string in your files goes through t() (react-i18next useTranslation(ns)). Shared/shell strings use the "core" namespace. Each of your plugins gets src/plugins/<id>/locales/en.json + tr.json and a manifest "locales" field (import the JSON files); manifest display strings (name, activityBar.tooltip, sidebar.title, command titles, bottom panel titles) become keys in the plugin namespace (e.g. "manifest.name", "commands.openReleases").
2. Shell components render manifest strings with useManifestText() from '@/i18n'.
3. Cover column headers (resourceCatalog), toolbar buttons, empty states, error banners, confirm prompts, context menu items (rowActions), tooltips, placeholders, aria-labels, unit labels, chart titles and the login screen. Plurals via _one/_other suffixes. Numbers/dates shown to users use src/i18n/format.ts where it makes sense (compact ages like "5m" may stay).
4. Permission gating with usePermissions/can/<Can> from src/core/permissions.ts in your files: delete/restart/scale/rollback/uninstall/edit need canWrite; shell/exec needs canExec. Hide context-menu items and disable buttons with an explanatory i18n tooltip that distinguishes "cluster is read-only" from "your role is viewer".
5. Complete, natural Turkish translations following the glossary; npm run i18n:check must pass (key parity + interpolation variables).
6. Do not change behaviour beyond i18n and gating; keep mock data (event messages, log lines, resource names) in English.
Verify: npx tsc -b && npm run build && npm run lint && npm run i18n:check (failures caused by the platform agent's in-progress files: retry later, report if persistent). Then grep your files for leftover user-visible English literals (JSX text nodes and title=/placeholder=/aria-label= props with Capitalized words) and fix them.
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

const VERIFY = {
  type: 'object',
  properties: {
    summary: { type: 'string' },
    fixes: { type: 'array', items: { type: 'string' } },
    checks: {
      type: 'array',
      items: {
        type: 'object',
        properties: { command: { type: 'string' }, result: { type: 'string' } },
        required: ['command', 'result'],
      },
    },
    e2e: { type: 'string' },
    browser: { type: 'string' },
    remainingIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['summary', 'fixes', 'checks', 'e2e', 'browser', 'remainingIssues'],
}

phase('Build')
const [be, fea, feb] = await parallel([
  () => agent(COMMON + BE, { label: 'backend-foundation', phase: 'Build', schema: REPORT }),
  () => agent(COMMON + FEA, { label: 'frontend-platform', phase: 'Build', schema: REPORT }),
  () => agent(COMMON + FEB, { label: 'i18n-conversion', phase: 'Build', schema: REPORT }),
])
log(`Build done: backend=${be ? 'ok' : 'FAILED'} platform=${fea ? 'ok' : 'FAILED'} i18n=${feb ? 'ok' : 'FAILED'}`)

phase('Integrate')
const VERIFY_PROMPT = `${COMMON}
YOUR ROLE: integrator and verifier for the v0.5 foundation. The three build agents (backend foundation, frontend platform, i18n conversion) have FINISHED, so you may now edit ANY file. Still never modify git state (the orchestrator commits), never mutate the homelab cluster, never push.

Their reports (JSON; null means the agent failed — then inspect its area yourself and finish what is missing):
BACKEND: ${JSON.stringify(be)}
PLATFORM: ${JSON.stringify(fea)}
I18N: ${JSON.stringify(feb)}

Tasks:
1. Apply the valid "requests" from the reports; resolve listed contract deviations (fix the side that deviates from docs/design-v0.5.md, or if the contract is wrong, fix docs/design-v0.5.md and say so).
2. Contract alignment: compare backend handlers (backend/internal/api/**, backend/internal/plugin/**) with the frontend callers (src/data/**, plugins using pluginApi) for every endpoint of §3-§4: paths, methods, query/body field names, response shapes, error codes. Fix mismatches.
3. Run everything and fix all failures: npx tsc -b; npm run build; npm run lint (pre-existing only-export-components warnings are fine, new errors are not); npm run i18n:check; cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./... .
4. End-to-end smoke test in ${SCRATCH}/w1-verify/: build the embedded binary (npm run build; copy dist/* into backend/internal/web/dist/ keeping .gitkeep; go build -o ${SCRATCH}/w1-verify/kubebench .), run it on PORT=19877 with a fresh KUBEBENCH_DB in that folder and KUBECONFIG=/home/ubuntu/.kube/config (READ-ONLY use of the cluster). With curl: GET / and one JS asset; login admin/admin; GET /api/nodes -> 403 password_change_required; POST /api/auth/password; /api/info (permissions, capabilities true, cluster); /api/plugins; create user viewer1 and assign role viewer on the cluster; log in as viewer1 (forced password change, since the admin set the password); GET /api/pods OK; DELETE /api/pods?ns=x&name=y -> 403 forbidden_role (first prove from the code that the role check runs before any Kubernetes call); as admin PATCH the cluster readOnly=true; an admin write attempt -> 403 cluster_read_only (same proof first); PATCH readOnly=false; GET /api/audit lists the mutations. Afterwards restore backend/internal/web/dist to contain only .gitkeep and stop the server.
5. Browser smoke test (time-box about 20 minutes; skip with a note if Playwright/Chromium cannot be installed): install Playwright + Chromium inside ${SCRATCH}/w1-verify/pw (NOT in the repo; e.g. npm init -y && npm i playwright && npx playwright install chromium; system deps with sudo npx playwright install-deps chromium if needed). Against the binary (rebuild/restart it as in step 4 with a fresh DB): log in, complete the forced password change, check the shell renders (activity bar; the Extensions view lists plugins), open a pods list, switch the language to Turkish in Settings and confirm visible text changes, switch to the light theme, open a YAML tab and confirm Monaco loads with no requests to CDNs (inspect network requests for jsdelivr/unpkg/cdnjs), collect console errors and page errors, save screenshots in the scratch folder. Fix real bugs you find, then re-run. Restore backend/internal/web/dist afterwards.
6. Final report: what you fixed, check results, e2e and browser results (with screenshot paths), and anything left.
`
const verify = await agent(VERIFY_PROMPT, { label: 'integrate-verify', phase: 'Integrate', schema: VERIFY })
return { be, fea, feb, verify }
````
