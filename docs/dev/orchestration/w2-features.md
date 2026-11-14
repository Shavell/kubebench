# Workflow W2 — feature waves (verbatim script, not run yet)

Generic "implement each feature in an isolated git worktree, then merge serially into
`release/v0.5.0`" workflow. It is parameterised through `args.features`
(`[{ id, title, port, spec }]`). Feature specs live in [`feature-specs.md`](feature-specs.md).
The script was written but never launched.

Run it once per wave, e.g. W2a = terminal, resource-editing, workload-ops; W2b = helm, argocd,
storage; W2c = security, access, topology. Update `SCRATCH` and the test-cluster rules
before running (handoff §6).

````js
export const meta = {
  name: 'kubebench-v05-features',
  description: 'KubeBench v0.5 features: implement each feature in an isolated worktree, then merge serially into release/v0.5.0 with full checks',
  phases: [
    { title: 'Implement', detail: 'one agent per feature in its own git worktree' },
    { title: 'Merge', detail: 'serial merges into release/v0.5.0, conflict resolution, full checks' },
  ],
}

// args: { features: [{ id, title, port, spec }] }
const SCRATCH = '/tmp/claude-1000/-home-ubuntu-tmp-kube-bench/a1be3ba8-9914-43f9-ab98-a84cbf922e15/scratchpad'
const REPO = '/home/ubuntu/tmp/kube-bench'

const COMMON = `You are a senior engineer on KubeBench, a VSCode-style Kubernetes workbench (React 19 + TS + Vite frontend embedded in a single Go binary: client-go, Helm v3 SDK, modernc SQLite). You work in an ISOLATED git worktree (your current working directory) created from branch release/v0.5.0, which already contains the v0.5 foundation: per-cluster roles + read-only clusters + audit + plugin platform in the backend; plugin SDK v2, permissions, i18n (en/tr), shared UI and locally bundled Monaco in the frontend.

FIRST STEPS:
1. Confirm you are inside a worktree (git rev-parse --show-toplevel must NOT be ${REPO} itself), then create your branch: git checkout -b feat/FEATURE_ID
2. npm ci (node_modules is per worktree; the Go build cache is shared).
3. Read docs/design-v0.5.md (the contract; §N references), CLAUDE.md, and the foundation code you build on: backend/internal/api (central route helper, access levels, audit, errors), backend/internal/plugin (backend plugin contract, Router, KV, helpers), backend/internal/plugins/all, backend/internal/kube (Runtime: CS/Dyn interfaces, Mapper, ResolveKind, NewRuntimeForTest), src/data/apiClient.ts, src/data/pluginData.ts (pluginApi, definePluginData, usePluginQuery, usePluginStatus), src/core/plugin (manifest types, registry, resourceActions, globalSdk), src/core/permissions.ts, src/shared (Dialog, ConfirmDialog, Form, notify, Tabs, Unavailable, editor/CodeEditor, editor/DiffEditor, Table, Toolbar, EmptyState, Badge, contextMenu), src/i18n. Reuse these; do not re-invent them.

PARALLEL WORK: other agents implement other features in their own worktrees at the same time; every branch is merged into release/v0.5.0 afterwards by an integrator. Keep merges painless: keep changes focused on your feature and prefer new files; in shared aggregator files (src/plugins/index.ts, backend/internal/plugins/all/all.go, the core route table in backend/internal/api, src/data/providers.ts, src/data/types.ts, src/data/api/index.ts, src/data/mock/index.ts, src/i18n/locales/*.json, package.json) make minimal append-only additions — never reorder, reformat or rename existing code there.

HARD RULES (§0 of the contract):
- The k3s homelab cluster (~/.kube/config, context k3s-homelab) is READ-ONLY: never send any mutating request to it (create/update/patch/delete/evict/exec/attach/port-forward — not even with dryRun). Prove mutation paths with Go unit tests using fake clients (k8s.io/client-go/kubernetes/fake, k8s.io/client-go/dynamic/fake, discovery fakes) or injectable seams (e.g. an executor factory interface), and with the deterministic mock frontend. Read-only live checks against the homelab are encouraged (lists, gets, logs).
- Never push, never touch git remotes, never build/pull/push container images, never deploy, never create clusters.
- Every user-visible string goes through i18n with complete en + tr entries (follow the existing Turkish glossary/tone in src/i18n/locales/tr.json; Kubernetes nouns stay untranslated with apostrophe suffixes); npm run i18n:check must pass.
- Live mode never falls back to mock data (show Unavailable with the reason). Mock data is deterministic (seeded RNG; no Date.now()/Math.random() in generated data).
- Permissions: gate UI with usePermissions/<Can> (write for mutations, exec for shell/debug/secret reveal); the backend enforces via the route access level — register every new route with the correct level (read/write/exec/admin), dry-run variants need read only.
- Colors only via --vscode-* tokens (both themes). Charts: single-series small multiples, no legends. TS: verbatimModuleSyntax, noUnusedLocals, erasableSyntaxOnly (no enums/parameter properties/namespaces). Layering: plugins never import other plugins or core/shell; cross-plugin navigation via openTab with tab kinds.
- Quality bar: production-grade, idiomatic Go and React, small focused files, meaningful tests, accurate comments, no dead code or fake "TODO" features. Complete the listed scope.
- Scratch space: ${SCRATCH}/FEATURE_ID/ (never leave temp files in the repo). Use port PORT for any backend you run and PORT+1000 for a Vite dev server, so you never collide with parallel agents.

VERIFY before finishing (all must pass):
- npx tsc -b && npm run build && npm run lint && npm run i18n:check (pre-existing only-export-components warnings are acceptable)
- cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./...
- Feature checks listed in your spec (unit tests with fakes; read-only live checks against the homelab using a binary built from your tree with a fresh KUBEBENCH_DB in your scratch dir: KUBEBENCH_DB=..., PORT=PORT, KUBECONFIG=/home/ubuntu/.kube/config; first login is admin/admin and requires POST /api/auth/password before other calls).
- UI check in mock mode: run the Vite dev server (npx vite --port <PORT+1000> --strictPort) and, if Playwright is available in ${SCRATCH}/w1-verify/pw (reuse it; do not install it into the repo), drive your feature's main flows headlessly, collect console errors and save screenshots under your scratch dir. If it is not available, at least load the pages with curl and reason through the components carefully.

COMMIT: commit on feat/FEATURE_ID in a few logical Conventional Commits (feat(<scope>): ..., test(...), etc.), each ending with the trailer line: Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>. Leave the worktree clean (git status empty). Do NOT merge into release/v0.5.0.

FINAL ANSWER: the structured report (branch, worktree path, commit list, summary, files touched, verification commands + results, merge notes about shared files you touched, open issues).
`

