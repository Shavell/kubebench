# Workflow W2b — verbatim script

W2b features `helm` (Helm plugin v2, on `kubebench-e2e`), `argocd` (on
`kubebench-gitops` with a real Argo CD) and `storage` (on `kubebench-ops`
with the hostpath CSI driver and snapshots) in worktrees, in parallel with
an adversarial review of the W2a features; fixes `f801deb`..`2928ca0`;
merges `7f63741` (Argo CD), `c405d95` (Helm), `9510292` (Storage).

The script is reproduced verbatim. `SCRATCH` is the session's scratch
directory on the Fedora VM (it held the k3d kubeconfigs, the Playwright
install and agent scratch files) and must be replaced before the script is
run elsewhere; see [README.md](README.md#adapting-the-scripts-to-a-new-session).

````js
export const meta = {
  name: 'kubebench-v05-w2b',
  description: 'KubeBench v0.5 W2b: Helm v2, Argo CD and Storage plugins in worktrees; parallel adversarial review of the W2a features + fixes; serial merges with k3d smoke tests',
  phases: [
    { title: 'Implement', detail: 'one agent per feature in its own git worktree (branch feat/<id> from the release tip)' },
    { title: 'Review', detail: 'security review + correctness review of the W2a features (read-only)' },
    { title: 'Fix', detail: 'verify each finding independently, fix the real ones, commit' },
    { title: 'Merge', detail: 'serial merges into release/v0.5.0 after the fixes, full checks, k3d smoke tests' },
  ],
}

const SCRATCH = '/tmp/claude-0/-root-workspace-tmp-kube-bench/e3b76051-98cd-4ef5-963e-080ccedcb0b7/scratchpad'
const REPO = '/root/workspace/tmp/kube-bench'
const BASE = 'f801deb'
const GITOPS = SCRATCH + '/e2e/kubeconfig-gitops'
const E2E = SCRATCH + '/e2e/kubeconfig'
const OPS = SCRATCH + '/e2e/kubeconfig-ops'
const TRAILER = 'Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>'

const ENV = `ENVIRONMENT (Fedora 44 VM, you run as root):
- Start every shell with: export PATH=/opt/kubebench-tools/bin:$PATH (helm 3.22, kubectl 1.35, goreleaser v2, actionlint, kubeconform, nfpm). Also installed: go 1.27, node 24 + npm 11, docker, shellcheck, dpkg-deb, jq, python3, make.
- Disposable test clusters (k3d, k3s v1.35.5, metrics-server and local-path StorageClass in each):
  * "kubebench-e2e" — kubeconfig ${E2E} (API https://127.0.0.1:6550; nodes k3d-kubebench-e2e-server-0, -agent-0, -agent-1; Traefik at http://127.0.0.1:18080). Shared by several agents: namespaced work only.
  * "kubebench-ops" — kubeconfig ${OPS} (API https://127.0.0.1:6551; nodes k3d-kubebench-ops-server-0, -agent-0, -agent-1). Reserved for the storage feature and its merge (exclusive): it may install cluster-wide storage test infrastructure there (see its task).
  * "kubebench-gitops" — kubeconfig ${GITOPS} (API https://127.0.0.1:6552; nodes k3d-kubebench-gitops-server-0, -agent-0). A real Argo CD v3.6.1 runs in namespace argocd. Reserved for the argocd feature and its merge (exclusive).
- Playwright + Chromium (headless, working) live in ${SCRATCH}/pw. Reuse them; never install Playwright into the repo. In scripts: import { chromium } from '${SCRATCH}/pw/node_modules/playwright/index.mjs'

HARD RULES (on this machine they replace the cluster rules of contract §0):
- ~/.kube/config belongs to an UNRELATED external cluster. Never read it, never use it, never let a tool fall back to it: export KUBECONFIG to one of the k3d kubeconfig files above in EVERY shell before running kubectl or helm (helm reads the kubeconfig even for lint/template), and start every kubebench process with KUBECONFIG set to one of them (or none). Never register a cluster in KubeBench from any other kubeconfig.
- Never delete, stop or recreate the k3d clusters. Mutating requests are allowed only inside namespaces you create yourself, named e2e-<your-role>-<anything>, and on cluster-scoped test objects you create named e2e-<your-role>-<anything>; delete them when you are done. Node operations only where your task allows them, always undone at the end.
- Never push, never touch git remotes, never push or tag images for a registry, never deploy anywhere except the disposable k3d clusters.
- Every user-visible frontend string goes through i18n with complete en + tr entries (follow the Turkish glossary/tone already in src/i18n/locales/tr.json; Kubernetes nouns stay untranslated with apostrophe suffixes, polite "siz" form); npm run i18n:check must pass. Live mode never falls back to mock data (show Unavailable with the reason). Mock data is deterministic (seeded RNG; no Date.now()/Math.random() in generated data).
- Permissions: gate UI with usePermission/<Can> (write for mutations, exec for shell/debug/secret reveal) — disabled with the reason as tooltip, or hidden; destructive actions go through ConfirmDialog (type-to-confirm on production clusters). The backend enforces through the route access level: register every new route with the correct level (read/write/exec/admin); dry-run variants need read only (DryRunnable + a real dry run).
- Colors only via --vscode-* tokens (dark and light). Charts: single-series small multiples, no legends. TS: verbatimModuleSyntax, noUnusedLocals, erasableSyntaxOnly (no enums/parameter properties/namespaces). Layering: plugins never import other plugins or core/shell; cross-plugin navigation via openTab with tab kinds and showInPanel for bottom-panel views.
- Quality bar: production-grade, idiomatic Go and React, small focused files, meaningful tests, accurate comments, no dead code or fake "TODO" features. Complete the listed scope.
`

const IMPL_COMMON = `You are a senior engineer on KubeBench, a VSCode-style Kubernetes workbench (React 19 + TS + Vite frontend embedded in a single Go binary: client-go, Helm v3 SDK, modernc SQLite). You work in an ISOLATED git worktree (your current working directory) of the repo ${REPO}. The branch release/v0.5.0 already contains the whole v0.5 foundation plus the W2a features (WebSocket terminal + SSE logs, YAML apply/dry-run/diff/delete/secret reveal in the editor plugin, node/rollout/workload/CronJob/namespace operations): per-cluster roles, read-only clusters, audit, the backend plugin platform; plugin SDK v2, permissions, i18n (en/tr), the shared UI kit and locally bundled Monaco in the frontend; packaging (chart, deb/rpm, install.sh, manual CI).

FIRST STEPS:
1. Confirm you are inside a worktree: git rev-parse --show-toplevel must print a path under ${REPO}/.claude/worktrees/, NOT ${REPO} itself. Run git as plain commands from your worktree directory (no "cd X && git ..." chains; the harness refuses them).
2. The worktree was created from main, which is OLD. Create your branch from the release tip: git checkout -b feat/FEATURE_ID ${BASE} — then confirm with git log --oneline -3 that HEAD is ${BASE} ("fix(kube): give the debug target check its own name"). Never check out release/v0.5.0 itself (the main tree has it checked out).
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
    id: 'helm', title: 'Helm plugin v2', port: 19884, kubeconfig: E2E, cluster: 'kubebench-e2e',
    adapt: `YOUR FEATURE: F4 "helm" — Helm plugin v2. Read its spec in docs/dev/orchestration/feature-specs.md (section "## F4"); it extends contract §7.4. ADAPTATIONS FOR THIS RUN (they override the spec where they differ):
- The review fixer already changed the current Helm UI to identify releases by namespace AND name (openReleaseTab(ns, name), tab id helm-release:<ns>/<name>) and made requireHelm wait for the capability probe; keep both properties in the v2 plugin. Helm's own HTTP client still follows redirects (documented); keep that note accurate.
- The release data dir is $KUBEBENCH_DATA_DIR/helm (repositories.yaml, cache, registry config, uploads). Repository credentials must never be returned by any GET and must not appear in audit details.
- Live checks on kubebench-e2e (namespaces e2e-helm-*): add the HTTP repo podinfo (https://stefanprodan.github.io/podinfo) and search it; install podinfo from the repo (dry-run first: manifest + notes, nothing created), from OCI (oci://ghcr.io/stefanprodan/charts/podinfo with a version) and from an uploaded .tgz (helm pull it into your scratch dir first); upgrade with changed values (dry-run diff, then real), history, values per revision (user/all), manifest, rollback, uninstall with and without keep-history; repo update; a viewer gets 403 on install/upgrade/rollback/uninstall and on repo add (admin only) before any Helm call; every mutation is audited; the old /api/helm/* routes are gone and the frontend uses /api/plugins/helm. UI: mock flows plus a live flow against your binary: install wizard (search -> version -> values -> dry run -> install), upgrade with diff, rollback from history.`,
  },
  {
    id: 'argocd', title: 'Argo CD plugin', port: 19885, kubeconfig: GITOPS, cluster: 'kubebench-gitops',
    adapt: `YOUR FEATURE: F5 "argocd" — Argo CD plugin. Read its spec in docs/dev/orchestration/feature-specs.md (section "## F5"); it extends contract §7.5. ADAPTATIONS FOR THIS RUN (they override the spec where they differ):
- Your cluster kubebench-gitops runs a REAL Argo CD v3.6.1 (namespace argocd, apps-in-any-namespace NOT enabled, so Applications live in namespace argocd). Name every Application/AppProject/ApplicationSet you create e2e-argocd-* and every destination namespace e2e-argocd-*; delete them at the end (cascade). Sources: https://github.com/argoproj/argocd-example-apps.git (paths guestbook, helm-guestbook, kustomize-guestbook) — network access works.
- Verified on this cluster by the orchestrator: a sync triggered by patching .operation does NOT pick up spec.syncPolicy.syncOptions by itself (CreateNamespace=true was ignored until it was copied into operation.sync.syncOptions). Like the Argo CD API server/UI, copy the app's syncOptions (and the sync source/revision semantics) into the operation unless the caller overrides them; cover this with a test.
- Also verify the Unavailable state against kubebench-e2e (${E2E}), which has no Argo CD CRDs (read-only check).
- Live checks on kubebench-gitops: list/detail of apps with real status (sync, health, resources tree, conditions, history, operationState); sync (incl. prune, dry-run, a revision, selective resources) driven to Succeeded by the real controller; 409 while an operation runs; refresh and hard refresh; rollback to a history entry (and 409 when automated sync is on); delete with and without cascade (resources removed / kept); projects and ApplicationSets lists (create one simple ApplicationSet with a list generator); viewer 403 on sync/rollback/delete before any Kubernetes call; audit rows. UI: mock flows plus a live flow against your binary: apps grouped by project, open an app, sync dialog, history rollback, open a managed resource in KubeBench through a tab kind.`,
  },
  {
    id: 'storage', title: 'Storage plugin (PV/PVC/StorageClass creator)', port: 19886, kubeconfig: OPS, cluster: 'kubebench-ops',
    adapt: `YOUR FEATURE: F6 "storage" — PV/PVC/StorageClass creator. Read its spec in docs/dev/orchestration/feature-specs.md (section "## F6"); it extends contract §7.6. ADAPTATIONS FOR THIS RUN (they override the spec where they differ):
- Your cluster is kubebench-ops (${OPS}), reserved for you. To test snapshots, clones and expansion for real you MAY install there: the external-snapshotter CRDs + snapshot-controller and the csi-driver-host-path (kubernetes-csi) with a StorageClass (allowVolumeExpansion: true) and a VolumeSnapshotClass — from their official GitHub manifests at pinned release tags. Test both states: before installing (snapshots endpoint reports the CRDs missing; local-path PVC without expansion support refused) and after. Leave the installed CSI infrastructure in place at the end (the merge agent reuses it) but delete all your PVCs/PVs/StorageClasses/snapshots named e2e-storage-* and your namespaces.
- KubelizeMe (a reference product) recognises cloud CSI drivers (ebs.csi.aws.com, pd.csi.storage.gke.io, disk.csi.azure.com, driver.longhorn.io, rancher.io/local-path, kubernetes.io/no-provisioner) and gives provisioner-specific hints (e.g. type=gp3 for AWS EBS); add such hints for StorageClass parameters and in the CSI source of the PV wizard, as plain data in the plugin.
- Live checks: overview (default class marker, counts, capacity), CSI drivers list; PVC wizard create (dry-run validate first) and bind to an Available PV; a hostPath PV and a local PV with node affinity picked from the nodes; a CSI PV; a StorageClass; expand a hostpath-CSI PVC (and the refusals: smaller size, class without expansion); snapshot list, clone from PVC and restore from snapshot; server-side validation errors for bad names/quantities/access modes; viewer 403 on create/resize; audit rows. UI: mock flows plus a live flow against your binary for the PVC wizard with YAML preview -> Validate -> Create -> opening the new resource, and Expand.`,
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


const REVIEW_COMMON = `You are a reviewer for KubeBench v0.5 (React 19 + TS + Vite frontend embedded in a single Go binary: client-go, Helm v3 SDK, modernc SQLite). The W2a features were just merged and are committed on branch release/v0.5.0 in ${REPO}: the review scope is the change set git diff 24f29bb..${BASE} — the WebSocket terminal and SSE logs (F1), resource editing: apply/dry-run/diff/delete/metadata/secret reveal and the editor plugin (F2), node/rollout/workload/pod/CronJob/namespace operations (F3), plus the review fixes to the foundation made in the same range. Use git log/show/diff; the contract is docs/design-v0.5.md (§7.1-§7.3 and what the features recorded there); CLAUDE.md lists the rules; the specs are in docs/dev/orchestration/feature-specs.md.

READ-ONLY: do not edit, create or delete files in ${REPO} and do not change git state there. You may copy the repo or the backend into ${SCRATCH}/REVIEWER/ and write throwaway tests or scripts there to PROVE a finding (go test in the copy, curl/websocket clients against a binary you build there). If you run the binary, use port PORT, a fresh KUBEBENCH_DB in your scratch dir and KUBECONFIG=${E2E}, and only create namespaces named e2e-REVIEWER-*; delete them and stop the server afterwards. Do not run node operations.

${ENV}

How to report: only real, specific defects — no style nits, no "consider", no duplicates of the known limitations below. For each finding give the exact file and line, a concrete failure or attack scenario (inputs -> wrong behaviour), the evidence (code path walked end to end, or the output of your proof), severity, confidence, and a minimal fix. Prefer fewer, verified findings over many speculative ones.

KNOWN LIMITATIONS (do not report again): everything deferred or documented by the W1/W2a fixer and integrators — session tokens stored raw in SQLite (file is 0600); Helm's own client follows redirects; pasted kubeconfigs need KUBEBENCH_ALLOW_KUBECONFIG_EXEC for exec plugins; dry runs of apply by viewers are not audited (by design); a dry-run of a manifest creating a Namespace plus objects inside it reports the inner objects as failed (same as kubectl); a viewer refused on exec routes gets the generic "cannot change resources" message; a process ignoring Ctrl+C/Ctrl+D after hangup can keep running in the container (same as kubectl exec); no frontend unit-test harness; the per-user (not per-tab) active cluster; the 875 kB main chunk.
`

const REVIEW_SEC = REVIEW_COMMON.replaceAll('REVIEWER', 'review-sec').replaceAll('PORT', '19894') + `
YOUR LENS: SECURITY. Focus: GET /api/exec/ws (Origin enforcement, gate before upgrade, command allowlist, message size limits, idle/ping, resource exhaustion with many sessions, the exec.session audit row, what happens on a read-only flip or role change mid-session), GET /api/logs/stream (selector fan-out limits, viewer access to containers, stream lifetime, goroutine leaks), apply (multi-document limits, body size, kinds that escalate privileges — can an operator create ClusterRoleBindings or modify KubeBench's own namespace/ServiceAccount? is that acceptable under §3.1 since the stored credential decides? redacted-secret marker handling, force apply), generic delete (propagation, cluster-scoped objects, KubeBench's own namespace), metadata patch, secret reveal (exec access, audit, caching headers, never in logs), workload ops (drain/eviction scope, debug container image and target, CronJob trigger naming/injection, namespace delete protections incl. KUBEBENCH_NAMESPACE), the editor's handling of revealed values in the frontend (copy buttons, no persistence in layout storage/localStorage), XSS in YAML/log/terminal rendering.`

const REVIEW_CORR = REVIEW_COMMON.replaceAll('REVIEWER', 'review-corr').replaceAll('PORT', '19895') + `
YOUR LENS: CORRECTNESS and CONTRACT COMPLIANCE (not security). Focus: the terminal/logs protocol vs §7.1 (frames, exit codes, resize, reconnect, fallback executor, mock mode), the logs viewer (ring buffer, follow/pause, previous container, multi-pod workload-logs tab), apply semantics (created/configured/unchanged detection, namespace defaulting, cluster-scoped warnings, conflict code, dry-run), neat output, delete propagation, drain result shape and PDB handling, rollout history/rollback correctness (revision ordering, pod-template-hash removal, paused refusal), scale/restart for each kind, bulk delete limits, CronJob trigger job naming (<=63 chars), namespace create validation; frontend: provider mock/live parity and determinism, refreshData after mutations, the useQuery change made in the workload-ops merge (stale data vs loading states across all pages), permission gating of every new action (row menus, detail toolbars, palette, keyboard), unsaved-changes guard, i18n completeness and Turkish quality for the new strings (grep t('...') keys against the en bundles), leaks of EventSource/WebSocket/xterm instances when tabs close, light-theme tokens for the new CSS.`

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
Other W2b features (helm, argocd, storage) may already have been merged before you; check git log.

Steps:
1. Make sure the tree is clean and on release/v0.5.0 (git status; git branch --show-current). If not clean, stop and report.
2. git merge --no-ff feat/${f.id} -m "merge: ${f.title}" -m "${TRAILER}"
3. Resolve conflicts preserving the intent of BOTH sides (aggregator files: keep the union, keep ordering sane; the review fixes must survive). package.json conflicts: resolve package.json, then regenerate package-lock.json with npm install and verify npm ci works afterwards. go.mod/go.sum conflicts: resolve go.mod, then cd backend && go mod tidy. Locale JSON conflicts: merge the keys of both sides and keep valid JSON.
4. Run the full checks and fix every integration failure (types, duplicate routes or registrations, i18n parity, tests, layering): npx tsc -b && npm run build && npm run lint && npm run i18n:check; cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./... .
5. Smoke test on the k3d cluster ${f.cluster} (KUBECONFIG=${f.kubeconfig}): build the embedded binary in ${SCRATCH}/merge-${f.id}/ (npm run build; copy dist/* into backend/internal/web/dist/ keeping .gitkeep; go build -o ${SCRATCH}/merge-${f.id}/kubebench . in backend/), start it on port ${f.port + 50} with a fresh DB, log in (admin/admin, then POST /api/auth/password), exercise the feature's main endpoints INCLUDING its mutations inside a namespace e2e-merge-${f.id}-* that you create (for argocd create Applications in namespace argocd named e2e-merge-argocd-* and sync them through the real controller; for storage reuse the CSI infrastructure the implementer left on kubebench-ops), check the audit rows, check that a viewer is refused, and drive one main UI flow of the feature with Playwright against the binary. Also re-run a quick regression of the earlier merged features' main endpoints if any are present. Stop the server, delete the namespace and restore backend/internal/web/dist to only .gitkeep.
6. Commit the merge (and any fix-ups as separate "fix(...)" commits ending with the trailer line "${TRAILER}"). The tree must end clean.
Report in the structured format (merged=false only if you could not complete the merge; explain why).`
}

// Review the W2a features while the W2b features are implemented.
const reviewsPromise = parallel([
  () => agent(REVIEW_SEC, { label: 'review:security', phase: 'Review', schema: FINDINGS_SCHEMA }),
  () => agent(REVIEW_CORR, { label: 'review:correctness', phase: 'Review', schema: FINDINGS_SCHEMA }),
])

const fixPromise = reviewsPromise.then((reviews) => {
  const [sec, corr] = reviews
  const total = (sec ? sec.findings.length : 0) + (corr ? corr.findings.length : 0)
  log(`Review done: security=${sec ? sec.findings.length : 'FAILED'} correctness=${corr ? corr.findings.length : 'FAILED'} findings`)
  if (total === 0) return null
  const FIX_PROMPT = `You are the fixer for KubeBench v0.5 in the MAIN working tree ${REPO} (branch release/v0.5.0). Read CLAUDE.md and docs/design-v0.5.md. Feature agents are working in their own worktrees right now (branches feat/helm, feat/argocd, feat/storage); they will be merged after you, so keep your changes minimal and focused, and do not rename or move foundation APIs unless a fix requires it (if it does, say so clearly in your report). Leave .claude/ alone.

${ENV}
- Scratch space: ${SCRATCH}/w2b-fix/. Ports: 19896 (binary), 20896 (Vite) — no others.

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
