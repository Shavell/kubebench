# Agent orchestration for v0.5

v0.5 is built by several Claude Code agents coordinated with the Workflow
tool. This folder keeps their prompts and scripts **verbatim** so another
machine or session can resume the same plan. Status and next steps:
[`../v0.5-handoff.md`](../v0.5-handoff.md).

| File | What | State |
|---|---|---|
| [`w1-foundation.md`](w1-foundation.md) | W1 workflow (old VM): backend foundation ‖ frontend platform ‖ i18n conversion → integrate & verify | ran, stopped mid-way |
| [`packaging-agent.md`](packaging-agent.md) | Background agent (old VM): Makefile, Dockerfile, chart, manifests, .deb/.rpm, install.sh, offline bundle, GitHub workflows (manual), Dependabot, deployment/releasing docs | ran, stopped early |
| [`w2-features.md`](w2-features.md) | Generic feature workflow (old VM): worktree per feature → serial merge with full checks | not run; the W2/W3 scripts below grew out of it |
| [`feature-specs.md`](feature-specs.md) | Specs F1–F11 fed to the feature workflows | used by W2a–W3a |
| [`w1-resume-fedora.md`](w1-resume-fedora.md) | W1 resumed on the Fedora VM: foundation, UI kit, i18n, packaging (worktree) → integrate on k3d → packaging e2e | ran, merged |
| [`w2a.md`](w2a.md) | W2a: terminal, resource editing, workload ops ‖ review of W1 → fixes → serial merges | interrupted by a session restart |
| [`w2a-resume.md`](w2a-resume.md) | W2a resumed: remaining review fixes and the terminal feature, then the three merges | ran, merged |
| [`w2b.md`](w2b.md) | W2b: Helm v2, Argo CD, Storage ‖ review of W2a → fixes → merges | ran, merged |
| [`w2c.md`](w2c.md) | W2c: Security, Access, Topology ‖ review of W2b → fixes → merges | ran, merged |
| [`w3a.md`](w3a.md) | W3a: live updates, UX polish, update check + What's new ‖ review of W2c → fixes → merges | ran, merged |
| [`w3b.md`](w3b.md) | W3b: cloud providers ‖ review of W3a → fixes → merge, German UI, documentation | ran (this documentation is its last step) |

## Conventions the prompts rely on

- **Contract first.** Every agent implements
  [`../../design-v0.5.md`](../../design-v0.5.md). When the code must
  deviate, the contract is updated in the same change.
- **Shared working tree, disjoint ownership** (W1). Agents running in the
  same tree only edit the files they own. They never change git state, and
  the orchestrator commits.
- **Worktree per feature** (W2). Each agent works in its own worktree, on
  a branch `feat/<id>`, and commits there. An integrator merges the
  branches one at a time into `release/v0.5.0`, resolves conflicts in the
  aggregator files and runs every check.
- **Ports** are unique per agent: backend on `port`, Vite on `port + 1000`.
  **Scratch files** go outside the repo.
- **Checks**:
  - frontend: `npx tsc -b && npm run build && npm run lint && npm run i18n:check`;
  - backend: `cd backend && test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go build ./...`.
- **Cluster safety.** On the old VM the homelab (`k3s-homelab`) was
  read-only for agents. On the Fedora VM `~/.kube/config` belongs to an
  unrelated cluster and is never used: every shell exports `KUBECONFIG` to
  one of the disposable k3d clusters (handoff §9), and mutations stay in
  namespaces the agent created (`e2e-<role>-…`).
- **Commits** follow Conventional Commits and end with the trailer
  `Co-Authored-By: Claude Opus 5.5 <noreply@anthropic.com>`.
- **Never** push, touch remotes, build or push images, or deploy, unless
  the user asks.

## Adapting the scripts to a new session

1. Replace the `SCRATCH` constant with the new session's scratch directory.
2. W1 must continue from the WIP rather than start over. Prepend the
   finished/open checklists from handoff §4 to each build prompt.
3. On a machine with fewer than 4 CPUs the Workflow tool runs at most 2
   agents at a time. The scripts queue work and stay correct; they just
   take longer.
