# Workflow W2a (first run) — verbatim script

W2a features `terminal`, `resource-editing` and `workload-ops`, each in
its own git worktree and branch `feat/<id>`, in parallel with a read-only
security review and correctness review of the W1 foundation; then verify
and fix the findings and merge the branches serially with k3d smoke tests
(`kubebench-e2e` and, for node operations, `kubebench-ops`).

The session restarted while it ran; [`w2a-resume.md`](w2a-resume.md)
finished it.

The script is reproduced verbatim. `SCRATCH` is the session's scratch
directory on the Fedora VM (it held the k3d kubeconfigs, the Playwright
install and agent scratch files) and must be replaced before the script is
run elsewhere; see [README.md](README.md#adapting-the-scripts-to-a-new-session).

````js
export const meta = {
  name: 'kubebench-v05-w2a',
  description: 'KubeBench v0.5 W2a: terminal, resource-editing, workload-ops in worktrees; parallel adversarial review of the W1 foundation + fixes; serial merges with k3d smoke tests',
  phases: [
    { title: 'Implement', detail: 'one agent per feature in its own git worktree (branch feat/<id> from the release tip)' },
    { title: 'Review', detail: 'security review + correctness review of the W1 foundation (read-only)' },
    { title: 'Fix', detail: 'verify each finding independently, fix the real ones, commit' },
    { title: 'Merge', detail: 'serial merges into release/v0.5.0 after the fixes, full checks, k3d smoke tests' },
  ],
}

const SCRATCH = '/tmp/claude-0/-root-workspace-tmp-kube-bench/e3b76051-98cd-4ef5-963e-080ccedcb0b7/scratchpad'
const REPO = '/root/workspace/tmp/kube-bench'
const BASE = '24f29bb'
const E2E = SCRATCH + '/e2e/kubeconfig'
const OPS = SCRATCH + '/e2e/kubeconfig-ops'
const TRAILER = 'Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>'

const ENV = `ENVIRONMENT (Fedora 44 VM, you run as root):
- Start every shell with: export PATH=/opt/kubebench-tools/bin:$PATH (helm 3.22, kubectl 1.35, goreleaser v2, actionlint, kubeconform, nfpm). Also installed: go 1.27, node 24 + npm 11, docker, shellcheck, dpkg-deb, jq, python3, make.
- Disposable test clusters (k3d, k3s v1.35.5, metrics-server and local-path StorageClass in both):
  * "kubebench-e2e" — kubeconfig ${E2E} (API https://127.0.0.1:6550; nodes k3d-kubebench-e2e-server-0, -agent-0, -agent-1; Traefik at http://127.0.0.1:18080). Shared by several agents: namespaced work only.
  * "kubebench-ops" — kubeconfig ${OPS} (API https://127.0.0.1:6551; nodes k3d-kubebench-ops-server-0, -agent-0, -agent-1). Reserved for node operations (cordon/drain/taints) of the workload-ops feature and its merge.
- Playwright + Chromium (headless, working) live in ${SCRATCH}/pw. Reuse them; never install Playwright into the repo. In scripts: import { chromium } from '${SCRATCH}/pw/node_modules/playwright/index.mjs'

HARD RULES (on this machine they replace the cluster rules of contract §0):
- ~/.kube/config belongs to an UNRELATED external cluster. Never read it, never use it, never let a tool fall back to it: export KUBECONFIG to one of the two k3d kubeconfig files above in EVERY shell before running kubectl or helm (helm reads the kubeconfig even for lint/template), and start every kubebench process with KUBECONFIG set to one of them (or none). Never register a cluster in KubeBench from any other kubeconfig.
- Never delete, stop or recreate the k3d clusters. Mutating requests are allowed only inside namespaces you create yourself, named e2e-<your-role>-<anything>, and on cluster-scoped test objects you create named e2e-<your-role>-<anything>; delete them when you are done. Node operations only where your task allows them, always undone at the end.
- Never push, never touch git remotes, never push or tag images for a registry, never deploy anywhere except the disposable k3d clusters.
- Every user-visible frontend string goes through i18n with complete en + tr entries (follow the Turkish glossary/tone already in src/i18n/locales/tr.json; Kubernetes nouns stay untranslated with apostrophe suffixes, polite "siz" form); npm run i18n:check must pass. Live mode never falls back to mock data (show Unavailable with the reason). Mock data is deterministic (seeded RNG; no Date.now()/Math.random() in generated data).
- Permissions: gate UI with usePermission/<Can> (write for mutations, exec for shell/debug/secret reveal) — disabled with the reason as tooltip, or hidden; destructive actions go through ConfirmDialog (type-to-confirm on production clusters). The backend enforces through the route access level: register every new route with the correct level (read/write/exec/admin); dry-run variants need read only (DryRunnable + a real dry run).
- Colors only via --vscode-* tokens (dark and light). Charts: single-series small multiples, no legends. TS: verbatimModuleSyntax, noUnusedLocals, erasableSyntaxOnly (no enums/parameter properties/namespaces). Layering: plugins never import other plugins or core/shell; cross-plugin navigation via openTab with tab kinds and showInPanel for bottom-panel views.
- Quality bar: production-grade, idiomatic Go and React, small focused files, meaningful tests, accurate comments, no dead code or fake "TODO" features. Complete the listed scope.
`

const IMPL_COMMON = `You are a senior engineer on KubeBench, a VSCode-style Kubernetes workbench (React 19 + TS + Vite frontend embedded in a single Go binary: client-go, Helm v3 SDK, modernc SQLite). You work in an ISOLATED git worktree (your current working directory) of the repo ${REPO}. The branch release/v0.5.0 already contains the whole v0.5 foundation: per-cluster roles, read-only clusters, audit, the backend plugin platform; plugin SDK v2, permissions, i18n (en/tr), the shared UI kit and locally bundled Monaco in the frontend; packaging (chart, deb/rpm, install.sh, manual CI).

FIRST STEPS:
1. Confirm you are inside a worktree: git rev-parse --show-toplevel must print a path under ${REPO}/.claude/worktrees/, NOT ${REPO} itself. Run git as plain commands from your worktree directory (no "cd X && git ..." chains; the harness refuses them).
2. The worktree was created from main, which is OLD. Create your branch from the release tip: git checkout -b feat/FEATURE_ID ${BASE} — then confirm with git log --oneline -3 that HEAD is ${BASE} ("docs(deployment): correct the Helm upgrade and uninstall commands"). Never check out release/v0.5.0 itself (the main tree has it checked out).
3. npm ci (node_modules is per worktree; Go module and build caches are shared).
4. Read docs/design-v0.5.md (the contract; §N references), CLAUDE.md, your feature spec (see YOUR FEATURE below) and the foundation code you build on:
   backend — internal/api/routes.go (the route table: one handle() per route with its access level; auditKinds), router.go (authenticate, authorize, audit; call.rt is the caller's active-cluster Runtime), permissions.go, audit.go, respond.go; internal/plugin (Access, DryRunnable/IsDryRun, Router, Context, KV, WriteJSON/WriteError/WriteK8sError); internal/plugins/all; internal/kube (Runtime: CS kubernetes.Interface, Dyn, Metrics, Mapper(), ResolveKind/ResolveResource, RawGet, NewRuntimeForTest; Manager).
   frontend — src/data/apiClient.ts; src/data/providers.ts, types.ts, api/*, mock/*; src/data/pluginData.ts (pluginApi, definePluginData, usePluginQuery, usePluginStatus); src/core/plugin (types, registry, resourceActions + ResourceActionButtons, panelRequests.showInPanel, globalSdk, sdk/kubebench.d.ts); src/core/permissions.ts (usePermission, <Can>, deniedReason); src/shared (Dialog, ConfirmDialog, form/*, notify + Toaster, Tabs, Unavailable, ErrorNotice, Page, editor/CodeEditor + editor/DiffEditor, Table, Toolbar, EmptyState, Badge, Button, contextMenu, tabPage.css); src/i18n (+ format.ts).
   Reuse these; do not re-invent them.

PARALLEL WORK: other agents implement other features in their own worktrees right now, and a fixer may add small review fixes to release/v0.5.0 meanwhile; every branch is merged into release/v0.5.0 afterwards by an integrator. Keep merges painless: keep changes focused on your feature and prefer new files; in shared aggregator files (src/plugins/index.ts, backend/internal/plugins/all/all.go, backend/internal/api/routes.go, src/data/providers.ts, src/data/types.ts, src/data/api/*.ts, src/data/mock/index.ts, locale JSON files, package.json) make minimal append-only additions — never reorder, reformat or rename existing code there. In locale JSON files put your keys in a group named after your feature where that makes sense.

${ENV}
- Scratch space: ${SCRATCH}/FEATURE_ID/ (never leave temp files in the repo). Ports: backend PORT, Vite dev server PORT+1000 — never others.

VERIFY before finishing (all must pass):
- npx tsc -b && npm run build && npm run lint && npm run i18n:check (the two pre-existing only-export-components warnings are acceptable)
- cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./...
- The feature checks listed in YOUR FEATURE (unit tests with fakes, and live checks on the k3d cluster assigned to you with a binary built from your tree: npm run build, copy dist/* into backend/internal/web/dist/ keeping .gitkeep, go build -o ${SCRATCH}/FEATURE_ID/kubebench . in backend/, run it with a fresh KUBEBENCH_DB in your scratch dir, PORT=PORT and KUBECONFIG set to your cluster's kubeconfig; first login is admin/admin and requires POST /api/auth/password before other calls; restore backend/internal/web/dist to only .gitkeep afterwards).
- UI checks: in mock mode (npx vite --port <PORT+1000> --strictPort) AND against your binary in live mode, drive the feature's main flows with Playwright headlessly, collect console and page errors, save screenshots under your scratch dir, and stop every server you started.

COMMIT: commit on feat/FEATURE_ID in a few logical Conventional Commits (feat(<scope>): ..., test(...), etc.), each ending with the trailer line "${TRAILER}". Leave the worktree clean (git status empty; backend/internal/web/dist only .gitkeep). Do NOT merge into release/v0.5.0.

FINAL ANSWER: the structured report (branch, worktree path, commits, summary, files touched, verification commands + results, merge notes about shared files you touched, contract changes, open issues).
`

const FEATURES = [
  {
    id: 'terminal', title: 'Terminal & logs (WebSocket PTY, SSE logs)', port: 19881, kubeconfig: E2E, cluster: 'kubebench-e2e',
    adapt: `YOUR FEATURE: F1 "terminal" — Terminal & logs. Read its spec in docs/dev/orchestration/feature-specs.md (section "## F1"); it extends contract §7.1. ADAPTATIONS FOR THIS RUN (they override the spec where they differ):
- Audit action: the route table cannot override the audit action name yet (integrator note). Add a small, general mechanism next to auditKinds in routes.go/audit.go (e.g. an auditActions map or a route option) so that GET /api/exec/ws records exactly one row with action "exec.session" and target pod/<ns>/<pod>/<container> at upgrade time (the audit writer already writes rows at hijack time). Refused upgrades (viewer, read-only cluster, bad Origin) must be rejected before the upgrade and before any Kubernetes call.
- Origin: reuse origin.go (it already covers WebSocket upgrades, X-Forwarded-Host only with KUBEBENCH_TRUST_PROXY); a missing Origin is allowed only for non-browser clients, as the spec says.
- Bottom panel: the integrator replaced terminalStore.ts with the core channel showInPanel(kind, props) (src/core/plugin/panelRequests.ts): bottom panel tabs declare the kinds they handle; the terminal panels handle pod-logs and pod-exec. Build the multi-session terminal and the new logs viewer on that channel; keep the pod-exec and pod-logs tab kinds and props {namespace, pod, container} stable (the workload-ops feature opens pod-exec for debug containers).
- Do NOT touch src/plugins/terminal/YamlEditorTab.tsx (the resource-editing feature owns the YAML tab).
- Live checks on the k3d cluster kubebench-e2e (namespace e2e-terminal-*): SSE logs for a kube-system pod (curl -N for about 5 s) and with selector=k8s-app=kube-dns; a real exec session through the WebSocket into a pod you create (a small ws client script in your scratch dir): run a command, resize, read the exit code; idle/ping behaviour where practical; a viewer gets 403 on /api/exec/ws with no Kubernetes call; exactly one exec.session audit row per session. UI: mock flows, plus a live flow against your binary that opens a shell sub-tab on a real pod, runs a command and sees its output, and opens the logs viewer with follow on.`,
  },
  {
    id: 'resource-editing', title: 'YAML apply / dry-run / diff / delete / labels / secret reveal', port: 19882, kubeconfig: E2E, cluster: 'kubebench-e2e',
    adapt: `YOUR FEATURE: F2 "resource-editing" — YAML apply, dry-run, diff, delete. Read its spec in docs/dev/orchestration/feature-specs.md (section "## F2"); it extends contract §7.2. ADAPTATIONS FOR THIS RUN (they override the spec where they differ):
- The YAML tab is src/plugins/terminal/YamlEditorTab.tsx (it already uses the shared CodeEditor; GET /api/yaml now returns apiVersion/kind for typed objects too). You own it and the "yaml" tab kind. You may move the YAML tab and the new "New resource" tab into a new core plugin folder (e.g. src/plugins/editor/, core: true) if that keeps the terminal plugin focused; keep the tab kind name "yaml" and its props backwards compatible. The terminal feature agent was told not to touch YamlEditorTab.tsx.
- Access is per route pattern, so a query parameter cannot raise the access level: secret reveal needs its own route with access exec (for example GET /api/resource/reveal or POST /api/secrets/reveal) and it must produce an audit row even though it does not mutate — check how the router decides what to audit and make reveal audited explicitly. Record the final shape in docs/design-v0.5.md §7.2. Same for apply: dryRun=true must be checked as read and not audited (DryRunnable + a real server-side dry run).
- Generic delete and the labels/annotations editor hook into the workloads plugin rows and detail pages: add new files for the dialogs and keep the hook lines in rowActions/detail toolbars minimal (the workload-ops feature edits the same plugin in parallel).
- Live checks on the k3d cluster kubebench-e2e (namespace e2e-resource-editing-*): GET /api/yaml?neat=true for several kinds; a multi-document apply (first created, then configured, then unchanged); a dry-run that creates nothing; an SSA conflict (a field owned by another field manager, e.g. set with kubectl apply --server-side --field-manager=other) reported with code conflict, then a force apply; delete with each propagation policy; the metadata patch (add/remove labels and annotations); secret reveal (exec; audited; viewer 403); redacted-secret rejection. UI: mock flows, plus a live flow against your binary: Edit YAML -> Review diff -> Dry run -> Apply, New resource from a template, Delete with propagation.`,
  },
  {
    id: 'workload-ops', title: 'Node, rollout, workload, pod, CronJob, namespace operations', port: 19883, kubeconfig: OPS, cluster: 'kubebench-ops',
    adapt: `YOUR FEATURE: F3 "workload-ops" — node and workload operations. Read its spec in docs/dev/orchestration/feature-specs.md (section "## F3"); it extends contract §7.3. ADAPTATIONS FOR THIS RUN (they override the spec where they differ):
- Your live cluster is "kubebench-ops" (kubeconfig ${OPS}), reserved for you during implementation. You may cordon, drain and taint its AGENT nodes (k3d-kubebench-ops-agent-0, -agent-1) — never the server node — and you must uncordon them and remove your taints before you finish. Work in namespaces e2e-workload-ops-*.
- The resource-editing feature adds a generic delete and a labels editor to the same workloads plugin in parallel: put your dialogs/actions in new files and keep the hook lines in rowActions/detail toolbars minimal.
- The debug container dialog opens the pod-exec tab kind for the new ephemeral container (props {namespace, pod, container}); the terminal feature is replacing the shell implementation behind that tab kind in parallel, so only rely on the tab kind contract.
- Live checks on kubebench-ops: history + rollback of a Deployment with three revisions (and the refusals: paused, current revision); pause/resume; restart and scale for StatefulSet, DaemonSet and ReplicaSet; force delete and bulk delete; a debug (ephemeral) container added and running (verify with kubectl); CronJob trigger (Job named <cj>-manual-xxxxx with the annotation and ownerRef) and suspend; namespace create and delete, including the refusal for system namespaces and KubeBench's own namespace; cordon/uncordon; a drain of k3d-kubebench-ops-agent-1 where a PodDisruptionBudget blocks one pod (the result must list it) and DaemonSet pods are ignored; the taints editor; a viewer gets 403 on cordon before any Kubernetes call; every mutation produces an audit row. UI: mock flows, plus a live flow against your binary for the drain dialog with results and the rollout history with rollback.`,
  },
]

const IMPL_SCHEMA = {
  type: 'object',
  properties: {
    branch: { type: 'string' },
    worktreePath: { type: 'string' },
    commits: { type: 'array', items: { type: 'string' } },
    summary: { type: 'string' },
    filesTouched: { type: 'array', items: { type: 'string' } },
    verification: { type: 'array', items: { type: 'object', properties: { command: { type: 'string' }, result: { type: 'string' } }, required: ['command', 'result'] } },
    mergeNotes: { type: 'string' },
    contractChanges: { type: 'array', items: { type: 'string' } },
    openIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['branch', 'worktreePath', 'commits', 'summary', 'filesTouched', 'verification', 'mergeNotes', 'contractChanges', 'openIssues'],
}

const FINDINGS_SCHEMA = {
  type: 'object',
  properties: {
    scope: { type: 'string' },
    summary: { type: 'string' },
    findings: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          severity: { type: 'string', enum: ['critical', 'high', 'medium', 'low'] },
          confidence: { type: 'string', enum: ['high', 'medium', 'low'] },
          category: { type: 'string' },
          file: { type: 'string' },
          line: { type: 'integer' },
          title: { type: 'string' },
          scenario: { type: 'string' },
          evidence: { type: 'string' },
          suggestedFix: { type: 'string' },
        },
        required: ['id', 'severity', 'confidence', 'category', 'file', 'line', 'title', 'scenario', 'evidence', 'suggestedFix'],
      },
    },
  },
  required: ['scope', 'summary', 'findings'],
}