const IMPL_SCHEMA = {
  type: 'object',
  properties: {
    branch: { type: 'string' },
    worktreePath: { type: 'string' },
    commits: { type: 'array', items: { type: 'string' } },
    summary: { type: 'string' },
    filesTouched: { type: 'array', items: { type: 'string' } },
    verification: {
      type: 'array',
      items: {
        type: 'object',
        properties: { command: { type: 'string' }, result: { type: 'string' } },
        required: ['command', 'result'],
      },
    },
    mergeNotes: { type: 'string' },
    openIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['branch', 'worktreePath', 'commits', 'summary', 'filesTouched', 'verification', 'mergeNotes', 'openIssues'],
}

const MERGE_SCHEMA = {
  type: 'object',
  properties: {
    merged: { type: 'boolean' },
    mergeCommit: { type: 'string' },
    conflicts: { type: 'array', items: { type: 'string' } },
    fixes: { type: 'array', items: { type: 'string' } },
    checks: {
      type: 'array',
      items: {
        type: 'object',
        properties: { command: { type: 'string' }, result: { type: 'string' } },
        required: ['command', 'result'],
      },
    },
    openIssues: { type: 'array', items: { type: 'string' } },
  },
  required: ['merged', 'mergeCommit', 'conflicts', 'fixes', 'checks', 'openIssues'],
}

// Merges must not overlap (shared index/working tree): chain them.
let mergeChain = Promise.resolve()
function serialized(fn) {
  const run = mergeChain.then(fn)
  mergeChain = run.catch(() => {})
  return run
}

function implPrompt(f) {
  return COMMON.replaceAll('FEATURE_ID', f.id).replaceAll('PORT', String(f.port)) +
    `\n\nYOUR FEATURE: ${f.title} (id: ${f.id})\n${f.spec}\n`
}

function mergePrompt(f, impl) {
  return `You are the integrator for KubeBench v0.5 in the MAIN working tree ${REPO} (branch release/v0.5.0). Read CLAUDE.md and docs/design-v0.5.md. Hard rules: never push or touch remotes, never mutate the homelab cluster (~/.kube/config is read-only), never build images, never create clusters. Leave .claude/ (agent worktrees) alone.

Merge the feature branch feat/${f.id} (${f.title}). Implementer's report: ${JSON.stringify(impl)}

Steps:
1. Make sure the tree is clean and on release/v0.5.0 (git status; git branch --show-current). If not clean, stop and report.
2. git merge --no-ff feat/${f.id} -m "merge: ${f.title}" (add the trailer line "Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>" to the merge message via -m twice).
3. Resolve conflicts preserving the intent of BOTH sides (aggregator files: keep the union, keep ordering sane). package.json conflicts: resolve package.json, then regenerate package-lock.json with npm install (verify npm ci works afterwards). go.mod/go.sum conflicts: resolve go.mod, then cd backend && go mod tidy. Locale JSON conflicts: merge keys from both sides, keep valid JSON.
4. Run the full checks and fix every integration failure (types, duplicate routes/registrations, i18n parity, tests): npx tsc -b && npm run build && npm run lint && npm run i18n:check; cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./... .
5. Quick smoke: build the embedded binary in ${SCRATCH}/merge-${f.id}/ (npm run build; copy dist/* into backend/internal/web/dist/ keeping .gitkeep; go build -o ${SCRATCH}/merge-${f.id}/kubebench .), start it on port ${f.port + 50} with a fresh DB and KUBECONFIG=/home/ubuntu/.kube/config, log in (admin/admin, then POST /api/auth/password), hit the feature's main READ-ONLY endpoints with curl, stop it, and restore backend/internal/web/dist to only .gitkeep.
6. Commit the merge (and any fix-ups as separate "fix(...)" commits with the trailer). The tree must end clean.
Report the result in the structured format (merged=false only if you could not complete the merge; explain why).`
}

const features = args.features
phase('Implement')
const results = await pipeline(
  features,
  (f) => agent(implPrompt(f), { label: `impl:${f.id}`, phase: 'Implement', isolation: 'worktree', schema: IMPL_SCHEMA }),
  (impl, f) => {
    if (!impl) {
      log(`impl:${f.id} failed — skipping merge`)
      return null
    }
    return serialized(() => agent(mergePrompt(f, impl), { label: `merge:${f.id}`, phase: 'Merge', schema: MERGE_SCHEMA }))
      .then((merge) => ({ feature: f.id, impl, merge }))
  },
)
return results
````
