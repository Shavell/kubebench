# Workflow W3b — verbatim script

W3b: the `cloud` feature (EKS, AKS, GKE) in a worktree with a parallel
review of the W3a features (fixes `ecb80e8`..`401009f`), the cloud merge
`4b13777`, the German UI language (`ee287d6`, `30ed65f`, two translators in
parallel) and the documentation set (README EN/TR, CONTRIBUTING, SECURITY,
CHANGELOG, `docs/*`, CLAUDE.md, this handoff update).

The script is reproduced verbatim. `SCRATCH` is the session's scratch
directory on the Fedora VM (it held the k3d kubeconfigs, the Playwright
install and agent scratch files) and must be replaced before the script is
run elsewhere; see [README.md](README.md#adapting-the-scripts-to-a-new-session).

````js
export const meta = {
  name: 'kubebench-v05-w3b',
  description: 'KubeBench v0.5 W3b: EKS/AKS/GKE cloud support in a worktree with a parallel review of W3a + fixes, merge, then German UI language and the documentation set',
  phases: [
    { title: 'Implement', detail: 'cloud provider support in its own worktree' },
    { title: 'Review', detail: 'security + correctness review of the W3a features (read-only)' },
    { title: 'Fix', detail: 'verify each finding independently, fix the real ones, commit' },
    { title: 'Merge', detail: 'merge the cloud branch, full checks, k3d smoke test' },
    { title: 'German', detail: 'add the de language to every locale bundle (two translators in parallel)' },
    { title: 'Docs', detail: 'README EN/TR, CONTRIBUTING, CHANGELOG, docs/*, CLAUDE.md, handoff' },
  ],
}

const SCRATCH = '/tmp/claude-0/-root-workspace-tmp-kube-bench/e3b76051-98cd-4ef5-963e-080ccedcb0b7/scratchpad'
const REPO = '/root/workspace/tmp/kube-bench'
const BASE = 'ecb80e8'
const GITOPS = SCRATCH + '/e2e/kubeconfig-gitops'
const E2E = SCRATCH + '/e2e/kubeconfig'
const OPS = SCRATCH + '/e2e/kubeconfig-ops'
const TRAILER = 'Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>'

const ENV = `ENVIRONMENT (Fedora 44 VM, you run as root):
- Start every shell with: export PATH=/opt/kubebench-tools/bin:$PATH (helm 3.22, kubectl 1.35, goreleaser v2, actionlint, kubeconform, nfpm). Also installed: go 1.27, node 24 + npm 11, docker, shellcheck, dpkg-deb, jq, python3, make.
- Disposable test clusters (k3d, k3s v1.35.5, metrics-server and local-path StorageClass in each):
  * "kubebench-e2e" — kubeconfig ${E2E} (API https://127.0.0.1:6550; nodes k3d-kubebench-e2e-server-0, -agent-0, -agent-1; Traefik at http://127.0.0.1:18080). Shared by several agents: namespaced work only.
  * "kubebench-ops" — kubeconfig ${OPS} (API https://127.0.0.1:6551; nodes k3d-kubebench-ops-server-0, -agent-0, -agent-1). Has the hostpath CSI driver and snapshot controller installed. Not used in this wave except read-only.
  * "kubebench-gitops" — kubeconfig ${GITOPS} (API https://127.0.0.1:6552; nodes k3d-kubebench-gitops-server-0, -agent-0). A real Argo CD v3.6.1 runs in namespace argocd. Read-only use in this wave.
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

const IMPL_COMMON = `You are a senior engineer on KubeBench, a VSCode-style Kubernetes workbench (React 19 + TS + Vite frontend embedded in a single Go binary: client-go, Helm v3 SDK, modernc SQLite). You work in an ISOLATED git worktree (your current working directory) of the repo ${REPO}. The branch release/v0.5.0 already contains the whole v0.5 foundation plus the W2a/W2b/W2c/W3a features (live updates over SSE with scoped query topics, UX polish with a kubectl-like palette and lazy-loaded plugin pages, the GitHub update check + What's new (§7.11); Security posture, Access/RBAC and Topology plugins too; WebSocket terminal + SSE logs, YAML apply/dry-run/diff/delete/secret reveal in the editor plugin, node/rollout/workload/CronJob/namespace operations, the Helm v2, Argo CD and Storage backend+frontend plugins under internal/plugins/<id> and src/plugins/<id> — use them as the reference for how a backend plugin with its own frontend plugin is built): per-cluster roles, read-only clusters, audit, the backend plugin platform; plugin SDK v2, permissions, i18n (en/tr), the shared UI kit and locally bundled Monaco in the frontend; packaging (chart, deb/rpm, install.sh, manual CI).

FIRST STEPS:
1. Confirm you are inside a worktree: git rev-parse --show-toplevel must print a path under ${REPO}/.claude/worktrees/, NOT ${REPO} itself. Run git as plain commands from your worktree directory (no "cd X && git ..." chains; the harness refuses them).
2. The worktree was created from main, which is OLD. Create your branch from the release tip: git checkout -b feat/FEATURE_ID ${BASE} — then confirm with git log --oneline -3 that HEAD is ${BASE} ("fix(ui): declare live-update topics for the namespace picker, Events view and recent warnings"). Never check out release/v0.5.0 itself (the main tree has it checked out).
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
    id: 'cloud', title: 'Cloud provider support (EKS, AKS, GKE)', port: 19900, kubeconfig: E2E, cluster: 'kubebench-e2e',
    adapt: `YOUR FEATURE: F13 "cloud" — NEW, requested by the product owner: "EKS, AKS and GKE support; take tricks from KubelizeMe". There is no spec yet: write it into docs/design-v0.5.md (new §7.12 Cloud providers) and docs/dev/orchestration/feature-specs.md (## F13) first, then implement it.
What KubelizeMe does (reference product, minified bundle in /root/workspace/tmp/kubelizeme_extracted/assets/index-*.js — grep "eks.amazonaws.com", "get_version_support", "Cloud provider detection"): it detects the provider from the API server URL (".eks.amazonaws.com" -> eks, ".azmk8s.io" -> aks, "container.googleapis.com"/GKE endpoints -> gke, ".k8s.ondigitalocean.com" -> doks, ".ovh.net" -> ovh, linode), shows a provider badge, and checks the Kubernetes version against the provider's support window (end-of-life warning). Adapt that, do not copy code.
Requirements:
- Detection (backend, read): provider from the server URL, refined by node spec.providerID (aws://, azure://, gce://, digitalocean://, linode://) and well-known labels (eks.amazonaws.com/nodegroup, kubernetes.azure.com/cluster, cloud.google.com/gke-nodepool, k3s/rke2/openshift markers); plus the server version. Exposed in GET /api/clusters entries and /api/info cluster ({provider, version}); cached per runtime, refreshed on rebuild.
- Version support: a bundled, dated data table (upstream Kubernetes and EKS/AKS/GKE standard-support end dates for the recent minor versions, with its source URLs and "as of" date in a comment; easy to update) -> {status: supported|ending-soon (<90 days)|end-of-life|unknown, endDate}. Frontend: provider badge (text badge, no brand logos) in the Clusters view, the cluster switcher and the status bar; an end-of-life warning on the cluster overview.
- Credentials without exec plugins: kubeconfigs exported by the clouds use exec plugins (aws eks get-token / aws-iam-authenticator, gke-gcloud-auth-plugin, kubelogin). Since SEC-05 those need KUBEBENCH_ALLOW_KUBECONFIG_EXEC when pasted (and the binaries are not in the image anyway). Implement in-process token providers recognised from those exec stanzas and used INSTEAD of running any command (so no KUBEBENCH_ALLOW_KUBECONFIG_EXEC needed): EKS — AWS SDK v2 default credential chain (env, shared profile named in the exec env/args, web identity/IRSA, EC2/ECS metadata) and a presigned STS GetCallerIdentity URL with the x-k8s-aws-id header -> "k8s-aws-v1." token (same algorithm as aws-iam-authenticator; cache until ~1 min before expiry; region from the args/env); GKE — golang.org/x/oauth2/google default credentials (GOOGLE_APPLICATION_CREDENTIALS, workload identity, metadata server), scope cloud-platform; AKS — azidentity (DefaultAzureCredential or the --login mode of kubelogin: azurecli is NOT available, support spn/msi/workloadidentity/env) for the AKS AAD server application scope (6dae42f8-4368-4678-94ff-3960e28e3630/.default or the --server-id from the args). The credentials are the KubeBench server's own identity (pod identity in-cluster: IRSA, GKE Workload Identity, Azure Workload Identity — document the chart's serviceAccount.annotations for each) or per-cluster credentials an admin supplies in the Add/Edit cluster dialog (AWS access key/secret/session token or profile, GCP service-account JSON, Azure tenant/client id/secret) stored with the cluster record like the kubeconfig and never returned by any GET (write-only fields; audit details must not contain them).
- Security: only admins add/edit clusters (unchanged). Document in docs/design-v0.5.md §7.12 and the deployment docs that an admin can make the server use its ambient cloud identity for any cluster they add.
- Tests: unit tests for detection (URLs, providerIDs, labels), the version table logic, kubeconfig exec-stanza recognition (aws/aws-iam-authenticator/gke-gcloud-auth-plugin/kubelogin variants, unknown commands still refused without the flag), the EKS token format against a fixed signer/clock (decode the presigned URL: host sts.<region>.amazonaws.com, Action=GetCallerIdentity, X-Amz-Expires=60, signed header x-k8s-aws-id), and a fake token source for GKE/AKS. No real cloud account is available: say so; live checks on kubebench-e2e only for detection (k3s), version status, the UI badges, and the Add-cluster dialog's credential fields (stored, never returned, audit clean).`,
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


const REVIEW_COMMON = `You are a reviewer for KubeBench v0.5 (React 19 + TS + Vite frontend embedded in a single Go binary: client-go, Helm v3 SDK, modernc SQLite). The W3a features were just merged and are committed on branch release/v0.5.0 in ${REPO}: the review scope is the change set git diff f08489a..${BASE} — live updates (F10: watch hub, GET /api/watch SSE, scoped query topics), UX polish (F11: palette commands, shortcuts, virtualization, lazy pages), the update check + What's new (F12, contract §7.11: internal/updates, store migration v3, /api/updates, PUT /api/settings/updates, lastSeenVersion), plus the review fixes to the W2c features in the same range. Use git log/show/diff; the contract is docs/design-v0.5.md (§7.10-§7.11 and what the features recorded there); CLAUDE.md lists the rules; the specs are in docs/dev/orchestration/feature-specs.md.

READ-ONLY: do not edit, create or delete files in ${REPO} and do not change git state there. You may copy the repo or the backend into ${SCRATCH}/REVIEWER/ and write throwaway tests or scripts there to PROVE a finding (go test in the copy, curl/websocket clients against a binary you build there). If you run the binary, use port PORT, a fresh KUBEBENCH_DB in your scratch dir and KUBECONFIG=${E2E} (for Argo CD read-only checks you may also use ${GITOPS}; for storage read-only checks ${OPS}), and only create namespaces named e2e-REVIEWER-* (in kubebench-e2e only); delete them and stop the server afterwards. Do not run node operations.

${ENV}

How to report: only real, specific defects — no style nits, no "consider", no duplicates of the known limitations below. For each finding give the exact file and line, a concrete failure or attack scenario (inputs -> wrong behaviour), the evidence (code path walked end to end, or the output of your proof), severity, confidence, and a minimal fix. Prefer fewer, verified findings over many speculative ones.

KNOWN LIMITATIONS (do not report again): core lists are not served from informer caches; the RBAC polling fallback has unit tests only; the main chunk is still ~671 kB; palette tabs opened with -n keep a plain-string title; the palette's completion lookups declare no topics; lastSeenVersion is not validated as SemVer; a 404 from the releases API is treated as 'no releases'; no frontend unit-test runner; the rbac-defaults label marks a binding as system (product decision pending); the Markdown export does not escape bare URLs; the What's new dialog is modal at first sign-in (by design).
`

const REVIEW_SEC = REVIEW_COMMON.replaceAll('REVIEWER', 'review-sec').replaceAll('PORT', '19894') + `
YOUR LENS: SECURITY. Focus: GET /api/watch (authorization per topic and namespace — can a viewer learn about objects/namespaces they could not list? revocation, subscriber/topic caps, write deadlines, informer resource use per cluster, goroutine leaks), the informer credentials (whose RBAC is used), the update checker (SSRF via KUBEBENCH_UPDATE_URL is config-only — fine — but response size limits, JSON parsing limits, TLS, proxy handling, release notes rendering in the frontend: the markdown renderer must not allow HTML/javascript: links), PUT /api/settings/updates and POST /api/updates/check gating and audit, PATCH /api/auth/me (what else can a user change about themselves?), the store migration v3, the palette commands (exec/describe/logs respecting permissions; injection into URLs/qs), CHANGELOG bundling.`

const REVIEW_CORR = REVIEW_COMMON.replaceAll('REVIEWER', 'review-corr').replaceAll('PORT', '19895') + `
YOUR LENS: CORRECTNESS and CONTRACT COMPLIANCE (not security). Focus: live updates vs §7.10 (debounce, topic mapping for every page/plugin, fallback to polling, Live/Polling/Offline states, reconnect with backoff, informer lifecycle on runtime rebuild and cluster switch, multiple tabs), the updates feature vs §7.11 (SemVer comparison incl. prereleases and v-prefix, modes, caching/ETag, error states, What's new once per user per version, changelog parsing of Keep a Changelog sections, mock determinism), UX polish (palette parsing of kubectl-like commands, aliases, shortcuts conflicts with Monaco/xterm, virtualization correctness with selection and keyboard navigation, lazy-loading not breaking deep-linked/restored tabs), i18n completeness and Turkish quality, leaks (EventSource, timers), light-theme tokens.`

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
No other feature is merged in this wave before you; check git log.

Steps:
1. Make sure the tree is clean and on release/v0.5.0 (git status; git branch --show-current). If not clean, stop and report.
2. git merge --no-ff feat/${f.id} -m "merge: ${f.title}" -m "${TRAILER}"
3. Resolve conflicts preserving the intent of BOTH sides (aggregator files: keep the union, keep ordering sane; the review fixes must survive). package.json conflicts: resolve package.json, then regenerate package-lock.json with npm install and verify npm ci works afterwards. go.mod/go.sum conflicts: resolve go.mod, then cd backend && go mod tidy. Locale JSON conflicts: merge the keys of both sides and keep valid JSON.
4. Run the full checks and fix every integration failure (types, duplicate routes or registrations, i18n parity, tests, layering): npx tsc -b && npm run build && npm run lint && npm run i18n:check; cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./... .
5. Smoke test on the k3d cluster ${f.cluster} (KUBECONFIG=${f.kubeconfig}): build the embedded binary in ${SCRATCH}/merge-${f.id}/ (npm run build; copy dist/* into backend/internal/web/dist/ keeping .gitkeep; go build -o ${SCRATCH}/merge-${f.id}/kubebench . in backend/), start it on port ${f.port + 50} with a fresh DB, log in (admin/admin, then POST /api/auth/password), exercise the feature's main endpoints INCLUDING its mutations inside a namespace e2e-merge-${f.id}-* that you create (for cloud: provider detection and version status of the k3d cluster, the provider badge in the UI, and the Add-cluster credential fields being write-only; also add a pasted kubeconfig with an aws eks get-token exec stanza pointing at an unreachable server and confirm no command runs and the error is clear), check the audit rows, check that a viewer is refused, and drive one main UI flow of the feature with Playwright against the binary. Also re-run a quick regression of the earlier merged features' main endpoints if any are present. Stop the server, delete the namespace and restore backend/internal/web/dist to only .gitkeep.
6. Commit the merge (and any fix-ups as separate "fix(...)" commits ending with the trailer line "${TRAILER}"). The tree must end clean.
Report in the structured format (merged=false only if you could not complete the merge; explain why).`
}

// Review the W3a features while the cloud feature is implemented.
const reviewsPromise = parallel([
  () => agent(REVIEW_SEC, { label: 'review:security', phase: 'Review', schema: FINDINGS_SCHEMA }),
  () => agent(REVIEW_CORR, { label: 'review:correctness', phase: 'Review', schema: FINDINGS_SCHEMA }),
])

const fixPromise = reviewsPromise.then((reviews) => {
  const [sec, corr] = reviews
  const total = (sec ? sec.findings.length : 0) + (corr ? corr.findings.length : 0)
  log(`Review done: security=${sec ? sec.findings.length : 'FAILED'} correctness=${corr ? corr.findings.length : 'FAILED'} findings`)
  if (total === 0) return null
  const FIX_PROMPT = `You are the fixer for KubeBench v0.5 in the MAIN working tree ${REPO} (branch release/v0.5.0). Read CLAUDE.md and docs/design-v0.5.md. Feature agents are working in their own worktrees right now (branch feat/cloud); they will be merged after you, so keep your changes minimal and focused, and do not rename or move foundation APIs unless a fix requires it (if it does, say so clearly in your report). Leave .claude/ alone.

${ENV}
- Scratch space: ${SCRATCH}/w3b-fix/. Ports: 19901 (binary), 20901 (Vite) — no others.

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
    if (!impl) { log(`impl:${f.id} failed — skipping its merge`); return null }
    return serialized(() => fixPromise.then((fix) => agent(mergePrompt(f, impl, fix), { label: `merge:${f.id}`, phase: 'Merge', schema: MERGE_SCHEMA })))
      .then((merge) => ({ feature: f.id, impl, merge }))
  },
)
const reviews = await reviewsPromise
const fix = await fixPromise

const STEP_SCHEMA = {
  type: 'object',
  properties: {
    summary: { type: 'string' },
    filesChanged: { type: 'array', items: { type: 'string' } },
    commits: { type: 'array', items: { type: 'string' } },
    checks: { type: 'array', items: { type: 'object', properties: { command: { type: 'string' }, result: { type: 'string' } }, required: ['command', 'result'] } },
    openIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['summary', 'filesChanged', 'commits', 'checks', 'openIssues'],
}

const DE_COMMON = `You are a professional German software localizer working on KubeBench v0.5 in the MAIN working tree ${REPO} (branch release/v0.5.0; all features are merged). Read CLAUDE.md and docs/design-v0.5.md §6.1. The product owner asked for German as the third UI language ("almanca dil olacak, sonra belki diğerleri de eklenebilir"), so adding a language must stay a data-only change afterwards.
${ENV}
- Scratch space: ${SCRATCH}/w3b-de/. No servers except a Vite dev server on the port given to you.

German style guide: formal "Sie", concise UI German as in VS Code's German localization (e.g. Explorer, Einstellungen, Befehlspalette, Abbrechen, Löschen, Neu starten, Skalieren, Zurücksetzen/Rollback, Deinstallieren, Installieren, Aktualisieren/Upgrade, Werte, Revision, Repository, Design (theme)=Design, Sprache, Dunkel, Hell, System, Aktiviert, Deaktiviert, Verfügbar, Nicht verfügbar, Schreibgeschützt (read-only), Betrachter (viewer), Operator, Administrator, Anmelden, Abmelden, Kennwort, Benutzername, Erweiterungen, Überwachungsprotokoll (audit log), Ereignisse, Protokolle (logs), Übersicht, Details, Arbeitslasten (workloads), Metriken). Kubernetes/Helm/Argo CD nouns stay English and keep their capitalisation (Pod, Pods, Deployment, Namespace, Node, Secret, ConfigMap, PVC, StorageClass, Helm-Release, Chart, CRD, Cluster); German compound/plural handling around them is natural ("Pod löschen", "3 Pods", "Namespace auswählen"). Keep every {{interpolation}} variable and every _one/_other plural pair (German needs both; add _zero only if English has it). Keep keyboard shortcut text, units and placeholders exact. No machine-translation artefacts; consistent terminology across all bundles (agree on terms by reading what the other translator already wrote in de.json files when you start each bundle).
`
const DE_CORE = DE_COMMON + `
YOUR PART (agent "w3b-de-core"): FIRST make the code support German: add 'de' to SUPPORTED_LANGUAGES in src/i18n (language names shown in Settings in their own language: English, Türkçe, Deutsch), make sure language detection/fallback and src/i18n/format.ts handle de-DE, and make scripts/check-i18n.mjs treat the language set as data (it already derives it from src/i18n/locales — add de.json there first so the other translator can start). Then write de.json for: src/i18n/locales, src/core/locales and src/plugins/{workloads,editor,terminal,settings,clusters,users,audit,extensions,metrics,crd}/locales (and any other core plugin bundle not listed for the other translator). Another translator (agent "w3b-de-plugins") does src/plugins/{helm,argocd,storage,security,access,topology}/locales, the live-updates/updates/cloud bundles wherever they live, and examples/plugins/**/locales at the same time; do not edit those. Vite port 15190.
Verify: npm run i18n:check (it may fail only because of the other translator's unfinished bundles — your own must be complete), npx tsc -b, then a Playwright pass over the mock app (npx vite --port 15190 --strictPort) in German: shell, workloads list/detail, settings, users, audit, extensions, editor — screenshots in your scratch dir, and look for truncated/overflowing German labels (German is ~30% longer): fix layout CSS issues you find (min-width/ellipsis), staying within your bundles' plugins and src/core|src/shared CSS.
COMMIT only your files (git add <paths>; git commit) as "feat(i18n): German UI language — core and platform" with the trailer line "${TRAILER}"; if git reports an index.lock from the other translator, wait and retry. Do not commit the other translator's files.`
const DE_PLUGINS = DE_COMMON + `
YOUR PART (agent "w3b-de-plugins"): write de.json for src/plugins/{helm,argocd,storage,security,access,topology}/locales, every bundle added by the live-updates, updates and cloud features that is not under the core plugins listed for the other translator, and examples/plugins/**/locales. Another translator (agent "w3b-de-core") adds 'de' to the code and writes the core/platform/core-plugin bundles at the same time; wait until src/i18n/locales/de.json exists before running the check. Also translate backend-provided texts only if the backend already has a localisation mechanism for them (e.g. security check catalog texts exist in locale files — translate those there; do not invent a backend i18n system). Vite port 15191.
Verify: npm run i18n:check (must pass once both translators are done — re-run until it does, coordinating by waiting), npx tsc -b, then a Playwright pass over the mock app (npx vite --port 15191 --strictPort) in German over your plugins' pages: Helm install wizard, Argo CD app detail, storage wizards, security overview and findings, access who-can, topology — screenshots in your scratch dir; fix truncation/overflow CSS in your plugins.
COMMIT only your files (git add <paths>; git commit) as "feat(i18n): German UI language — plugins" with the trailer line "${TRAILER}"; if git reports an index.lock, wait and retry.`

phase('German')
const de = await parallel([
  () => agent(DE_CORE, { label: 'de:core', phase: 'German', schema: STEP_SCHEMA }),
  () => agent(DE_PLUGINS, { label: 'de:plugins', phase: 'German', schema: STEP_SCHEMA }),
])

phase('Docs')
const DOCS = `You are the technical writer and release engineer for KubeBench v0.5 in the MAIN working tree ${REPO} (branch release/v0.5.0; every feature is merged, German was just added). Read CLAUDE.md, docs/design-v0.5.md (the contract, now with §7.11 updates and §7.12 cloud), docs/dev/v0.5-handoff.md, docs/dev/orchestration/*, docs/notes/*, CHANGELOG.md, docs/deployment.md, docs/releasing.md and the code where needed — every statement you write must be true for the code as it is (check, don't guess).
${ENV}
- Scratch space: ${SCRATCH}/w3b-docs/. Port 19902 if you run the binary (e.g. for screenshots with Playwright).

Write/refresh (English unless stated):
1. README.md: what KubeBench is, feature tour of v0.5 (plugins: workloads, editor, terminal/logs, helm, argocd, storage, security, access, topology, metrics, crd, extensions, users/audit; live updates; update check + What's new; EKS/AKS/GKE; en/tr/de UI), screenshots (take fresh ones from the built binary in mock/demo or against kubebench-e2e with Playwright, save under docs/images/, keep them small, PNG <300 kB each), quick start for Kubernetes (Helm OCI + manifests), VM (install.sh, .deb/.rpm), Docker, development setup, links to docs. README.tr.md: a faithful Turkish version (natural Turkish, the glossary used in the UI). Do not write a German README.
2. CONTRIBUTING.md: dev setup, architecture overview pointers, the layering/i18n/mock-live/permission rules from CLAUDE.md, how to add a core plugin / backend plugin / external UI plugin, how to add a UI language (data-only), checks to run, Conventional Commits, PR process. LICENSE: the owner has NOT chosen yet (options under discussion: PolyForm Noncommercial 1.0.0 with a separate commercial license, or BUSL 1.1) — do not create LICENSE; write a short "License" section saying the project is source-available and the license file will be added before the first public release, and add a "Contributor License Agreement" section stating contributions require agreeing to a CLA so the project can offer commercial licenses (keep it neutral; no legal text).
3. SECURITY.md: supported versions, how to report (GitHub private vulnerability reporting on Shavell/kube-bench), the security model summary (roles, read-only clusters, audit, secret redaction, exec/kubeconfig flags, cloud identity note).
4. docs/: architecture.md (refresh for v0.5: backend packages, plugin platform, router/access levels, watch hub, updates, cloud credentials; frontend shell/plugins/data/i18n), plugins.md (core, backend and external plugin development, SDK reference pointing at src/core/plugin/sdk/kubebench.d.ts and the example), api.md (the full HTTP API reference generated by reading internal/api/routes.go and each plugin's Routes: method, path, access level, params, response shape, error codes; docs/backend.md then becomes a short pointer to api.md), configuration.md (every env var with default and meaning, from internal/config, matching packaging/kubebench.env and the chart values), i18n.md (namespaces, adding strings, adding a language, the check script), security.md (deeper than SECURITY.md: threat model and hardening for operators), design-go-backend.md (mark as historical if superseded).
5. CHANGELOG.md: make the [0.5.0] section accurate and complete against what actually shipped (git log 689adc6..HEAD; group Added/Changed/Fixed/Security/Removed; mention server/ removal and the security fixes at a high level), keep [Unreleased] empty, keep the backfilled 0.1-0.4 entries. Keep the date empty/unset if the release date is not known (the release gate expects that) — check scripts/extract-changelog.sh and docs/releasing.md for the expected format.
6. docs/notes/kubelizeme-comparison.md and .tr.md: reconcile the v0.5 column with what actually shipped (and list what is still missing: AI assistant, Composer, cluster compare, port-forward, ...).
7. CLAUDE.md: update to the final v0.5 rules (keep it concise).
8. docs/dev/v0.5-handoff.md: add a dated "§9 Progress on the Fedora VM (2026-11-14/15)" section: waves W1-W3b with their merge commits, the test clusters used, the review findings fixed per wave (high level), what remains (W4 final review, push to github + origin, merge into main, tag/release, LICENSE decision), and copy the workflow scripts that ran on this VM into docs/dev/orchestration/ as markdown files with the scripts verbatim in js code fences: ${SCRATCH}/w2a-resume.js, ${SCRATCH}/w2b.js, ${SCRATCH}/w2c.js, ${SCRATCH}/w3a.js, ${SCRATCH}/w3b.js, and the two scripts under /root/.claude/projects/-root-workspace-tmp-kube-bench/e3b76051-98cd-4ef5-963e-080ccedcb0b7/workflows/scripts/ (kubebench-v05-w1-resume-*.js and kubebench-v05-w2a-*.js); replace absolute scratch paths in your prose with descriptions where helpful but keep the scripts verbatim; update docs/dev/orchestration/README.md's table.
Run: npm run i18n:check, npx tsc -b, npm run build, a markdown link check of your own (every relative link resolves), and the backend build; restore backend/internal/web/dist to only .gitkeep if you embedded the frontend. Commit in a few logical Conventional Commits (docs: ..., docs(dev): ...) with the trailer line "${TRAILER}". The tree must end clean.`
const docs = await agent(DOCS, { label: 'docs', phase: 'Docs', schema: STEP_SCHEMA })

return { reviews, fix, results, de, docs }
````