const FIX_SCHEMA = {
  type: 'object',
  properties: {
    summary: { type: 'string' },
    dispositions: {
      type: 'array',
      items: {
        type: 'object',
        properties: {
          id: { type: 'string' },
          status: { type: 'string', enum: ['fixed', 'rejected', 'deferred'] },
          reason: { type: 'string' },
        },
        required: ['id', 'status', 'reason'],
      },
    },
    commits: { type: 'array', items: { type: 'string' } },
    checks: { type: 'array', items: { type: 'object', properties: { command: { type: 'string' }, result: { type: 'string' } }, required: ['command', 'result'] } },
    openIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['summary', 'dispositions', 'commits', 'checks', 'openIssues'],
}

const MERGE_SCHEMA = {
  type: 'object',
  properties: {
    merged: { type: 'boolean' },
    mergeCommit: { type: 'string' },
    conflicts: { type: 'array', items: { type: 'string' } },
    fixes: { type: 'array', items: { type: 'string' } },
    checks: { type: 'array', items: { type: 'object', properties: { command: { type: 'string' }, result: { type: 'string' } }, required: ['command', 'result'] } },
    smoke: { type: 'string' },
    openIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['merged', 'mergeCommit', 'conflicts', 'fixes', 'checks', 'smoke', 'openIssues'],
}

const REVIEW_COMMON = `You are a reviewer for KubeBench v0.5 (React 19 + TS + Vite frontend embedded in a single Go binary: client-go, Helm v3 SDK, modernc SQLite). The W1 foundation was just finished and is committed on branch release/v0.5.0 in ${REPO}: the review scope is the change set git diff dc4b30f..${BASE} (use git log/show/diff; the contract is docs/design-v0.5.md; CLAUDE.md lists the rules). Feature work (terminal, resource editing, workload operations) will be built on top of this foundation, so flaws found now are cheap to fix.

READ-ONLY: do not edit, create or delete files in ${REPO} and do not change git state there. You may copy the repo or the backend into ${SCRATCH}/REVIEWER/ and write throwaway tests or scripts there to PROVE a finding (go test in the copy, curl against a binary you build there). If you run the binary, use port PORT, a fresh KUBEBENCH_DB in your scratch dir and KUBECONFIG=${E2E}, and only create namespaces named e2e-REVIEWER-*; delete them and stop the server afterwards.

${ENV}

How to report: only real, specific defects — no style nits, no "consider", no duplicates of the listed known limitations below. For each finding give the exact file and line, a concrete failure or attack scenario (inputs -> wrong behaviour), the evidence (code path walked end to end, or the output of your proof), severity (critical/high/medium/low), confidence, and a minimal fix. Prefer fewer, verified findings over many speculative ones; if you could not prove something, say so in the evidence and lower the confidence.

KNOWN LIMITATIONS already recorded by the integrator (do not report them again): the active cluster is per user rather than per browser tab; refused admin requests on /{id} routes use the path id as audit target; a runtime rebuild resets the metrics sample history; Helm still uses legacy /api/helm/* routes until W2b; uncached Detect calls run sequentially in GET /api/plugins; low test coverage in the old kube code; the 875 kB main chunk; Monaco's own UI language is fixed per page load; the inline theme script in index.html and a future CSP; external plugins load for other users only on their next page load; server-only plugin names are not translated; showInPanel is not in the external SDK yet; the bare deb/rpm starting with admin/admin on :9876 (documented product decision); the chart's generated admin password is not forced to change; the plain manifests rendered for a second namespace reuse ClusterRoleBinding "kubebench"; helm package timestamps; no LICENSE yet.
`

const REVIEW_SEC = REVIEW_COMMON.replaceAll('REVIEWER', 'review-sec').replaceAll('PORT', '19891') + `
YOUR LENS: SECURITY. Scope: backend/main.go, bootstrap.go, healthcheck.go, jobs.go; backend/internal/api/** (router.go, routes.go, permissions.go, audit.go, auditlog.go, origin.go, ratelimit.go, auth.go, users.go, clusters.go, info.go, passthrough.go, plugins.go, pluginrouter.go, detect.go, extplugins.go, resources.go, helmcrd.go, metrics.go, respond.go); backend/internal/store/**; backend/internal/kube/{kubeconfig.go,raw.go,manager.go,runtime.go,discovery.go}; backend/internal/plugin/**; src/core/boot/**, src/core/session/**, src/core/plugin/{externalPlugins.ts,globalSdk.ts}, src/data/apiClient.ts; charts/kubebench/**, deploy/kubernetes/**, install.sh, packaging/**, Dockerfile, .github/workflows/**.
Look in particular for:
- authentication and sessions: token entropy and storage, session fixation, cookie flags per KUBEBENCH_SECURE_COOKIES/TRUST_PROXY, revocation on password change/user delete/role change, timing-safe comparisons, password hashing cost, the bootstrap admin and must-change-password flow (can any read/write route be reached before the change?), login rate limiting and its keying (X-Forwarded-For spoofing without TRUST_PROXY, unbounded memory);
- authorization: every route's access level in routes.go vs what the handler does; the gate order; plugin routes; methods (HEAD, OPTIONS, method override headers); path tricks (double slashes, encoded slashes %2F, trailing slashes, dot segments, case) against the mux and the gates; per-user active cluster switching to a cluster the user is not assigned to; users acting on other users; last-admin and self-delete protections;
- the Kubernetes passthrough GET /api/k8s/{path...}: any way to read Secret data (case/encoding tricks, subresources, /api/v1/namespaces/x/secrets, label/field selectors, List/Table formats, other resources that embed secret data such as ServiceAccount token requests or configmaps are fine but secrets via /apis/... paths are not), any way to reach a non-GET verb or watch, response size caps, error bodies leaking data;
- CSRF: the Origin check for every non-GET route and WebSocket upgrade, missing Origin, "null" Origin, X-Forwarded-Host handling, GET routes with side effects;
- audit: mutations or exec that are not audited, attacker-controlled content making audit rows misleading, audit rows for refused requests;
- external plugin files GET /plugins/{id}/{file}: traversal, symlinks, content-type and X-Content-Type-Options, serving files outside the plugin dir, plugin.json validation, which users can load disabled plugins' files;
- cluster inventory: adding clusters by server-side path (KUBEBENCH_ALLOW_PATH_KUBECONFIG gating), kubeconfigs with exec credential plugins or auth-provider commands (would let an admin run commands on the KubeBench host — is that acceptable, documented, gated?), kubeconfigs referencing local files (bootstrap embeds referenced cert files — can a user-supplied kubeconfig make the server read arbitrary local files and expose them?), SSRF to internal addresses;
- secret leakage in logs, error messages, /api/info, audit details; body size limits and other DoS vectors (JSON bodies, multipart, slowloris given no ReadTimeout);
- frontend: XSS (dangerouslySetInnerHTML, unescaped HTML in toasts/markdown, links with javascript: URLs), the external plugin loader (only same-origin /plugins/<id>/ URLs? what if plugin.json entry is crafted?), what the window.KubeBench SDK exposes;
- packaging: chart RBAC presets and the read-only preset really excluding secrets, NetworkPolicy, generated password handling (lookup), securityContext; install.sh (temp files, checksum verification before use, password handling/echoing, file permissions, curl|bash failure modes); the systemd unit hardening; GitHub workflows (script injection through workflow_dispatch inputs used inside run:, permissions, pull_request_target misuse).`

const REVIEW_CORR = REVIEW_COMMON.replaceAll('REVIEWER', 'review-corr').replaceAll('PORT', '19892') + `
YOUR LENS: CORRECTNESS and CONTRACT COMPLIANCE (not security). Scope: the whole change set, with emphasis on backend/internal/api, backend/internal/kube (runtime.go, manager.go, discovery.go, raw.go), backend/internal/store, backend/main.go + bootstrap.go + jobs.go, and the frontend src/core/boot, src/core/session, src/data/**, src/core/plugin/**, src/core/permissions.ts, src/shared (the new components), src/plugins/{users,clusters,audit,extensions,settings}, the i18n bundles and the permission gating in src/plugins/**.
Look in particular for:
- contract mismatches with docs/design-v0.5.md §1-§6 that the integrator missed (request/response shapes, status and error codes, defaults of the §2 variables);
- concurrency: the Manager rebuilding a runtime while requests use the old one (use-after-Close), the Detect cache, the capability probe and WaitProbed, the sampler goroutines and their shutdown, data races (run go test -race on the backend packages in your copy), SQLite busy/locking behaviour under concurrent requests, graceful shutdown that cuts or hangs requests;
- resource leaks: goroutines, HTTP response bodies, timers, informers, runtimes of deleted clusters;
- store/migrations: upgrade edge cases (a v1 DB with odd data), the audit prune and pagination (nextBefore off-by-one, filters combined), cascades;
- frontend state bugs: boot sequence races (/api/info vs /api/plugins vs external plugins), stale closures, effects with missing dependencies, subscriptions not cleaned up, unhandled promise rejections, error states that never show, the registry filtering (disabled/unavailable plugins still reachable through commands, tabs restored from persisted layout, resource actions), permission gating gaps (a mutation reachable by a viewer or on a read-only cluster through any UI path — row menus, detail toolbars, command palette, keyboard shortcuts);
- i18n: hardcoded user-visible strings left, wrong plural handling, interpolation mistakes, Turkish strings that are wrong or unnatural, keys used in code but missing in the bundles (the check script only compares bundles with each other — grep t('...') keys against en.json);
- shared components: Dialog focus trap/restore and Esc handling, ConfirmDialog type-to-confirm edge cases, Form controls, notify/Toaster lifecycles, CodeEditor/DiffEditor theme and disposal, light theme tokens missing for some --vscode-* variables used in CSS (grep var(--vscode-...) usages against both theme blocks);
- mock determinism violations (Date.now()/Math.random() in mock data generation).`

function implPrompt(f) {
  return IMPL_COMMON.replaceAll('FEATURE_ID', f.id).replaceAll('PORT', String(f.port)) + `\n${f.adapt}\n`
}

function mergePrompt(f, impl, fix) {
  return `You are the integrator for KubeBench v0.5 in the MAIN working tree ${REPO} (branch release/v0.5.0). Read CLAUDE.md and docs/design-v0.5.md. Leave .claude/ (agent worktrees) alone and do not delete feature branches or worktrees (the orchestrator cleans up).

${ENV}
- Scratch space: ${SCRATCH}/merge-${f.id}/. Ports: ${f.port + 50} (binary), ${f.port + 1050} (Vite) — no others.

Merge the feature branch feat/${f.id} (${f.title}).
Implementer's report: ${JSON.stringify(impl)}
Before the feature merges, a fixer applied review fixes to the foundation on release/v0.5.0 (report; null = no fixes): ${JSON.stringify(fix)}
Other W2a features (terminal, resource-editing, workload-ops) may already have been merged before you; check git log.

Steps:
1. Make sure the tree is clean and on release/v0.5.0 (git status; git branch --show-current). If not clean, stop and report.
2. git merge --no-ff feat/${f.id} -m "merge: ${f.title}" -m "${TRAILER}"
3. Resolve conflicts preserving the intent of BOTH sides (aggregator files: keep the union, keep ordering sane; the review fixes must survive). package.json conflicts: resolve package.json, then regenerate package-lock.json with npm install and verify npm ci works afterwards. go.mod/go.sum conflicts: resolve go.mod, then cd backend && go mod tidy. Locale JSON conflicts: merge the keys of both sides and keep valid JSON.
4. Run the full checks and fix every integration failure (types, duplicate routes or registrations, i18n parity, tests, layering): npx tsc -b && npm run build && npm run lint && npm run i18n:check; cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./... .
5. Smoke test on the k3d cluster ${f.cluster} (KUBECONFIG=${f.kubeconfig}): build the embedded binary in ${SCRATCH}/merge-${f.id}/ (npm run build; copy dist/* into backend/internal/web/dist/ keeping .gitkeep; go build -o ${SCRATCH}/merge-${f.id}/kubebench . in backend/), start it on port ${f.port + 50} with a fresh DB, log in (admin/admin, then POST /api/auth/password), exercise the feature's main endpoints INCLUDING its mutations inside a namespace e2e-merge-${f.id}-* that you create (for workload-ops also cordon/uncordon an agent node and undo it), check the audit rows, check that a viewer is refused, and drive one main UI flow of the feature with Playwright against the binary. Also re-run a quick regression of the earlier merged features' main endpoints if any are present. Stop the server, delete the namespace and restore backend/internal/web/dist to only .gitkeep.
6. Commit the merge (and any fix-ups as separate "fix(...)" commits ending with the trailer line "${TRAILER}"). The tree must end clean.
Report in the structured format (merged=false only if you could not complete the merge; explain why).`
}

// Review the foundation while the features are implemented.
const reviewsPromise = parallel([
  () => agent(REVIEW_SEC, { label: 'review:security', phase: 'Review', schema: FINDINGS_SCHEMA }),
  () => agent(REVIEW_CORR, { label: 'review:correctness', phase: 'Review', schema: FINDINGS_SCHEMA }),
])

const fixPromise = reviewsPromise.then((reviews) => {
  const [sec, corr] = reviews
  const total = (sec ? sec.findings.length : 0) + (corr ? corr.findings.length : 0)
  log(`Review done: security=${sec ? sec.findings.length : 'FAILED'} correctness=${corr ? corr.findings.length : 'FAILED'} findings`)
  if (total === 0) return null
  const FIX_PROMPT = `You are the fixer for KubeBench v0.5 in the MAIN working tree ${REPO} (branch release/v0.5.0). Read CLAUDE.md and docs/design-v0.5.md. Feature agents are working in their own worktrees right now (branches feat/terminal, feat/resource-editing, feat/workload-ops); they will be merged after you, so keep your changes minimal and focused, and do not rename or move foundation APIs unless a fix requires it (if it does, say so clearly in your report). Leave .claude/ alone.

${ENV}
- Scratch space: ${SCRATCH}/w2a-fix/. Ports: 19893 (binary), 20893 (Vite) — no others.

Two independent reviewers produced the findings below (JSON). Reviewers can be wrong: for EACH finding, verify it yourself first — walk the code path end to end and, where feasible, reproduce it with a failing test or a curl/Playwright proof. Reject false positives with a short reason. For confirmed findings, fix the root cause with a regression test where sensible; keep the contract (docs/design-v0.5.md) in sync if a fix changes documented behaviour. Defer (with a reason) only what needs a product decision or is out of proportion; never silently drop a finding.

SECURITY REVIEW: ${JSON.stringify(sec)}
CORRECTNESS REVIEW: ${JSON.stringify(corr)}

Then run every check: npx tsc -b && npm run build && npm run lint && npm run i18n:check; cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test -race ./... && go build ./... ; and, if you changed chart/packaging files, helm lint --strict charts/kubebench, actionlint, shellcheck. Commit the fixes on release/v0.5.0 in logical Conventional Commits (fix(api): ..., fix(ui): ..., fix(chart): ..., test(...): ...), each ending with the trailer line "${TRAILER}". The tree must end clean (backend/internal/web/dist only .gitkeep).
Report every finding's disposition (fixed / rejected / deferred, with the reason), the commits and the checks.`
  return agent(FIX_PROMPT, { label: 'fix:review-findings', phase: 'Fix', schema: FIX_SCHEMA })
})

// Merges must not overlap (shared index/working tree) and must start after the fixes.
let mergeChain = fixPromise.then(() => undefined, () => undefined)
function serialized(fn) {
  const run = mergeChain.then(fn)
  mergeChain = run.then(() => undefined, () => undefined)
  return run
}

const results = await pipeline(
  FEATURES,
  (f) => agent(implPrompt(f), { label: `impl:${f.id}`, phase: 'Implement', isolation: 'worktree', schema: IMPL_SCHEMA }),
  (impl, f) => {
    if (!impl) {
      log(`impl:${f.id} failed — skipping its merge`)
      return null
    }
    log(`impl:${f.id} done (${impl.commits.length} commits) — queued for merge`)
    return serialized(() => fixPromise.then((fix) => agent(mergePrompt(f, impl, fix), { label: `merge:${f.id}`, phase: 'Merge', schema: MERGE_SCHEMA })))
      .then((merge) => ({ feature: f.id, impl, merge }))
  },
)

const reviews = await reviewsPromise
const fix = await fixPromise
return { reviews, fix, results }
````
